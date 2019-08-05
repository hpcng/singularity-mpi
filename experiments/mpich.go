// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package experiments

import "fmt"

func updateMPICHDefFile(myCfg *mpiConfig, sysCfg *SysConfig) error {
	var compileCfg compileConfig
	compileCfg.mpiVersionTag = "MPICHVERSION"
	compileCfg.mpiURLTag = "MPICHURL"
	compileCfg.mpiTarballTag = "MPICHTARBALL"

	err := doUpdateDefFile(myCfg, sysCfg, &compileCfg)
	if err != nil {
		return fmt.Errorf("failed to update MPICH definition file: %s", err)
	}

	return nil
}
