// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package jm

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/sylabs/singularity-mpi/internal/pkg/buildenv"
	"github.com/sylabs/singularity-mpi/internal/pkg/job"
	"github.com/sylabs/singularity-mpi/internal/pkg/syexec"
	"github.com/sylabs/singularity-mpi/internal/pkg/sympierr"
	"github.com/sylabs/singularity-mpi/internal/pkg/sys"
	util "github.com/sylabs/singularity-mpi/internal/pkg/util/file"
)

const (
	// Native is the value set to JM.ID when mpirun shall be used to submit a job
	NativeID = "native"

	// Slurm is the value set to JM.ID when Slurm shall be used to submit a job
	SlurmID = "slurm"
)

// Loader checks whether a giv job manager is applicable or not
type Loader interface {
	Load() bool
}

// SetConfigFn is a "function pointer" that lets us store the configuration of a given job manager
type SetConfigFn func() error

// GetConfigFn is a "function pointer" that lets us get the configuration of a given job manager
type GetConfigFn func() error

// LoadFn loads a specific job manager once detected
type LoadFn func(*JM, *sys.Config) error

// SubmitFn is a "function pointer" that lets us job a new job
type SubmitFn func(*job.Job, *buildenv.Info, *sys.Config) (syexec.SyCmd, error)

// JM is the structure representing a specific JM
type JM struct {
	// ID identifies which job manager has been detected on the system
	ID string

	// Set is the function that sets the configuration of the current job manager
	Set SetConfigFn

	// Get is the function that gets the configuration of the current job manager
	Get GetConfigFn

	Load LoadFn

	// Submit is the function to submit a job through the current job manager
	Submit SubmitFn
}

// Detect figures out which job manager must be used on the system and return a
// structure that gather all the data necessary to interact with it
func Detect() JM {
	// Default job manager
	loaded, comp := NativeDetect()
	if !loaded {
		log.Fatalln("unable to find a default job manager")
	}

	// Now we check if we can find better
	loaded, slurmComp := SlurmDetect()
	if loaded {
		return slurmComp
	}

	return comp
}

// Load is the function to use to load the JM component
func Load(jm *JM) error {
	return nil
}

// TempFile creates a temporary file that is used to store a batch script
func TempFile(j *job.Job, env *buildenv.Info, sysCfg *sys.Config) error {
	filePrefix := "sbash-" + j.Container.Name
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
		path = filepath.Join(env.InstallDir, fileName)
		j.BatchScript = path
		if util.PathExists(path) {
			return sympierr.ErrFileExists
		}
		if j.Container.InstallDir == "" {
			j.Container.InstallDir = env.InstallDir
		}
	}

	j.CleanUp = func(...interface{}) error {
		err := os.RemoveAll(path)
		if err != nil {
			return fmt.Errorf("unable to delete %s: %s", path, err)
		}
		return nil
	}

	return nil
}
