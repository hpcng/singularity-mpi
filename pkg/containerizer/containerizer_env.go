// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package containerizer

import (
	"fmt"
	"log"
	"path/filepath"

	"github.com/gvallee/kv/pkg/kv"
	"github.com/sylabs/singularity-mpi/pkg/buildenv"
	"github.com/sylabs/singularity-mpi/pkg/container"
	"github.com/sylabs/singularity-mpi/pkg/mpi"
	"github.com/sylabs/singularity-mpi/pkg/sys"
)

func getCommonContainerConfiguration(kvs []kv.KV, container *container.Config, sysCfg *sys.Config) (buildenv.Info, func(), error) {
	var containerBuildEnv buildenv.Info

	cleanup, err := buildenv.CreateDefaultContainerEnvCfg(&containerBuildEnv, kvs, sysCfg)
	if err != nil {
		return containerBuildEnv, nil, fmt.Errorf("failed to create container environment: %s", err)
	}

	// Data from the user's configuration file
	container.Name = kv.GetValue(kvs, "app_name") + ".sif"
	container.Distro = kv.GetValue(kvs, "distro")

	// These different structures are used during different stage of the creation of the container
	// so yes we have some duplication in term of value stored in elements of different structures
	// but this allows us to have fairly independent components without dependency circles.
	if sysCfg.Persistent == "" {
		container.Path = filepath.Join(containerBuildEnv.ScratchDir, container.Name)
	} else {
		container.Path = filepath.Join(containerBuildEnv.InstallDir, container.Name)
	}

	container.BuildDir = containerBuildEnv.BuildDir
	container.InstallDir = containerBuildEnv.InstallDir
	container.DefFile = filepath.Join(containerBuildEnv.BuildDir, kv.GetValue(kvs, "app_name")+".def")
	if sysCfg.ScratchDir != "" {
		log.Printf("Changing system-wide scratch directory from %s to %s\n", sysCfg.ScratchDir, containerBuildEnv.ScratchDir)
	}
	sysCfg.ScratchDir = containerBuildEnv.ScratchDir

	return containerBuildEnv, cleanup, nil
}

func getCommonMPIContainerConfiguration(kvs []kv.KV, containerMPI *mpi.Config, sysCfg *sys.Config) (buildenv.Info, func(), error) {
	containerMPI.Implem.ID, containerMPI.Implem.Version = sys.ParseDistroID(kv.GetValue(kvs, "mpi"))
	containerMPI.Implem.URL = getMPIURL(containerMPI.Implem.ID, containerMPI.Implem.Version, sysCfg)

	return getCommonContainerConfiguration(kvs, &containerMPI.Container, sysCfg)
}

func getHybridConfiguration(kvs []kv.KV, containerMPI *mpi.Config, sysCfg *sys.Config) (buildenv.Info, func(), error) {
	containerBuildEnv, cleanup, err := getCommonMPIContainerConfiguration(kvs, containerMPI, sysCfg)
	if err != nil {
		return containerBuildEnv, cleanup, err
	}
	containerMPI.Container.Model = container.HybridModel
	return containerBuildEnv, cleanup, nil
}

func getBindConfiguration(kvs []kv.KV, containerMPI *mpi.Config, sysCfg *sys.Config) (buildenv.Info, func(), error) {
	containerBuildEnv, cleanup, err := getCommonMPIContainerConfiguration(kvs, containerMPI, sysCfg)
	if err != nil {
		return containerBuildEnv, cleanup, err
	}
	containerMPI.Container.Model = container.BindModel
	return containerBuildEnv, cleanup, nil
}
