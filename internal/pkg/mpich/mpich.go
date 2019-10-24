// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package mpich

import (
	"github.com/sylabs/singularity-mpi/internal/pkg/deffile"
)

const (
	// VersionTag is the tag used to refer to the MPI version in MPICH template(s)
	VersionTag = "MPICHVERSION"
	// URLTag is the tag used to refer to the MPI URL in MPICH template(s)
	URLTag     = "MPICHURL"
	// TarballTag is the tag used to refer to the MPI tarball in MPICH template(s)
	TarballTag = "MPICHTARBALL"
)

// MPICHGetExtraMpirunArgs returns the extra mpirun arguments required by MPICH for a specific configuration
func MPICHGetExtraMpirunArgs() []string {
	var extraArgs []string
	return extraArgs
}

// MPICHGetConfigureExtraArgs returns the extra arguments required to configure MPICH
func MPICHGetConfigureExtraArgs() []string {
	var extraArgs []string
	return extraArgs
}

// GetDeffileTemplateTags returns the tags used on the MPICH template(s)
func GetDeffileTemplateTags() deffile.TemplateTags {
	var tags deffile.TemplateTags
	tags.Tarball = TarballTag
	tags.URL = URLTag
	tags.Version = VersionTag
	return tags
}

