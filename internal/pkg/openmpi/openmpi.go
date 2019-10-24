// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package openmpi

import (
	"fmt"
	"log"

	"github.com/sylabs/singularity-mpi/internal/pkg/kv"
	"github.com/sylabs/singularity-mpi/internal/pkg/network"

	"github.com/sylabs/singularity-mpi/internal/pkg/deffile"
	"github.com/sylabs/singularity-mpi/internal/pkg/util/sy"

	"github.com/sylabs/singularity-mpi/internal/pkg/autotools"
	"github.com/sylabs/singularity-mpi/internal/pkg/buildenv"
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
	/*
		if sys.IBEnabled {
			extraArgs = append(extraArgs, "--mca")
			extraArgs = append(extraArgs, "btl")
			extraArgs = append(extraArgs, "openib,self,vader")
		}
	*/

	return extraArgs
}

func GetExtraConfigureArgs(sysCfg *sys.Config) []string {
	var extraArgs []string
	if sysCfg.SlurmEnabled {
		extraArgs = append(extraArgs, "--with-slurm")
	}

	if sysCfg.IBEnabled {
		kvs, err := sy.LoadMPIConfigFile()
		if err != nil {
			log.Printf("[WARN] Unable to load the configuration of the tool; unable to fully Infiniband: %s\n", err)
			return extraArgs
		}

		mlxDir := kv.GetValue(kvs, network.MXMDirKey)
		if mlxDir == "" {
			log.Printf("[WARN] Infiniband detected but the MXM directory is undefined in the configuration file")
		} else {
			extraArgs = append(extraArgs, "--with-mxm="+mlxDir)
		}

		knemDir := kv.GetValue(kvs, network.KNEMDirKey)
		if knemDir == "" {
			log.Printf("[WARN] Infiniband detected but the KNEM directory is undefined in the configuration file")
		} else {
			extraArgs = append(extraArgs, "--with-knem="+knemDir)
		}
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
