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

func getCommonContainerConfiguration(kvs []kv.KV, containerMPI *mpi.Config, sysCfg *sys.Config) buildenv.Info {
	// (deffile.DefFileData, buildenv.Info) {
	var containerBuildEnv buildenv.Info

	// These different structures are used during different stage of the creation of the container
	// so yes we have some duplication in term of value stored in elements of different structures
	// but this allows us to have fairly independent components without dependency circles.
	containerBuildEnv.BuildDir = filepath.Join(kv.GetValue(kvs, "scratch_dir"), "container", "build")
	containerBuildEnv.InstallDir = containerBuildEnv.BuildDir
	containerBuildEnv.ScratchDir = kv.GetValue(kvs, "scratch_dir")
	containerMPI.Container.BuildDir = containerBuildEnv.BuildDir
	containerMPI.Container.InstallDir = containerBuildEnv.BuildDir
	containerMPI.Container.Name = kv.GetValue(kvs, "app_name") + ".sif"
	if sysCfg.Persistent == "" {
		containerMPI.Container.Path = filepath.Join(kv.GetValue(kvs, "output_dir"), containerMPI.Container.Name)
	} else {
		containerInstallDir := filepath.Join(sysCfg.Persistent, sys.ContainerInstallDirPrefix+kv.GetValue(kvs, "app_name"))
		containerMPI.Container.Path = filepath.Join(containerInstallDir, containerMPI.Container.Name)
		if !util.PathExists(containerInstallDir) {
			err := util.DirInit(containerInstallDir)
			if err != nil {
				log.Printf("[WARN] failed to create %s", containerInstallDir)
			}
		}
	}
	containerMPI.Container.DefFile = filepath.Join(kv.GetValue(kvs, "output_dir"), kv.GetValue(kvs, "app_name")+".def")
	containerMPI.Container.Distro = kv.GetValue(kvs, "distro")
	containerBuildEnv.InstallDir = filepath.Join(kv.GetValue(kvs, "output_dir"), "install")
	containerMPI.Implem.ID = kv.GetValue(kvs, "mpi")
	containerMPI.Implem.Version = kv.GetValue(kvs, "container_mpi")
	containerMPI.Implem.URL = getMPIURL(kv.GetValue(kvs, "mpi"), containerMPI.Implem.Version, sysCfg)

	return containerBuildEnv
}

func getHybridConfiguration(kvs []kv.KV, containerMPI *mpi.Config, sysCfg *sys.Config) buildenv.Info {
	containerBuildEnv := getCommonContainerConfiguration(kvs, containerMPI, sysCfg)
	containerMPI.Container.Model = container.HybridModel
	return containerBuildEnv
}

func getBindConfiguration(kvs []kv.KV, containerMPI *mpi.Config, sysCfg *sys.Config) buildenv.Info {
	containerBuildEnv := getCommonContainerConfiguration(kvs, containerMPI, sysCfg)
	containerMPI.Container.Model = container.BindModel
	return containerBuildEnv
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
	//	var containerCfg container.Config
	var containerMPI mpi.Config

	skipHostMPI := false

	log.Printf("* Loading configuration from %s\n", sysCfg.AppContainizer)
	// Load config file
	kvs, err := kv.LoadKeyValueConfig(sysCfg.AppContainizer)
	if err != nil {
		return containerMPI.Container, fmt.Errorf("Impossible to load configuration file: %s", err)
	}

	// Some sanity checks
	if kv.GetValue(kvs, "scratch_dir") == "" {
		return containerMPI.Container, fmt.Errorf("scratch directory is not defined")
	}
	if kv.GetValue(kvs, "output_dir") == "" {
		return containerMPI.Container, fmt.Errorf("output directory is not defined")
	}
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

	switch kv.GetValue(kvs, mpiModelKey) {
	case container.HybridModel:
		containerBuildEnv = getHybridConfiguration(kvs, &containerMPI, sysCfg)
	case container.BindModel:
		containerBuildEnv = getBindConfiguration(kvs, &containerMPI, sysCfg)
	}

	// Load some generic data
	curTime := time.Now()
	url := kv.GetValue(kvs, "registery")
	if string(url[len(url)-1]) != "/" {
		url = url + "/"
	}
	sysCfg.Registry = url + kv.GetValue(kvs, "app_name") + ":" + curTime.Format("20060102")
	sysCfg.ScratchDir = kv.GetValue(kvs, "scratch_dir")

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
	log.Printf("-> MPI URL: %s\n", containerMPI.Container.URL)
	log.Printf("-> Build directory: %s\n", containerMPI.Container.BuildDir)
	log.Printf("-> Install directory: %s\n", containerMPI.Container.InstallDir)
	log.Printf("-> Container name: %s\n", containerMPI.Container.Name)
	log.Printf("-> Container Linux distribution: %s\n", containerMPI.Container.Distro)
	log.Printf("-> Container MPI model: %s\n", containerMPI.Container.Model)
	log.Printf("-> Scratch directory: %s\n", sysCfg.ScratchDir)
	log.Printf("-> Target container image: %s\n", containerMPI.Container.Path)

	// Make sure the image already exists, if so, stop, we do not overwrite images, ever
	if util.FileExists(containerMPI.Container.Path) {
		fmt.Printf("%s already exists, stopping\n", containerMPI.Container.Path)
		return containerMPI.Container, nil
	}

	// If the scratch dir exists, we delete it to start fresh
	err = util.DirInit(sysCfg.ScratchDir)
	if err != nil {
		return containerMPI.Container, fmt.Errorf("failed to initialize %s: %s", sysCfg.ScratchDir, err)
	}

	err = util.DirInit(containerBuildEnv.BuildDir)
	if err != nil {
		return containerMPI.Container, fmt.Errorf("failed to initialize %s: %s", containerBuildEnv.BuildDir, err)
	}

	// Generate definition file
	log.Println("* Generating definition file...")
	_, err = generateMPIDeffile(&app, &containerMPI, sysCfg)
	if err != nil {
		return containerMPI.Container, fmt.Errorf("failed to generate definition file %s: %s", containerMPI.Container.DefFile, err)
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
