// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package jm

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/sylabs/singularity-mpi/internal/pkg/sympierr"
	util "github.com/sylabs/singularity-mpi/internal/pkg/util/file"

	"github.com/sylabs/singularity-mpi/internal/pkg/mpi"
	"github.com/sylabs/singularity-mpi/internal/pkg/sys"
)

const (
	// Native is the value set to JM.ID when mpirun shall be used to submit a job
	NativeID = "native"

	// Slurm is the value set to JM.ID when Slurm shall be used to submit a job
	SlurmID = "slurm"
)

type SubmitCmd struct {
	// Cmd represents the command to execute to submit the job
	Cmd *exec.Cmd

	// Ctx is the context of the command to execute to submit a job
	Ctx context.Context

	// CancelFn is the function to cancel the command to submit a job
	CancelFn context.CancelFunc

	// Env is the environment to append when executing the command
	Env []string
}

type Job struct {
	// NP is the number of ranks
	NP int64

	// NNodes is the number of nodes
	NNodes int64

	// CleanUp is the function to call once the job is completed to clean the system
	CleanUp CleanUpFn

	// BatchScript is the path to the script required to start a job (optional)
	BatchScript string

	// HostCfg is the MPI configuration to use on the host
	HostCfg *mpi.Config

	// ContainerCfg is the MPI configuration to use in the container
	ContainerCfg *mpi.Config

	// AppBin is the path to the application's binary, i.e., the binary to start
	AppBin string
}

// Loader checks whether a giv job manager is applicable or not
type Loader interface {
	Load() bool
}

/*
// Starter is responsible for starting a job
type Starter interface {
	Start(*JM) error
}
*/

// launcher represents the details to start a jon
type Launcher struct {
	Cmd     string
	CmdArgs []string
	Env     []string
}

type SetConfigFn func() error

type GetConfigFn func() error

type SubmitFn func(*Job, *sys.Config) (Launcher, error)

type CleanUpFn func(...interface{}) error

// JM is the structure representing a specific JM
type JM struct {
	// ID identifies which job manager has been detected on the system
	ID string

	Set SetConfigFn

	Get GetConfigFn

	Submit SubmitFn
}

// Detect figures out which job manager must be used on the system and return a
// structure that gather all the data necessary to interact with it
func Detect() JM {
	// Default job manager
	loaded, comp := LoadNative()
	if !loaded {
		log.Fatalln("unable to find a default job manager")
	}

	// Now we check if we can find better
	loaded, slurmComp := LoadSlurm()
	if loaded {
		return slurmComp
	}

	return comp
}

func TempFile(j *Job, sysCfg *sys.Config) error {
	filePrefix := "sbash-" + j.ContainerCfg.ContainerName
	path := ""
	if sysCfg.Persistent == "" {
		f, err := ioutil.TempFile("", filePrefix+"-")
		if err != nil {
			return fmt.Errorf("failed to create temporary file: %s", err)
		}
		path = f.Name()
		f.Close()
	} else {
		fileName := filePrefix + ".sh"
		path = filepath.Join(j.ContainerCfg.BuildDir, fileName)
		if util.PathExists(path) {
			j.BatchScript = path
			return sympierr.ErrFileExists
		}
	}
	j.BatchScript = path

	j.CleanUp = func(...interface{}) error {
		err := os.RemoveAll(path)
		if err != nil {
			return fmt.Errorf("unable to delete %s: %s", path, err)
		}
		return nil
	}

	return nil
}

func PrepareLaunchCmd(job *Job, sysCfg *sys.Config) (SubmitCmd, error) {
	var cmd SubmitCmd

	jm := Detect()

	launcher, err := jm.Submit(job, sysCfg)
	if err != nil {
		return cmd, fmt.Errorf("failed to create a launcher object: %s", err)
	}
	log.Printf("* Command object for '%s %s' is ready", launcher.Cmd, strings.Join(launcher.CmdArgs, " "))

	cmd.Ctx, cmd.CancelFn = context.WithTimeout(context.Background(), sys.CmdTimeout*time.Minute)
	cmd.Cmd = exec.CommandContext(cmd.Ctx, launcher.Cmd, launcher.CmdArgs...)

	return cmd, nil
}
