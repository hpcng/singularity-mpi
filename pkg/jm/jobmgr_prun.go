// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package jm

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/sylabs/singularity-mpi/internal/pkg/job"
	"github.com/sylabs/singularity-mpi/pkg/buildenv"
	"github.com/sylabs/singularity-mpi/pkg/container"
	"github.com/sylabs/singularity-mpi/pkg/syexec"
	"github.com/sylabs/singularity-mpi/pkg/sys"
)

// Prun is the structure representing the native job manager (i.e., directly use mpirun)
type Prun struct {
}

// PrunSetConfig sets the configuration of the native job manager
func PrunSetConfig() error {
	return nil
}

// PrunGetConfig gets the configuration of the native job manager
func PrunGetConfig() error {
	return nil
}

// PrunGetOutput retrieves the application's output after the completion of a job
func PrunGetOutput(j *job.Job, sysCfg *sys.Config) string {
	return j.OutBuffer.String()
}

// PrunGetError retrieves the error messages from an application after the completion of a job
func PrunGetError(j *job.Job, sysCfg *sys.Config) string {
	return j.ErrBuffer.String()
}

// PrunSubmit is the function to call to submit a job through the native job manager
func PrunSubmit(j *job.Job, env *buildenv.Info, sysCfg *sys.Config) (syexec.SyCmd, error) {
	var sycmd syexec.SyCmd
	var err error

	if j.App.BinPath == "" {
		return sycmd, fmt.Errorf("application binary is undefined")
	}

	sycmd.BinPath, err = exec.LookPath("prun")
	if err != nil {
		return sycmd, fmt.Errorf("prun not found")
	}

	for _, a := range j.Args {
		sycmd.CmdArgs = append(sycmd.CmdArgs, a)
	}
	sycmd.CmdArgs = append(sycmd.CmdArgs, "-x")
	sycmd.CmdArgs = append(sycmd.CmdArgs, "PATH")
	sycmd.CmdArgs = append(sycmd.CmdArgs, "-x")
	sycmd.CmdArgs = append(sycmd.CmdArgs, "SY_EXEC_ARGS")
	sycmd.CmdArgs = append(sycmd.CmdArgs, j.Container.Path)
	sycmd.CmdArgs = append(sycmd.CmdArgs, j.Container.AppExe)

	// Get the exec arguments and set the env var
	execArgs := container.GetMPIExecCfg(j.HostCfg, env, j.Container, sysCfg)
	syExecArgsEnv := "SY_EXEC_ARGS=\"" + strings.Join(execArgs, " ") + "\""
	log.Printf("Command to be executed: %s %s", sycmd.BinPath, strings.Join(sycmd.CmdArgs, " "))
	log.Printf("SY_EXEC_ARGS to be used: %s", strings.Join(execArgs, " "))

	newPath := getEnvPath(j.HostCfg, env)
	newLDPath := getEnvLDPath(j.HostCfg, env)
	log.Printf("-> PATH=%s", newPath)
	log.Printf("-> LD_LIBRARY_PATH=%s\n", newLDPath)
	log.Printf("Using %s as PATH\n", newPath)
	log.Printf("Using %s as LD_LIBRARY_PATH\n", newLDPath)
	sycmd.Env = append([]string{"LD_LIBRARY_PATH=" + newLDPath}, os.Environ()...)
	sycmd.Env = append([]string{"PATH=" + newPath}, sycmd.Env...)
	sycmd.Env = append([]string{syExecArgsEnv}, sycmd.Env...)

	j.GetOutput = PrunGetOutput
	j.GetError = PrunGetError

	return sycmd, nil
}

// PrunDetect is the function used by our job management framework to figure out if mpirun should be used directly.
// The native component is the default job manager. If application, the function returns a structure with all the
// "function pointers" to correctly use the native job manager.
func PrunDetect() (bool, JM) {
	var jm JM

	_, err := exec.LookPath("prun")
	if err != nil {
		log.Println("* prun not detected")
		return false, jm
	}

	jm.ID = PrunID
	jm.Get = PrunGetConfig
	jm.Set = PrunSetConfig
	jm.Submit = PrunSubmit

	// This is the default job manager, i.e., mpirun so we do not check anything, just return this component.
	// If the component is selected and mpirun not correctly installed, the framework will pick it up later.
	return true, jm
}
