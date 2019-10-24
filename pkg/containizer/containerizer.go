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

	"github.com/sylabs/singularity-mpi/internal/pkg/jm"
	"github.com/sylabs/singularity-mpi/internal/pkg/builder"
	"github.com/sylabs/singularity-mpi/internal/pkg/buildenv"
	"github.com/sylabs/singularity-mpi/internal/pkg/container"
	"github.com/sylabs/singularity-mpi/internal/pkg/deffile"
	"github.com/sylabs/singularity-mpi/internal/pkg/implem"
	"github.com/sylabs/singularity-mpi/internal/pkg/kv"
	"github.com/sylabs/singularity-mpi/internal/pkg/mpi"
	"github.com/sylabs/singularity-mpi/internal/pkg/sys"
	util "github.com/sylabs/singularity-mpi/internal/pkg/util/file"
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

func generateEnvFile(app *appConfig, mpiCfg *implem.Info, env *buildenv.Info, sysCfg *sys.Config) error {
	log.Printf("- Generating script to set environment (%s)", app.envScript)

	f, err := os.Create(app.envScript)
	if err != nil {
		return fmt.Errorf("failed to create %s: %s", app.envScript, err)
	}

	_, err = f.WriteString("#!/bin/bash\n#\n\n")
	if err != nil {
		return err
	}
	_, err = f.WriteString("export PATH=" + env.InstallDir + "/bin:$PATH\n")
	if err != nil {
		return err
	}
	_, err = f.WriteString("export LD_LIBRARY_PATH=" + env.InstallDir + "/lib:$LD_LIBRARY_PATH\n")
	if err != nil {
		return err
	}
	_, err = f.WriteString("export MANPATH=" + env.InstallDir + "/man:$MANPATH\n")
	if err != nil {
		return err
	}

	err = f.Close()
	if err != nil {
		return fmt.Errorf("failed to close %s: %s", app.envScript, err)
	}

	return nil
}

func generateMPIDeffile(app *appConfig, mpiCfg *mpi.Config, sysCfg *sys.Config) (deffile.DefFileData, error) {
	var def deffile.DefFileData
	f, err := os.Create(mpiCfg.Container.DefFile)
	if err != nil {
		return def, fmt.Errorf("failed to create %s: %s", mpiCfg.Container.DefFile, err)
	}

	deffileCfg := deffile.DefFileData{
		Path:   mpiCfg.Container.DefFile,
		Distro: mpiCfg.Container.Distro,
	}

	deffileCfg.MpiImplm.ID = mpiCfg.Implem.ID
	deffileCfg.MpiImplm.Version = mpiCfg.Implem.Version
	deffileCfg.MpiImplm.URL = mpiCfg.Implem.URL

	err = deffile.AddBootstrap(f, &deffileCfg)
	if err != nil {
		return deffileCfg, err
	}

	err = deffile.AddLabels(f, &deffileCfg)
	if err != nil {
		return deffileCfg, err
	}

	err = deffile.AddMPIEnv(f, &deffileCfg)
	if err != nil {
		return deffileCfg, err
	}

	// Get and compile the app
	_, err = f.WriteString("\tcd /opt && wget " + app.url + " && tar -xzf " + app.tarball + "\n")
	if err != nil {
		return deffileCfg, err
	}

	// Download and unpack the app
	_, err = f.WriteString("\tAPPDIR=`ls -l /opt | egrep '^d' | head -1 | awk '{print $9}'`\n")
	if err != nil {
		return deffileCfg, err
	}

	err = deffile.AddMPIInstall(f, &deffileCfg)
	if err != nil {
		return deffileCfg, err
	}

	// Compile the app
	_, err = f.WriteString("\tcd /opt/$APPDIR && " + app.compileCmd + "\n")
	if err != nil {
		return deffileCfg, err
	}

	// Clean up
	_, err = f.WriteString("\trm -rf /opt/" + app.tarball + " /tmp/ompi\n")
	if err != nil {
		return deffileCfg, err
	}

	err = f.Close()
	if err != nil {
		return deffileCfg, fmt.Errorf("failed to close %s: %s", deffileCfg.Path, err)
	}

	return deffileCfg, nil
}

