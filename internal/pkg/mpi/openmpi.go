// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package mpi

import (
	"fmt"

	"github.com/sylabs/singularity-mpi/internal/pkg/sys"
)

func updateOMPIDefFile(myCfg *Config, sysCfg *sys.Config) error {
	var compileCfg compileConfig
	compileCfg.mpiVersionTag = "OMPIVERSION"
	compileCfg.mpiURLTag = "OMPIURL"
	compileCfg.mpiTarballTag = "OMPITARBALL"

	err := updateDeffile(myCfg, sysCfg, &compileCfg)
	if err != nil {
		return fmt.Errorf("failed to update Open MPI definition file: %s", err)
	}

	return nil
}
