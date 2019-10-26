// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package app

import (
	"path/filepath"

	"github.com/sylabs/singularity-mpi/internal/pkg/sys"
)

func GetHelloworld(sysCfg *sys.Config) Info {
	var hw Info

	hw.Name = "helloworld"
	hw.BinPath = "/opt/mpitest"
	hw.Source = "file://" + filepath.Join(sysCfg.TemplateDir, "mpitest.c")
	return hw
}