// ContainerizeApp will parse the configuration file specific to an app, install
// the appropriate MPI on the host, as well as create the container.
func ContainerizeApp(sysCfg *sys.Config) (container.Config, error) {
	var containerCfg container.Config

	skipHostMPI := false

	// Load config file
	kvs, err := kv.LoadKeyValueConfig(sysCfg.AppContainizer)
	if err != nil {
		return containerCfg, fmt.Errorf("Impossible to load configuration file: %s", err)
	}

	// Some sanity checks
	if kv.GetValue(kvs, "scratch_dir") == "" {
		return containerCfg, fmt.Errorf("scratch directory is not defined")
	}
	if kv.GetValue(kvs, "output_dir") == "" {
		return containerCfg, fmt.Errorf("output directory is not defined")
	}
	if kv.GetValue(kvs, "app_url") == "" {
		return containerCfg, fmt.Errorf("Application URL is not defined")
	}
	if kv.GetValue(kvs, "app_exe") == "" {
		return containerCfg, fmt.Errorf("Application executable is not defined")
	}
	if kv.GetValue(kvs, "mpi") == "" {
		return containerCfg, fmt.Errorf("MPI implementation is not defined")
	}
	if kv.GetValue(kvs, "host_mpi") == "" {
		skipHostMPI = true
	}
	if kv.GetValue(kvs, "container_mpi") == "" {
		return containerCfg, fmt.Errorf("container MPI version is not defined")
	}

	// Put together the container's metadata
	var containerMPI mpi.Config
	var containerBuildEnv buildenv.Info
	var deffileCfg deffile.DefFileData
	var hostBuildEnv buildenv.Info
	containerCfg.Path = filepath.Join(kv.GetValue(kvs, "output_dir"), containerCfg.Name)
	containerBuildEnv.BuildDir = filepath.Join(kv.GetValue(kvs, "scratch_dir"), "container", "build")
	containerCfg.Name = kv.GetValue(kvs, "app_name") + ".sif"
	containerCfg.DefFile = filepath.Join(kv.GetValue(kvs, "output_dir"), kv.GetValue(kvs, "app_name")+".def")
	containerBuildEnv.InstallDir = filepath.Join(kv.GetValue(kvs, "output_dir"), "install")
	containerMPI.Implem.ID = kv.GetValue(kvs, "mpi")
	containerMPI.Implem.Version = kv.GetValue(kvs, "container_mpi")
	containerMPI.Implem.URL = getMPIURL(kv.GetValue(kvs, "mpi"), containerMPI.Implem.Version, sysCfg)
	deffileCfg.Distro = kv.GetValue(kvs, "distro")
	deffileCfg.InternalEnv = &containerBuildEnv
	deffileCfg.MpiImplm = &containerMPI.Implem

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
		return containerCfg, fmt.Errorf("application's URL is not defined")
	}
	if app.tarball == "" {
		return containerCfg, fmt.Errorf("application's package is not defined")
	}
	if app.compileCmd == "" {
		return containerCfg, fmt.Errorf("application's compilation command is not defined")
	}

	// Install MPI on host
	if !skipHostMPI {
		var hostMPI mpi.Config
		hostBuildEnv.BuildDir = filepath.Join(kv.GetValue(kvs, "scratch_dir"), "host", "build")
		hostMPI.Implem.ID = kv.GetValue(kvs, "mpi")
		hostMPI.Implem.Version = kv.GetValue(kvs, "host_mpi")
		deffileCfg.Path = filepath.Join(kv.GetValue(kvs, "output_dir"), "app.def")
		mpiDir := hostMPI.Implem.ID + "-" + hostMPI.Implem.Version
		hostBuildEnv.InstallDir = filepath.Join(kv.GetValue(kvs, "output_dir"), "install", mpiDir)
		hostMPI.Implem.URL = getMPIURL(kv.GetValue(kvs, "mpi"), hostMPI.Implem.Version, sysCfg)

		// todo: this should be part of hostMPI, not app
		app.envScript = filepath.Join(kv.GetValue(kvs, "output_dir"), hostMPI.Implem.ID+"-"+hostMPI.Implem.Version+".env")

		if !util.PathExists(kv.GetValue(kvs, "output_dir")) {
			err := os.MkdirAll(kv.GetValue(kvs, "output_dir"), 0766)
			if err != nil {
				return containerCfg, fmt.Errorf("failed to create %s: %s", kv.GetValue(kvs, "output_dir"), err)
			}
		}

		err = util.DirInit(hostBuildEnv.BuildDir)
		if err != nil {
			return containerCfg, fmt.Errorf("failed to initialize %s: %s", hostBuildEnv.BuildDir, err)
		}
		err = util.DirInit(hostBuildEnv.InstallDir)
		if err != nil {
			return containerCfg, fmt.Errorf("failed to initialize %s: %s", hostBuildEnv.InstallDir, err)
		}

		// Lookup the job manager configuration so we can know how to install MPI on the host
		jobmgr := jm.Detect()

		// Instantiate and call a builder
		b, err := builder.Load(&hostMPI.Implem)
		if err != nil {
			return containerCfg, fmt.Errorf("unable to create a builder: %s", err)
		}
		res := b.InstallHost(&hostMPI.Implem, &jobmgr, &hostBuildEnv, sysCfg)
		if res.Err != nil {
			return containerCfg, fmt.Errorf("failed to install MPI on the host: %s", res.Err)
		}

		// Generate env file
		err = generateEnvFile(&app, &hostMPI.Implem, &hostBuildEnv, sysCfg)
		if err != nil {
			return containerCfg, fmt.Errorf("failed to generate the environment variable: %s", err)
		}

		fmt.Printf("File to set the MPI environment: %s\n", app.envScript)
	}

	// Generate images

	err = util.DirInit(containerBuildEnv.BuildDir)
	if err != nil {
		return containerCfg, fmt.Errorf("failed to initialize %s: %s", containerBuildEnv.BuildDir, err)
	}

	// Generate definition file
	deffileCfg, err = generateMPIDeffile(&app, &containerMPI, sysCfg)
	if err != nil {
		return containerCfg, fmt.Errorf("failed to generate definition file %s: %s", containerCfg.DefFile, err)
	}

	// Create container
	err = container.Create(&containerCfg, sysCfg)
	if err != nil {
		return containerCfg, fmt.Errorf("failed to create container: %s", err)
	}

	// todo: Upload image if necessary
	if sysCfg.Upload {
		if os.Getenv(container.KeyPassphrase) == "" {
			log.Println("WARN: passphrase for key is not defined")
		}

		err = container.Sign(&containerCfg, sysCfg)
		if err != nil {
			return containerCfg, fmt.Errorf("failed to sign image: %s", err)
		}

		err = container.Upload(&containerCfg, sysCfg)
		if err != nil {
			return containerCfg, fmt.Errorf("failed to upload image: %s", err)
		}
	}

	fmt.Printf("Container image path: %s\n", containerCfg.Path)
	/*
		appPath := filepath.Join("/opt", app.dir, app.exe)
		fmt.Printf("Command example to execute your application with two MPI ranks: mpirun -np 2 singularity exec " + containerMPI.ContainerPath + " " + appPath + "\n")
	*/

	return containerCfg, nil
}
