// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package mpich

import (
	"github.com/sylabs/singularity-mpi/internal/pkg/deffile"
)

const (
	VersionTag = "MPICHVERSION"
	URLTag     = "MPICHURL"
	TarballTag = "MPICHTARBALL"
)

/*
func updateMPICHDefFile(myCfg *Config, sysCfg *sys.Config) error {
	var compileCfg compileConfig

	err := updateDeffile(myCfg, sysCfg, &compileCfg)
	if err != nil {
		return fmt.Errorf("failed to update MPICH definition file: %s", err)
	}

	return nil
}
*/

func MPICHGetExtraMpirunArgs() []string {
	var extraArgs []string
	return extraArgs
}

func MPICHGetConfigureExtraArgs() []string {
	var extraArgs []string
	return extraArgs
}

func GetDeffileTemplateTags() deffile.TemplateTags {
	var tags deffile.TemplateTags
	tags.Tarball = TarballTag
	tags.URL = URLTag
	tags.Version = VersionTag
	return tags
}

/*
func LoadMPICH(cfg *Config) (bool, Implementation) {
	var mpich Implementation
	if cfg.Implem.ID != MPICH {
		return false, mpich
	}
	cfg.Implem.GetExtraMpirunArgs = MPICHGetExtraMpirunArgs
	cfg.Implem.GetConfigureExtraArgs = MPICHGetConfigureExtraArgs
	return true, mpich
}
*/
