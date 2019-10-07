// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package jm

import (
	"bytes"
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

	// OutBuffer is a buffer with the output of the job
	OutBuffer bytes.Buffer

	// ErrBuffer is a buffer with the stderr of the job
	ErrBuffer bytes.Buffer

	// GetOutput is the function to call to gather the output of the application based on the use of a given job manager
	GetOutput GetOutputFn

	// GetError is the function to call to gather stderr of the application based on the use of a given job manager
	GetError GetErrorFn
}

// Loader checks whether a giv job manager is applicable or not
type Loader interface {
	Load() bool
}

// launcher represents the details to start a jon
type Launcher struct {
	Cmd     string
	CmdArgs []string
	Env     []string
}

// SetConfigFn is a "function pointer" that lets us store the configuration of a given job manager
type SetConfigFn func() error

// GetConfigFn is a "function pointer" that lets us get the configuration of a given job manager
type GetConfigFn func() error

// SubmitFn is a "function pointer" that lets us job a new job
type SubmitFn func(*Job, *sys.Config) (Launcher, error)

// CleanUpFn is a "function pointer" to call to clean up the system after the completion of a job
type CleanUpFn func(...interface{}) error

// GetOutputFn is a "function pointer" to call to gather the output of an application after completion of a job
type GetOutputFn func(*Job, *sys.Config) string

// GetErrorFn is a "function pointer" to call to gather stderr from an application after completion of a job
type GetErrorFn func(*Job, *sys.Config) string

// JM is the structure representing a specific JM
type JM struct {
	// ID identifies which job manager has been detected on the system
	ID string

	// Set is the function that sets the configuration of the current job manager
	Set SetConfigFn

	// Get is the function that gets the configuration of the current job manager
	Get GetConfigFn

	// Submit is the function to submit a job through the current job manager
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

// TempFile creates a temporary file that is used to store a batch script
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

// PrepareLaunchCmd interacts with a job manager backend to figure out how to launch a job
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
	cmd.Cmd.Stdout = &job.OutBuffer
	cmd.Cmd.Stderr = &job.ErrBuffer

	return cmd, nil
}
