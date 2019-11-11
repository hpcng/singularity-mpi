// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package containizer

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/sylabs/singularity-mpi/internal/pkg/app"
	"github.com/sylabs/singularity-mpi/internal/pkg/buildenv"
	"github.com/sylabs/singularity-mpi/internal/pkg/builder"
	"github.com/sylabs/singularity-mpi/internal/pkg/container"
	"github.com/sylabs/singularity-mpi/internal/pkg/deffile"
	"github.com/sylabs/singularity-mpi/internal/pkg/implem"
	"github.com/sylabs/singularity-mpi/internal/pkg/kv"
	"github.com/sylabs/singularity-mpi/internal/pkg/mpi"
	"github.com/sylabs/singularity-mpi/internal/pkg/sys"
	util "github.com/sylabs/singularity-mpi/internal/pkg/util/file"
)

const (
	mpiModelKey = "mpi_model"
)

type appConfig struct {
	// info is the high-level information about the application to execute
	info app.Info

	// tarball is the name of the tarball associated to the application
	tarball string

	// envScript is the path to the script that the user will be
	// able to use to set all the environment variables necessary to use the MPI installed on the host
	envScript string
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

	path := filepath.Join(sysCfg.EtcDir, mpiCfgFile)
	kvs, err := kv.LoadKeyValueConfig(path)
	if err != nil {
		log.Printf("[WARN] Cannot load configuration from %s: %s", path, err)
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

	// Sanity checks
	if app == nil || mpiCfg == nil || sysCfg == nil || mpiCfg.Container.DefFile == "" {
		return def, fmt.Errorf("invalid parameter(s)")
	}

	log.Printf("-> Create definition file %s\n", mpiCfg.Container.DefFile)

	deffileCfg := deffile.DefFileData{
		Path:   mpiCfg.Container.DefFile,
		Distro: mpiCfg.Container.Distro,
	}

	deffileCfg.MpiImplm = &mpiCfg.Implem
	deffileCfg.InternalEnv = &mpiCfg.Buildenv
	deffileCfg.Model = mpiCfg.Container.Model

	switch mpiCfg.Container.Model {
	case container.HybridModel:
		// todo: should call the builder and not directly that function
		err := deffile.CreateHybridDefFile(&app.info, &deffileCfg, sysCfg)
		if err != nil {
			return def, fmt.Errorf("unable to create container: %s", err)
		}
	case container.BindModel:
		b, err := builder.Load(&mpiCfg.Implem)
		if err != nil {
			return def, fmt.Errorf("unable to instantiate builder")
		}

		var hostAppBuildEnv buildenv.Info
		log.Println("Bind mode: compiling application on the host...")
		err = b.CompileAppOnHost(&app.info, mpiCfg, &hostAppBuildEnv, sysCfg)
		if err != nil {
			return def, fmt.Errorf("failed to compile the application on the host: %s", err)
		}

		// todo: should call the builder and not directly that function
		err = deffile.CreateBindDefFile(&app.info, &deffileCfg, sysCfg)
		if err != nil {
			return def, fmt.Errorf("unable to create container: %s", err)
		}
	}

	return deffileCfg, nil
}

func getCommonContainerConfiguration(kvs []kv.KV, containerMPI *mpi.Config, sysCfg *sys.Config) (buildenv.Info, func(), error) {
	// (deffile.DefFileData, buildenv.Info) {
	var containerBuildEnv buildenv.Info
	var err error
	var cleanup func()

	// Data from the user's configuration file
	containerMPI.Container.Name = kv.GetValue(kvs, "app_name") + ".sif"
	containerMPI.Container.Distro = kv.GetValue(kvs, "distro")
	containerMPI.Implem.ID = kv.GetValue(kvs, "mpi")
	containerMPI.Implem.Version = kv.GetValue(kvs, "container_mpi")
	containerMPI.Implem.URL = getMPIURL(kv.GetValue(kvs, "mpi"), containerMPI.Implem.Version, sysCfg)

	// These different structures are used during different stage of the creation of the container
	// so yes we have some duplication in term of value stored in elements of different structures
	// but this allows us to have fairly independent components without dependency circles.
	if sysCfg.Persistent == "" {
		// If we do not integrate with the sympi (i.e., no persistent mode), all subdirectories
		// in the system wide scratch directory or, if that directory is not defined, in a new
		// temporary directory. In any case, the temporary directory will NOT
		// be deleted since it will have both the container image and the definition file. The
		// subdirectories in the temporary directory should be deleted automatically.
		if sysCfg.ScratchDir != "" {
			containerBuildEnv.ScratchDir = sysCfg.ScratchDir
		} else {
			containerBuildEnv.ScratchDir, err = ioutil.TempDir("", "")
			if err != nil {
				return containerBuildEnv, nil, fmt.Errorf("failed to create temporary directory: %s", err)
			}
		}
		containerBuildEnv.BuildDir = filepath.Join(containerBuildEnv.ScratchDir, "container", "build")
		containerBuildEnv.InstallDir = filepath.Join(containerBuildEnv.ScratchDir, "install")
		containerMPI.Container.Path = filepath.Join(containerBuildEnv.ScratchDir, containerMPI.Container.Name)

		cleanup = func() {
			err := os.RemoveAll(containerBuildEnv.ScratchDir)
			if err != nil {
				log.Printf("failed to cleanup %s: %s", containerBuildEnv.ScratchDir, err)
			}
			err = os.RemoveAll(containerBuildEnv.BuildDir)
			if err != nil {
				log.Printf("failed to cleanup %s: %s", containerBuildEnv.BuildDir, err)
			}
			err = os.RemoveAll(containerBuildEnv.InstallDir)
			if err != nil {
				log.Printf("failed to cleanup %s: %s", containerBuildEnv.InstallDir, err)
			}
		}
	} else {
		containerBuildEnv.ScratchDir = filepath.Join(sysCfg.Persistent, "scratch_"+kv.GetValue(kvs, "app_name"))
		containerBuildEnv.BuildDir = filepath.Join(sysCfg.Persistent, "build_"+kv.GetValue(kvs, "app_name"))
		containerBuildEnv.InstallDir = filepath.Join(sysCfg.Persistent, sys.ContainerInstallDirPrefix+kv.GetValue(kvs, "app_name"))
		containerMPI.Container.Path = filepath.Join(containerBuildEnv.InstallDir, containerMPI.Container.Name)

		cleanup = func() {
			err := os.RemoveAll(containerBuildEnv.ScratchDir)
			if err != nil {
				log.Printf("failed to cleanup %s: %s", containerBuildEnv.ScratchDir, err)
			}
			err = os.RemoveAll(containerBuildEnv.BuildDir)
			if err != nil {
				log.Printf("failed to cleanup %s: %s", containerBuildEnv.BuildDir, err)
			}
		}
	}

	containerMPI.Container.BuildDir = containerBuildEnv.BuildDir
	containerMPI.Container.InstallDir = containerBuildEnv.InstallDir
	containerMPI.Container.DefFile = filepath.Join(containerBuildEnv.BuildDir, kv.GetValue(kvs, "app_name")+".def")
	if sysCfg.ScratchDir != "" {
		log.Printf("Changing system-wide scratch directory from %s to %s\n", sysCfg.ScratchDir, containerBuildEnv.ScratchDir)
	}
	sysCfg.ScratchDir = containerBuildEnv.ScratchDir

	return containerBuildEnv, cleanup, nil
}

func getHybridConfiguration(kvs []kv.KV, containerMPI *mpi.Config, sysCfg *sys.Config) (buildenv.Info, func(), error) {
	containerBuildEnv, cleanup, err := getCommonContainerConfiguration(kvs, containerMPI, sysCfg)
	if err != nil {
		return containerBuildEnv, cleanup, err
	}
	containerMPI.Container.Model = container.HybridModel
	return containerBuildEnv, cleanup, nil
}

func getBindConfiguration(kvs []kv.KV, containerMPI *mpi.Config, sysCfg *sys.Config) (buildenv.Info, func(), error) {
	containerBuildEnv, cleanup, err := getCommonContainerConfiguration(kvs, containerMPI, sysCfg)
	if err != nil {
		return containerBuildEnv, cleanup, err
	}
	containerMPI.Container.Model = container.BindModel
	return containerBuildEnv, cleanup, nil
}

func installMPIonHost(kvs []kv.KV, hostBuildEnv *buildenv.Info, app *appConfig, sysCfg *sys.Config) error {
	var hostMPI mpi.Config
	hostBuildEnv.BuildDir = filepath.Join(kv.GetValue(kvs, "scratch_dir"), "host", "build")
	hostMPI.Implem.ID = kv.GetValue(kvs, "mpi")
	hostMPI.Implem.Version = kv.GetValue(kvs, "host_mpi")
	mpiDir := hostMPI.Implem.ID + "-" + hostMPI.Implem.Version
	hostBuildEnv.InstallDir = filepath.Join(kv.GetValue(kvs, "output_dir"), "install", mpiDir)
	hostMPI.Implem.URL = getMPIURL(kv.GetValue(kvs, "mpi"), hostMPI.Implem.Version, sysCfg)

	// todo: this should be part of hostMPI, not app
	app.envScript = filepath.Join(kv.GetValue(kvs, "output_dir"), hostMPI.Implem.ID+"-"+hostMPI.Implem.Version+".env")

	if !util.PathExists(kv.GetValue(kvs, "output_dir")) {
		err := os.MkdirAll(kv.GetValue(kvs, "output_dir"), 0766)
		if err != nil {
			return fmt.Errorf("failed to create %s: %s", kv.GetValue(kvs, "output_dir"), err)
		}
	}

	err := util.DirInit(hostBuildEnv.BuildDir)
	if err != nil {
		return fmt.Errorf("failed to initialize %s: %s", hostBuildEnv.BuildDir, err)
	}
	err = util.DirInit(hostBuildEnv.InstallDir)
	if err != nil {
		return fmt.Errorf("failed to initialize %s: %s", hostBuildEnv.InstallDir, err)
	}

	// Instantiate and call a builder
	b, err := builder.Load(&hostMPI.Implem)
	if err != nil {
		return fmt.Errorf("unable to create a builder: %s", err)
	}
	res := b.InstallOnHost(&hostMPI.Implem, hostBuildEnv, sysCfg)
	if res.Err != nil {
		return fmt.Errorf("failed to install MPI on the host: %s", res.Err)
	}

	// Generate env file
	err = generateEnvFile(app, &hostMPI.Implem, hostBuildEnv, sysCfg)
	if err != nil {
		return fmt.Errorf("failed to generate the environment variable: %s", err)
	}

	fmt.Printf("File to set the MPI environment: %s\n", app.envScript)

	return nil
}

// ContainerizeApp will parse the configuration file specific to an app, install
// the appropriate MPI on the host, as well as create the container.
func ContainerizeApp(sysCfg *sys.Config) (container.Config, error) {
	var containerMPI mpi.Config

	skipHostMPI := false

	log.Printf("* Loading configuration from %s\n", sysCfg.AppContainizer)
	// Load config file
	kvs, err := kv.LoadKeyValueConfig(sysCfg.AppContainizer)
	if err != nil {
		return containerMPI.Container, fmt.Errorf("Impossible to load configuration file: %s", err)
	}

	// Some sanity checks
	if kv.GetValue(kvs, "app_name") == "" {
		return containerMPI.Container, fmt.Errorf("Application's name is not defined")
	}
	if kv.GetValue(kvs, "app_url") == "" {
		return containerMPI.Container, fmt.Errorf("Application URL is not defined")
	}
	if kv.GetValue(kvs, "app_exe") == "" {
		return containerMPI.Container, fmt.Errorf("Application executable is not defined")
	}
	if kv.GetValue(kvs, "mpi") == "" {
		return containerMPI.Container, fmt.Errorf("MPI implementation is not defined")
	}
	if kv.GetValue(kvs, "host_mpi") == "" {
		skipHostMPI = true
	}
	if kv.GetValue(kvs, "container_mpi") == "" {
		return containerMPI.Container, fmt.Errorf("container MPI version is not defined")
	}

	// Put together the container's metadata
	var hostBuildEnv buildenv.Info
	var containerBuildEnv buildenv.Info
	var cleanup func()

	switch kv.GetValue(kvs, mpiModelKey) {
	case container.HybridModel:
		containerBuildEnv, cleanup, err = getHybridConfiguration(kvs, &containerMPI, sysCfg)
		if err != nil {
			return containerMPI.Container, fmt.Errorf("failed to set build environment: %s", err)
		}
	case container.BindModel:
		containerBuildEnv, cleanup, err = getBindConfiguration(kvs, &containerMPI, sysCfg)
		if err != nil {
			return containerMPI.Container, fmt.Errorf("failed to set build environment: %s", err)
		}
	}
	if cleanup != nil {
		defer cleanup()
	}
	containerMPI.Buildenv = containerBuildEnv

	// Load some generic data
	curTime := time.Now()
	url := kv.GetValue(kvs, "registery")
	if string(url[len(url)-1]) != "/" {
		url = url + "/"
	}
	sysCfg.Registry = url + kv.GetValue(kvs, "app_name") + ":" + curTime.Format("20060102")

	// Load the app configuration
	var app appConfig
	app.info.Name = kv.GetValue(kvs, "app_name")
	app.info.Source = kv.GetValue(kvs, "app_url")
	app.tarball = path.Base(app.info.Source)
	app.info.BinName = kv.GetValue(kvs, "app_exe")
	app.info.InstallCmd = kv.GetValue(kvs, "app_compile_cmd")
	if app.info.Source == "" {
		return containerMPI.Container, fmt.Errorf("application's URL is not defined")
	}
	if app.tarball == "" {
		return containerMPI.Container, fmt.Errorf("application's package is not defined")
	}
	if app.info.InstallCmd == "" {
		return containerMPI.Container, fmt.Errorf("application's compilation command is not defined")
	}

	err = containerMPI.Buildenv.Init(sysCfg)
	if err != nil {
		return containerMPI.Container, fmt.Errorf("failed to initialize build environment: %s", err)
	}

	// Install MPI on host
	if !skipHostMPI {
		log.Println("* Installing MPI on host...")
		err := installMPIonHost(kvs, &hostBuildEnv, &app, sysCfg)
		if err != nil {
			return containerMPI.Container, fmt.Errorf("failed to install MPI on host: %s", err)
		}
	}

	// Generate images
	log.Println("* Container configuration:")
	log.Printf("-> Definition file: %s\n", containerMPI.Container.DefFile)
	log.Printf("-> MPI implementation: %s\n", containerMPI.Implem.ID)
	log.Printf("-> MPI implementation version: %s\n", containerMPI.Implem.Version)
	log.Printf("-> MPI URL: %s\n", containerMPI.Implem.URL)
	log.Printf("-> Scratch directory: %s\n", sysCfg.ScratchDir)
	log.Printf("-> Build directory: %s\n", containerMPI.Container.BuildDir)
	log.Printf("-> Install directory: %s\n", containerMPI.Container.InstallDir)
	log.Printf("-> Container name: %s\n", containerMPI.Container.Name)
	log.Printf("-> Container Linux distribution: %s\n", containerMPI.Container.Distro)
	log.Printf("-> Container path: %s\n", containerMPI.Container.Path)
	log.Printf("-> Container MPI model: %s\n", containerMPI.Container.Model)
	log.Printf("-> Target container image: %s\n", containerMPI.Container.Path)

	// Make sure the image already exists, if so, stop, we do not overwrite images, ever
	if util.FileExists(containerMPI.Container.Path) {
		fmt.Printf("%s already exists, stopping\n", containerMPI.Container.Path)
		return containerMPI.Container, nil
	}

	// Generate definition file
	log.Println("* Generating definition file...")
	deffileData, err := generateMPIDeffile(&app, &containerMPI, sysCfg)
	if err != nil {
		return containerMPI.Container, fmt.Errorf("failed to generate definition file %s: %s", containerMPI.Container.DefFile, err)
	}

	// Backup the definition file when in debug mode
	if sysCfg.Debug {
		// We do not track failure while backing up definition file
		deffileData.Backup(&containerBuildEnv)
	}

	// Create container
	log.Println("* Creating container image...")
	err = container.Create(&containerMPI.Container, sysCfg)
	if err != nil {
		return containerMPI.Container, fmt.Errorf("failed to create container: %s", err)
	}

	// todo: Upload image if necessary
	if sysCfg.Upload {
		if os.Getenv(container.KeyPassphrase) == "" {
			log.Println("WARN: passphrase for key is not defined")
		}

		err = container.Sign(&containerMPI.Container, sysCfg)
		if err != nil {
			return containerMPI.Container, fmt.Errorf("failed to sign image: %s", err)
		}

		err = container.Upload(&containerMPI.Container, sysCfg)
		if err != nil {
			return containerMPI.Container, fmt.Errorf("failed to upload image: %s", err)
		}
	}

	fmt.Printf("Container image path: %s\n", containerMPI.Container.Path)
	/*
		appPath := filepath.Join("/opt", app.dir, app.exe)
		fmt.Printf("Command example to execute your application with two MPI ranks: mpirun -np 2 singularity exec " + containerMPI.ContainerPath + " " + appPath + "\n")
	*/

	return containerMPI.Container, nil
}
