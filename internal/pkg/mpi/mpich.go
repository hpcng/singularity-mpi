// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package mpi

import (
	"fmt"

	"github.com/sylabs/singularity-mpi/internal/pkg/sys"
)

func updateMPICHDefFile(myCfg *Config, sysCfg *sys.Config) error {
	var compileCfg compileConfig
	compileCfg.mpiVersionTag = "MPICHVERSION"
	compileCfg.mpiURLTag = "MPICHURL"
	compileCfg.mpiTarballTag = "MPICHTARBALL"

	err := updateDeffile(myCfg, sysCfg, &compileCfg)
	if err != nil {
		return fmt.Errorf("failed to update MPICH definition file: %s", err)
	}

	return nil
}
