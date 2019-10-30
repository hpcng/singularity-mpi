// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package app

import (
	"github.com/sylabs/singularity-mpi/internal/pkg/sys"
)

// GetIMB returns the app.Info structure with all the details for the
// IMB test
func GetIMB(sysCfg *sys.Config) Info {
	var imb Info
	imb.Name = "IMB"
	imb.BinPath = "/opt/mpi-benchmarks/IMB-MPI1"
	imb.Source = "https://github.com/intel/mpi-benchmarks.git"
	imb.InstallCmd = "CC=mpicc CXX=mpic++ make IMB-MPI1"
	return imb
}
