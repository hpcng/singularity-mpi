// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package mpi

import (
	"log"
	"path/filepath"

	"github.com/sylabs/singularity-mpi/internal/pkg/app"
	"github.com/sylabs/singularity-mpi/internal/pkg/buildenv"
	"github.com/sylabs/singularity-mpi/internal/pkg/container"
	"github.com/sylabs/singularity-mpi/internal/pkg/impi"
	"github.com/sylabs/singularity-mpi/internal/pkg/implem"
	"github.com/sylabs/singularity-mpi/internal/pkg/openmpi"
	"github.com/sylabs/singularity-mpi/internal/pkg/sys"
)

// Config represents a configuration of MPI for a target platform
// todo: revisit this, i do not think we actually need it, i think it would make everything
// easier if we were dealing with the different elements separately
type Config struct {
	// Implem gathers information about the MPI implementation to use
	Implem implem.Info

	// Buildenv gathers all the information regarding the build environment used to setup MPI
	Buildenv buildenv.Info

	// Container associated to the MPI configuration
	Container container.Config
}

// GetPathToMpirun returns the path to mpirun based a configuration of MPI
func GetPathToMpirun(mpiCfg *implem.Info, env *buildenv.Info) string {
	// Intel MPI is installing the binaries and libraries in a quite complex setup
	if mpiCfg.ID == implem.IMPI {
		return impi.GetPathToMpirun(env)
	}

	return filepath.Join(env.InstallDir, "bin", "mpirun")
}

func getBindArguments(hostMPI *implem.Info, hostBuildenv *buildenv.Info, c *container.Config) []string {
	var bindArgs []string

	if c.Model == container.BindModel {
		if c.MPIDir == "" {
			log.Println("[WARN] the path to mount MPI in the container is undefined")
		}
		bindStr := hostBuildenv.InstallDir + ":" + c.MPIDir
		bindArgs = append(bindArgs, bindStr)
	}

	return bindArgs
}

// GetMpirunArgs returns the arguments required by a mpirun
func GetMpirunArgs(myHostMPICfg *implem.Info, hostBuildEnv *buildenv.Info, app *app.Info, syContainer *container.Config, sysCfg *sys.Config) ([]string, error) {
	args := []string{"singularity", "exec", "--cleanenv", "--contain", "--no-home", "--writable"}

	if sysCfg.Nopriv {
		args = append(args, "-u")
	}

	bindArgs := getBindArguments(myHostMPICfg, hostBuildEnv, syContainer)
	if len(bindArgs) > 0 {
		args = append(args, "--bind")
		args = append(args, bindArgs...)
	}

	args = append(args, syContainer.Path, app.BinPath)
	var extraArgs []string

	// We really do not want to do this but MPICH is being picky about args so for now, it will do the job.
	switch myHostMPICfg.ID {
	/*
		case implem.IMPI:
			extraArgs := impi.GetExtraMpirunArgs(myHostMPICfg, sysCfg)
	*/
	case implem.OMPI:
		extraArgs = append(extraArgs, openmpi.GetExtraMpirunArgs(sysCfg)...)
	}

	if len(extraArgs) > 0 {
		args = append(extraArgs, args...)
	}

	return args, nil
}

// GetMPIConfigFile returns the path to the configuration file for a given MPI implementation
func GetMPIConfigFile(id string, sysCfg *sys.Config) string {
	return filepath.Join(sysCfg.EtcDir, id+".conf")
}
