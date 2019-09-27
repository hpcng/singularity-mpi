// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package containizer

import (
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/sylabs/singularity-mpi/internal/pkg/deffile"
	util "github.com/sylabs/singularity-mpi/internal/pkg/util/file"
	"github.com/sylabs/singularity-mpi/internal/pkg/util/sy"

	"github.com/sylabs/singularity-mpi/internal/pkg/kv"
	"github.com/sylabs/singularity-mpi/internal/pkg/mpi"
	"github.com/sylabs/singularity-mpi/internal/pkg/sys"
)

type appConfig struct {
	// url is the URL to use to download the application
	url string
	// tarball is the name of the tarball associated to the application
	tarball string
	// dir is the directory of the source code once the tarball is unpacked
	//dir string
	// compileCmd is the command used to compile the application
	compileCmd string
	// envScript is the path to the script that the user will be
	// able to use to set all the environment variables necessary to use the MPI installed on the host
	envScript string
	// exe is the binary name to start the application
	exe string
}

func getMPIURL(mpi string, version string, sysCfg *sys.Config) string {
	var mpiCfgFile string

	switch mpi {
	case "openmpi":
		mpiCfgFile = "openmpi.conf"
	case "mpich":
		mpiCfgFile = "mpich.conf"
	case "intel":
		mpiCfgFile = "intel.conf"
	}

	path := filepath.Join(sysCfg.BinPath, "etc", mpiCfgFile)
	kvs, err := kv.LoadKeyValueConfig(path)
	if err != nil {
		return ""
	}
	for _, kv := range kvs {
		if kv.Key == version {
			return kv.Value
		}
	}

	return ""
}

func generateEnvFile(app *appConfig, mpiCfg *mpi.Config, sysCfg *sys.Config) error {
	log.Printf("- Generating script to set environment (%s)", app.envScript)

	f, err := os.Create(app.envScript)
	if err != nil {
		return fmt.Errorf("failed to create %s: %s", app.envScript, err)
	}

	_, err = f.WriteString("#!/bin/bash\n#\n\n")
	if err != nil {
		return err
	}
	_, err = f.WriteString("export PATH=" + mpiCfg.InstallDir + "/bin:$PATH\n")
	if err != nil {
		return err
	}
	_, err = f.WriteString("export LD_LIBRARY_PATH=" + mpiCfg.InstallDir + "/lib:$LD_LIBRARY_PATH\n")
	if err != nil {
		return err
	}
	_, err = f.WriteString("export MANPATH=" + mpiCfg.InstallDir + "/man:$MANPATH\n")
	if err != nil {
		return err
	}

	err = f.Close()
	if err != nil {
		return fmt.Errorf("failed to close %s: %s", app.envScript, err)
	}

	return nil
}

func generateMPIDeffile(app *appConfig, mpiCfg *mpi.Config, sysCfg *sys.Config) error {
	f, err := os.Create(mpiCfg.DefFile)
	if err != nil {
		return fmt.Errorf("failed to create %s: %s", mpiCfg.DefFile, err)
	}

	deffileCfg := deffile.DefFileData{
		Distro:     mpiCfg.Distro,
		MpiImplm:   mpiCfg.MpiImplm,
		MpiVersion: mpiCfg.MpiVersion,
		MpiURL:     mpiCfg.URL,
		//AppDir:     app.dir,
	}

	err = deffile.AddBootstrap(f, &deffileCfg)
	if err != nil {
		return err
	}

	err = deffile.AddLabels(f, &deffileCfg)
	if err != nil {
		return err
	}

	err = deffile.AddMPIEnv(f, &deffileCfg)
	if err != nil {
		return err
	}

	// Get and compile the app
	_, err = f.WriteString("\tcd /opt && wget " + app.url + " && tar -xzf " + app.tarball + "\n")
	if err != nil {
		return err
	}

	// Download and unpack the app
	_, err = f.WriteString("\tAPPDIR=`ls -l /opt | egrep '^d' | head -1 | awk '{print $9}'`\n")
	if err != nil {
		return err
	}

	err = deffile.AddMPIInstall(f, &deffileCfg)
	if err != nil {
		return err
	}

	// Compile the app
	_, err = f.WriteString("\tcd /opt/$APPDIR && " + app.compileCmd + "\n")
	if err != nil {
		return err
	}

	// Clean up
	_, err = f.WriteString("\trm -rf /opt/" + app.tarball + " /tmp/ompi\n")
	if err != nil {
		return err
	}

	err = f.Close()
	if err != nil {
		return fmt.Errorf("failed to close %s: %s", mpiCfg.DefFile, err)
	}

	return nil
}

