// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package job

import (
	"bytes"

	"github.com/sylabs/singularity-mpi/internal/pkg/implem"
	"github.com/sylabs/singularity-mpi/pkg/app"
	"github.com/sylabs/singularity-mpi/pkg/container"
	"github.com/sylabs/singularity-mpi/pkg/sys"
)

// CleanUpFn is a "function pointer" to call to clean up the system after the completion of a job
type CleanUpFn func(...interface{}) error

// GetOutputFn is a "function pointer" to call to gather the output of an application after completion of a job
type GetOutputFn func(*Job, *sys.Config) string

// GetErrorFn is a "function pointer" to call to gather stderr from an application after completion of a job
type GetErrorFn func(*Job, *sys.Config) string

// Job represents a job
type Job struct {
	// NP is the number of ranks
	NP int

	// NNodes is the number of nodes
	NNodes int

	// CleanUp is the function to call once the job is completed to clean the system
	CleanUp CleanUpFn

	// BatchScript is the path to the script required to start a job (optional)
	BatchScript string

	// HostCfg is the MPI configuration to use on the host
	HostCfg *implem.Info

	// ContainerCfg is the MPI configuration to use in the container
	Container *container.Config

	// App is the path to the application's binary, i.e., the binary to start
	App app.Info

	// OutBuffer is a buffer with the output of the job
	OutBuffer bytes.Buffer

	// ErrBuffer is a buffer with the stderr of the job
	ErrBuffer bytes.Buffer

	// GetOutput is the function to call to gather the output of the application based on the use of a given job manager
	GetOutput GetOutputFn

	// GetError is the function to call to gather stderr of the application based on the use of a given job manager
	GetError GetErrorFn

	// Args is a set of arguments to be used for launching the job
	Args []string
}
