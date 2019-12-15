// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package persistent

import (
	"path/filepath"

	"github.com/sylabs/singularity-mpi/pkg/implem"
	"github.com/sylabs/singularity-mpi/pkg/sys"
)

// GetPersistentHostMPIInstallDir returns the path to the directory where
// MPI should be installed when in persistent mode
func GetPersistentHostMPIInstallDir(mpi *implem.Info, sysCfg *sys.Config) string {
	return filepath.Join(sysCfg.Persistent, sys.MPIInstallDirPrefix+mpi.ID+"-"+mpi.Version)
}
