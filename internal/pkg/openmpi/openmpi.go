// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package openmpi

import (
	"fmt"
	"log"

	"github.com/gvallee/kv/pkg/kv"
	"github.com/sylabs/singularity-mpi/internal/pkg/autotools"
	"github.com/sylabs/singularity-mpi/internal/pkg/buildenv"
	"github.com/sylabs/singularity-mpi/internal/pkg/deffile"
	"github.com/sylabs/singularity-mpi/internal/pkg/network"
	"github.com/sylabs/singularity-mpi/internal/pkg/sy"
	"github.com/sylabs/singularity-mpi/internal/pkg/sys"
)

const (
	// VersionTag is the tag used to refer to the MPI version in Open MPI template(s)
	VersionTag = "OMPIVERSION"

	// URLTag is the tag used to refer to the MPI URL in Open MPI template(s)
	URLTag = "OMPIURL"

	// TarballTag is the tag used to refer to the MPI tarball in Open MPI template(s)
	TarballTag = "OMPITARBALL"
)

// Configure executes the appropriate command to configure Open MPI on the target platform
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

// GetExtraMpirunArgs returns the set of arguments required for the mpirun command for the target platform
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

// GetExtraConfigureArgs returns the set of arguments required for configure to configure Open MPI on the target platform
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

// GetDeffileTemplateTags returns the tag names used by Open MPI in its template
func GetDeffileTemplateTags() deffile.TemplateTags {
	var tags deffile.TemplateTags
	tags.Version = VersionTag
	tags.URL = URLTag
	tags.Tarball = TarballTag
	return tags
}
