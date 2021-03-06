// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package network

import (
	"fmt"
	"log"
	"os/exec"
	"strconv"

	"github.com/gvallee/kv/pkg/kv"
	"github.com/sylabs/singularity-mpi/pkg/sy"
	"github.com/sylabs/singularity-mpi/pkg/sys"
)

const (
	// IBForceKey is the key used in the configuration file to specific in Infiniband should always be used
	IBForceKey = "force_ib"

	// MXMDirKey is the key used in the configuration file to specify where MXM files are installed
	MXMDirKey = "mxm_dir"

	// KNEMDirKey is the key used in the configuration file to specify where knem files are installed
	KNEMDirKey = "knem_dir"
)

// LoadInfiniband is the function called to load the IB component
func LoadInfiniband(sysCfg *sys.Config) (bool, Info) {
	var ib Info

	_, err := exec.LookPath("ibstat")
	if err != nil {
		log.Println("* Infiniband not detected")
		return false, ib
	}

	log.Println("* Infiniband detected, updating the configuration file")
	ib.ID = Infiniband

	// We always check the config file just in case the user disabled IB
	kvs, err := sy.LoadMPIConfigFile()
	if err != nil {
		log.Printf("[WARN] Unable to load the configuration of the tool: %s\n", err)
		return false, ib
	}

	currentStatus := kv.GetValue(kvs, IBForceKey)
	if currentStatus == "" {
		sysCfg.IBEnabled = true
		// If the config file does not have a key for us, we create one
		log.Println("* Configuration file does not an entry about IB yet")
		err = IBSave(sysCfg)
		if err != nil {
			log.Printf("[WARN] unable to save IB configuration: %s", err)
		}
	} else {
		sysCfg.IBEnabled, err = strconv.ParseBool(currentStatus)
		if err != nil {
			log.Printf("[WARN] unable to set system parameter: %s", err)
			return false, ib
		}
	}

	return true, ib
}

// IBSave saves the IB configuration on the system into the tool's configuration file.
func IBSave(sysCfg *sys.Config) error {
	err := sy.ConfigFileUpdateEntry(sy.GetPathToSyMPIConfigFile(), IBForceKey, strconv.FormatBool(sysCfg.IBEnabled))
	if err != nil {
		return fmt.Errorf("unable to save IB configuration: %s", err)
	}

	return nil
}
