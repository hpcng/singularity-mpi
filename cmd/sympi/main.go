// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package main

import (
	"flag"
	"fmt"

	"github.com/sylabs/singularity-mpi/internal/pkg/sys"
)

func displayInstalled(dir string) {
	fmt.Println("do something")
}

func main() {
	list := flag.Bool("list", false, "List all MPI on the host and all MPI containers")

	flag.Parse()

	sympiDir := sys.GetSympiDir()

	if *list {
		displayInstalled(sympiDir)
	}
}
