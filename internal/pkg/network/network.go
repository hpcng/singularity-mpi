// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package network

import "log"

const (
	Infiniband = "IB"
	Default    = "default"
)

type Info struct {
	ID string
}

func Detect() Info {
	loaded, comp := LoadDefault()
	if !loaded {
		log.Fatalln("unable to find a default network configuration")
	}

	loaded, ibComp := LoadInfiniband()
	if loaded {
		return ibComp
	}

	return comp
}
