// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package jm

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/sylabs/singularity-mpi/internal/pkg/buildenv"
	"github.com/sylabs/singularity-mpi/internal/pkg/impi"
	"github.com/sylabs/singularity-mpi/internal/pkg/implem"
	"github.com/sylabs/singularity-mpi/internal/pkg/job"
	"github.com/sylabs/singularity-mpi/internal/pkg/mpi"
	"github.com/sylabs/singularity-mpi/internal/pkg/syexec"
	"github.com/sylabs/singularity-mpi/internal/pkg/sys"
)

// Native is the structure representing the native job manager (i.e., directly use mpirun)
type Native struct {
}

// NativeSetConfig sets the configuration of the native job manager
func NativeSetConfig() error {
	return nil
}

// NativeGetConfig gets the configuration of the native job manager
func NativeGetConfig() error {
	return nil
}

func getEnvPath(mpiCfg *implem.Info, env *buildenv.Info) string {
	// Intel MPI is installing the binaries and libraries in a quite complex setup
	if mpiCfg.ID == implem.IMPI {
		return filepath.Join(env.InstallDir, impi.IntelInstallPathPrefix, "bin") + ":" + os.Getenv("PATH")
	}

	return env.GetEnvPath()
}

func getEnvLDPath(mpiCfg *implem.Info, env *buildenv.Info) string {
	// Intel MPI is installing the binaries and libraries in a quite complex setup
	if mpiCfg.ID == implem.IMPI {
		return filepath.Join(env.InstallDir, impi.IntelInstallPathPrefix, "lib") + ":" + os.Getenv("LD_LIBRARY_PATH")
	}

	return env.GetEnvLDPath()
}

// NativeGetOutput retrieves the application's output after the completion of a job
func NativeGetOutput(j *job.Job, sysCfg *sys.Config) string {
	return j.OutBuffer.String()
}

// NativeGetError retrieves the error messages from an application after the completion of a job
func NativeGetError(j *job.Job, sysCfg *sys.Config) string {
	return j.ErrBuffer.String()
}

// NativeSubmit is the function to call to submit a job through the native job manager
func NativeSubmit(j *job.Job, env *buildenv.Info, sysCfg *sys.Config) (syexec.SyCmd, error) {
	var sycmd syexec.SyCmd

	if j.App.BinPath == "" {
		return sycmd, fmt.Errorf("application binary is undefined")
	}

	sycmd.BinPath = mpi.GetPathToMpirun(j.HostCfg, env)
	if j.NP > 0 {
		sycmd.CmdArgs = append(sycmd.CmdArgs, "-np")
		sycmd.CmdArgs = append(sycmd.CmdArgs, strconv.FormatInt(j.NP, 10))
	}

	mpirunArgs, err := mpi.GetMpirunArgs(j.HostCfg, env, &j.App, j.Container, sysCfg)
	if err != nil {
		return sycmd, fmt.Errorf("unable to get mpirun arguments: %s", err)
	}
	sycmd.CmdArgs = append(sycmd.CmdArgs, mpirunArgs...)

	newPath := getEnvPath(j.HostCfg, env)
	newLDPath := getEnvLDPath(j.HostCfg, env)
	log.Printf("-> PATH=%s", newPath)
	log.Printf("-> LD_LIBRARY_PATH=%s\n", newLDPath)
	log.Printf("Using %s as PATH\n", newPath)
	log.Printf("Using %s as LD_LIBRARY_PATH\n", newLDPath)
	sycmd.Env = append([]string{"LD_LIBRARY_PATH=" + newLDPath}, os.Environ()...)
	sycmd.Env = append([]string{"PATH=" + newPath}, os.Environ()...)

	j.GetOutput = NativeGetOutput
	j.GetError = NativeGetError

	return sycmd, nil
}

// LoadNative is the function used by our job management framework to figure out if mpirun should be used directly.
// The native component is the default job manager. If application, the function returns a structure with all the
// "function pointers" to correctly use the native job manager.
func NativeDetect() (bool, JM) {
	var jm JM
	jm.ID = NativeID
	jm.Get = NativeGetConfig
	jm.Set = NativeSetConfig
	jm.Submit = NativeSubmit

	// This is the default job manager, i.e., mpirun so we do not check anything, just return this component.
	// If the component is selected and mpirun not correctly installed, the framework will pick it up later.
	return true, jm
}
