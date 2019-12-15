// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package main

import (
	"io"
	"log"
	"os"

	"github.com/gvallee/go_util/pkg/util"
	"github.com/sylabs/singularity-mpi/pkg/sympi"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("%s requires at least one argument, a container name reported by the 'sympi -list' command.", os.Args[0])
	}

	logFile := util.OpenLogFile("syryun")
	nultiWriters := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(nultiWriters)
	sysCfg := sympi.GetDefaultSysConfig()
	sysCfg.Verbose = true

	var args []string
	for i := 1; i < len(os.Args)-1; i++ {
		args = append(args, os.Args[i])
	}

	err := sympi.RunContainer(os.Args[len(os.Args)-1], args, &sysCfg)
	if err != nil {
		log.Fatalf("impossible to run container %s: %s", os.Args[1], err)
	}
}
