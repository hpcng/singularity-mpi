// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package app

import (
	"path/filepath"

	"github.com/sylabs/singularity-mpi/pkg/sys"
)

// GetHelloworld returns the app.Info structure with all the details for our
// helloworld test
func GetHelloworld(sysCfg *sys.Config) Info {
	var hw Info

	hw.Name = "helloworld"
	hw.BinPath = "/opt/mpitest"
	hw.Source = "file://" + filepath.Join(sysCfg.TemplateDir, "mpitest.c")
	hw.ExpectedRankOutput = "Hello, I am rank #RANK/#NP"
	return hw
}
