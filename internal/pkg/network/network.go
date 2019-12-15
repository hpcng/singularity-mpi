// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package network

import (
	"log"

	"github.com/sylabs/singularity-mpi/pkg/sys"
)

const (
	// Infiniband is the ID used to identify Infiniband
	Infiniband = "IB"
	// Default is the ID used to identify the default networking configuration
	Default = "default"
)

// SaveFn is a function of a component to save the network configuration in a configuration file
type SaveFn func(sysCfg *sys.Config) error

// Info is a structure storing the details about the network on the system
type Info struct {
	ID   string
	Save SaveFn
}

// Detect is the function called to detect the network on the system and load the corresponding networking component
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
