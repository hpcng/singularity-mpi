// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package sy

import (
	"testing"

	"github.com/sylabs/singularity-mpi/internal/pkg/sys"

	"github.com/sylabs/singularity-mpi/internal/pkg/mpi"
)

func TestGetImageURL(t *testing.T) {
	var mpiCfg mpi.Config
	var sysCfg sys.Config

	mpiCfg.MpiImplm = mpi.OpenMPI
	mpiCfg.MpiVersion = "4.0.0"

	url := GetImageURL(&mpiCfg, &sysCfg)
	if url == "" {
		t.Fatalf("failed to get image URL")
	}
}
