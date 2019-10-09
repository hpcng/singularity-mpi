// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package openmpi

import (
	"fmt"

	"github.com/sylabs/singularity-mpi/internal/pkg/deffile"

	"github.com/sylabs/singularity-mpi/internal/pkg/autotools"
	"github.com/sylabs/singularity-mpi/internal/pkg/buildenv"
	"github.com/sylabs/singularity-mpi/internal/pkg/network"
	"github.com/sylabs/singularity-mpi/internal/pkg/sys"
)

const (
	VersionTag = "OMPIVERSION"
	URLTag     = "OMPIURL"
	TarballTag = "OMPITARBALL"
)

func Configure(env *buildenv.Info, sysCfg *sys.Config, extraArgs []string) error {
	var ac autotools.Config

	ac.Install = env.InstallDir
	ac.Source = env.SrcDir
	ac.ExtraConfigureArgs = extraArgs

	err := autotools.Configure(&ac)
	if err != nil {
		return fmt.Errorf("Unable to run configure: %s", err)
	}

	return nil
}

func GetExtraMpirunArgs(sys *sys.Config) []string {
	var extraArgs []string
	if sys.Network.ID == network.Infiniband {
		extraArgs = append(extraArgs, "--mca")
		extraArgs = append(extraArgs, "btl")
		extraArgs = append(extraArgs, "openib,self,vader")
	}

	return extraArgs
}

func GetExtraConfigureArgs(sysCfg *sys.Config) []string {
	var extraArgs []string
	if sysCfg.SlurmEnabled {
		return []string{"--with-slurm"}
	}

	return extraArgs
}

func GetDeffileTemplateTags() deffile.TemplateTags {
	var tags deffile.TemplateTags
	tags.Version = VersionTag
	tags.URL = URLTag
	tags.Tarball = TarballTag
	return tags
}

/*
func LoadOpenMPI(cfg implem.Info) (bool, builder.Config) {
	var builder builder.Config
	if cfg.ID != ID {
		return false, builder
	}
	builder.GetExtraMpirunArgs = OMPIGetExtraMpirunArgs
	return true, builder
}
*/