// ContainerizeApp will parse the configuration file specific to an app, install
// the appropriate MPI on the host, as well as create the container.
func ContainerizeApp(sysCfg *sys.Config) error {
	skipHostMPI := false

	// Load config file
	kvs, err := kv.LoadKeyValueConfig(sysCfg.AppContainizer)
	if err != nil {
		return fmt.Errorf("Impossible to load configuration file: %s", err)
	}

	// Some sanity checks
	if kv.GetValue(kvs, "scratch_dir") == "" {
		return fmt.Errorf("scratch directory is not defined")
	}
	if kv.GetValue(kvs, "output_dir") == "" {
		return fmt.Errorf("output directory is not defined")
	}
	if kv.GetValue(kvs, "app_url") == "" {
		return fmt.Errorf("Application URL is not defined")
	}
	if kv.GetValue(kvs, "app_exe") == "" {
		return fmt.Errorf("Application executable is not defined")
	}
	if kv.GetValue(kvs, "mpi") == "" {
		return fmt.Errorf("MPI implementation is not defined")
	}
	if kv.GetValue(kvs, "host_mpi") == "" {
		skipHostMPI = true
	}
	if kv.GetValue(kvs, "container_mpi") == "" {
		return fmt.Errorf("container MPI version is not defined")
	}

	// Load some generic data
	curTime := time.Now()
	url := kv.GetValue(kvs, "registery")
	if string(url[len(url)-1]) != "/" {
		url = url + "/"
	}
	sysCfg.Registry = url + kv.GetValue(kvs, "app_name") + ":" + curTime.Format("20060102")

	// Load the app configuration
	var app appConfig
	app.url = kv.GetValue(kvs, "app_url")
	app.tarball = path.Base(app.url)
	app.exe = kv.GetValue(kvs, "app_exe")
	app.compileCmd = kv.GetValue(kvs, "app_compile_cmd")
	if app.url == "" {
		return fmt.Errorf("application's URL is not defined")
	}
	if app.tarball == "" {
		return fmt.Errorf("application's package is not defined")
	}
	if app.compileCmd == "" {
		return fmt.Errorf("application's compilation command is not defined")
	}

	// Install MPI on host
	if !skipHostMPI {
		var hostMPI mpi.Config
		hostMPI.BuildDir = filepath.Join(kv.GetValue(kvs, "scratch_dir"), "host", "build")
		hostMPI.MpiImplm = kv.GetValue(kvs, "mpi")
		hostMPI.MpiVersion = kv.GetValue(kvs, "host_mpi")
		hostMPI.ContainerPath = filepath.Join(kv.GetValue(kvs, "output_dir"), "app.sif")
		hostMPI.DefFile = filepath.Join(kv.GetValue(kvs, "output_dir"), "app.def")
		mpiDir := hostMPI.MpiImplm + "-" + hostMPI.MpiVersion
		hostMPI.InstallDir = filepath.Join(kv.GetValue(kvs, "output_dir"), "install", mpiDir)
		hostMPI.URL = getMPIURL(kv.GetValue(kvs, "mpi"), hostMPI.MpiVersion, sysCfg)

		// todo: this should be part of hostMPI, not app
		app.envScript = filepath.Join(kv.GetValue(kvs, "output_dir"), hostMPI.MpiImplm+"-"+hostMPI.MpiVersion+".env")

		if !util.PathExists(kv.GetValue(kvs, "output_dir")) {
			err := os.MkdirAll(kv.GetValue(kvs, "output_dir"), 0766)
			if err != nil {
				return fmt.Errorf("failed to create %s: %s", kv.GetValue(kvs, "output_dir"), err)
			}
		}

		err = util.DirInit(hostMPI.BuildDir)
		if err != nil {
			return fmt.Errorf("failed to initialize %s: %s", hostMPI.BuildDir, err)
		}
		err = util.DirInit(hostMPI.InstallDir)
		if err != nil {
			return fmt.Errorf("failed to initialize %s: %s", hostMPI.InstallDir, err)
		}

		res := mpi.InstallHost(&hostMPI, sysCfg)
		if res.Err != nil {
			return fmt.Errorf("failed to install MPI on the host: %s", res.Err)
		}

		// Generate env file
		err = generateEnvFile(&app, &hostMPI, sysCfg)
		if err != nil {
			return fmt.Errorf("failed to generate the environment variable: %s", err)
		}

		fmt.Printf("File to set the MPI environment: %s\n", app.envScript)
	}

	// Generate images

	var containerMPI mpi.Config
	containerMPI.BuildDir = filepath.Join(kv.GetValue(kvs, "scratch_dir"), "container", "build")
	containerMPI.ContainerName = kv.GetValue(kvs, "app_name") + ".sif"
	containerMPI.ContainerPath = filepath.Join(kv.GetValue(kvs, "output_dir"), containerMPI.ContainerName)
	containerMPI.DefFile = filepath.Join(kv.GetValue(kvs, "output_dir"), kv.GetValue(kvs, "app_name")+".def")
	containerMPI.InstallDir = filepath.Join(kv.GetValue(kvs, "output_dir"), "install")
	containerMPI.MpiImplm = kv.GetValue(kvs, "mpi")
	containerMPI.MpiVersion = kv.GetValue(kvs, "container_mpi")
	containerMPI.URL = getMPIURL(kv.GetValue(kvs, "mpi"), containerMPI.MpiVersion, sysCfg)
	containerMPI.Distro = kv.GetValue(kvs, "distro")

	err = util.DirInit(containerMPI.BuildDir)
	if err != nil {
		return fmt.Errorf("failed to initialize %s: %s", containerMPI.BuildDir, err)
	}

	// Generate definition file
	err = generateMPIDeffile(&app, &containerMPI, sysCfg)
	if err != nil {
		return fmt.Errorf("failed to generate definition file %s: %s", containerMPI.DefFile, err)
	}

	// Create container
	err = mpi.CreateContainer(&containerMPI, sysCfg)
	if err != nil {
		return fmt.Errorf("failed to create container: %s", err)
	}

	// todo: Upload image if necessary
	if sysCfg.Upload {
		if os.Getenv(sy.KeyPassphrase) == "" {
			log.Println("WARN: passphrase for key is not defined")
		}

		err = sy.Sign(containerMPI, sysCfg)
		if err != nil {
			return fmt.Errorf("failed to sign image: %s", err)
		}

		err = sy.Upload(containerMPI, sysCfg)
		if err != nil {
			return fmt.Errorf("failed to upload image: %s", err)
		}
	}

	fmt.Printf("Container image path: %s\n", containerMPI.ContainerPath)
	/*
		appPath := filepath.Join("/opt", app.dir, app.exe)
		fmt.Printf("Command example to execute your application with two MPI ranks: mpirun -np 2 singularity exec " + containerMPI.ContainerPath + " " + appPath + "\n")
	*/

	return nil
}
