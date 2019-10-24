// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package network

import (
	"log"

	"github.com/sylabs/singularity-mpi/internal/pkg/sys"
)

const (
	Infiniband = "IB"
	Default    = "default"
)

type SaveFn func(sysCfg *sys.Config) error

type Info struct {
	ID   string
	Save SaveFn
}

func Detect(sysCfg *sys.Config) Info {
	loaded, comp := LoadDefault(sysCfg)
	if !loaded {
		log.Fatalln("unable to find a default network configuration")
	}

	loaded, ibComp := LoadInfiniband(sysCfg)
	if loaded {
		return ibComp
	}

	return comp
}
