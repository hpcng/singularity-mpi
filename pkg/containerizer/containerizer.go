// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package containerizer

import (
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/sylabs/singularity-mpi/internal/pkg/distro"

	"github.com/gvallee/go_util/pkg/util"
	"github.com/gvallee/kv/pkg/kv"
	"github.com/sylabs/singularity-mpi/internal/pkg/app"
	"github.com/sylabs/singularity-mpi/internal/pkg/buildenv"
	"github.com/sylabs/singularity-mpi/internal/pkg/builder"
	"github.com/sylabs/singularity-mpi/internal/pkg/container"
	"github.com/sylabs/singularity-mpi/internal/pkg/deffile"
	"github.com/sylabs/singularity-mpi/internal/pkg/implem"
	"github.com/sylabs/singularity-mpi/internal/pkg/mpi"
	"github.com/sylabs/singularity-mpi/internal/pkg/sys"
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
	mpiCfgFile := sys.GetMPIConfigFileName(mpi)
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
	if app.envScript == "" {
		// We generate the script only if its path is defined. The path not being defined just means that
		// we do not need it, for instance, MPI was installed to compile an app on the host for a
		// container based on the bind model
		log.Printf("- Path to script to set environment is undefined, skipping its creation...")
		return nil
	}

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

func generateStandardDeffile(app *appConfig, container *container.Config, sysCfg *sys.Config) (deffile.DefFileData, error) {
	deffileCfg := deffile.DefFileData{
		Path:     container.DefFile,
		DistroID: distro.ParseDescr(container.Distro),
	}

	// Sanity checks
	if app == nil || container == nil || sysCfg == nil || container.DefFile == "" {
		return deffileCfg, fmt.Errorf("invalid parameter(s)")
	}

	log.Printf("-> Create definition file %s\n", container.DefFile)

	err := deffile.CreateBasicDefFile(&app.info, &deffileCfg, sysCfg)
	if err != nil {
		return deffileCfg, fmt.Errorf("unable to create container: %s", err)
	}

	return deffileCfg, nil
}

func generateMPIDeffile(app *appConfig, mpiCfg *mpi.Config, sysCfg *sys.Config) (deffile.DefFileData, error) {
	deffileCfg := deffile.DefFileData{
		Path:     mpiCfg.Container.DefFile,
		DistroID: distro.ParseDescr(mpiCfg.Container.Distro),
	}

	// Sanity checks
	if app == nil || mpiCfg == nil || sysCfg == nil || mpiCfg.Container.DefFile == "" {
		return deffileCfg, fmt.Errorf("invalid parameter(s)")
	}

	log.Printf("-> Creating definition file %s for application %s\n", mpiCfg.Container.DefFile, app.info.Name)

	deffileCfg.MpiImplm = &mpiCfg.Implem
	deffileCfg.InternalEnv = &mpiCfg.Buildenv
	deffileCfg.InternalEnv.InstallDir = filepath.Join(sysCfg.Persistent, sys.MPIInstallDirPrefix+mpiCfg.Implem.ID+"-"+mpiCfg.Implem.Version)
	log.Printf("-> Installing MPI in container in %s\n", deffileCfg.InternalEnv.InstallDir)
	deffileCfg.Model = mpiCfg.Container.Model

	switch mpiCfg.Container.Model {
	case container.HybridModel:
		// todo: should call the builder and not directly that function
		err := deffile.CreateHybridDefFile(&app.info, &deffileCfg, sysCfg)
		if err != nil {
			return deffileCfg, fmt.Errorf("unable to create container: %s", err)
		}
	case container.BindModel:
		b, err := builder.Load(&mpiCfg.Implem)
		if err != nil {
			return deffileCfg, fmt.Errorf("unable to instantiate builder")
		}

		var hostAppBuildEnv buildenv.Info
		log.Println("Bind mode: compiling application on the host...")
		err = b.CompileMPIAppOnHost(&app.info, mpiCfg, &hostAppBuildEnv, sysCfg)
		if err != nil {
			return deffileCfg, fmt.Errorf("failed to compile the application on the host: %s", err)
		}

		// todo: should call the builder and not directly that function
		deffileCfg.InternalEnv.InstallDir = mpiCfg.Buildenv.InstallDir
		err = deffile.CreateBindDefFile(&app.info, &deffileCfg, sysCfg)
		if err != nil {
			return deffileCfg, fmt.Errorf("unable to create container: %s", err)
		}
	}

	return deffileCfg, nil
}

// ContainerizeApp will parse the configuration file specific to an app, install
// the appropriate MPI on the host, as well as create the container.
func ContainerizeApp(sysCfg *sys.Config) (container.Config, error) {
	var containerMPI mpi.Config

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

	// Put together the container's metadata
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
	default:
		// This is where we end up when no MPI is used by the container
		containerBuildEnv, cleanup, err = getCommonContainerConfiguration(kvs, &containerMPI.Container, sysCfg)
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
	url := kv.GetValue(kvs, "registry")
	if url != "" && string(url[len(url)-1]) != "/" {
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
		log.Println("-> Application does not need the execution of an install command")
	}

	// Generate images
	log.Println("* Container configuration:")
	log.Printf("-> Application's name: %s\n", app.info.Name)
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

	err = containerMPI.Buildenv.Init(sysCfg)
	if err != nil {
		return containerMPI.Container, fmt.Errorf("failed to initialize build environment: %s", err)
	}

	// Make sure the image already exists, if so, stop, we do not overwrite images, ever
	if util.FileExists(containerMPI.Container.Path) {
		fmt.Printf("%s already exists, stopping\n", containerMPI.Container.Path)
		return containerMPI.Container, nil
	}

	// Generate definition file
	log.Println("* Generating definition file...")
	var deffileData deffile.DefFileData
	if kv.GetValue(kvs, "mpi") != "" {
		deffileData, err = generateMPIDeffile(&app, &containerMPI, sysCfg)
		if err != nil {
			return containerMPI.Container, fmt.Errorf("failed to generate definition file %s: %s", containerMPI.Container.DefFile, err)
		}
	} else {
		deffileData, err = generateStandardDeffile(&app, &containerMPI.Container, sysCfg)
		if err != nil {
			return containerMPI.Container, fmt.Errorf("failed to generate definition file %s: %s", containerMPI.Container.DefFile, err)
		}
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

	return containerMPI.Container, nil
}
