// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package network

import (
	"log"
	"os/exec"
)

const (
	IBForceKey = "force_ib"
)

func LoadInfiniband() (bool, Info) {
	var ib Info

	_, err := exec.LookPath("ibstat")
	if err != nil {
		log.Println("* Infiniband not detected")
		return false, ib
	}

	ib.ID = Infiniband

	return true, ib
}

func IBSetConfig() error {
	log.Println("* Infiniband detected, updating the configuration file")

	return nil
}
