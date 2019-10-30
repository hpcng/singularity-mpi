// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package app

import (
	"github.com/sylabs/singularity-mpi/internal/pkg/sys"
)

// GetNetpipe returns the app.Info structure with all the details for the
// netpipe test
func GetNetpipe(sysCfg *sys.Config) Info {
	var netpipe Info
	netpipe.Name = "NetPIPE-5.1.4"
	netpipe.BinPath = "/opt/NetPIPE-5.1.4/NPmpi"
	netpipe.Source = "http://netpipe.cs.ksu.edu/download/NetPIPE-5.1.4.tar.gz"
	netpipe.InstallCmd = "make mpi"
	return netpipe
}
