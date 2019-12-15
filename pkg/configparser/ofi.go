// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package configparser

import (
	"fmt"

	"github.com/gvallee/kv/pkg/kv"
)

// OFIConfig is the structure gathering all the configuration details relevant for OFI.
// These details are loaded from the tool's OFI configuration file.
type OFIConfig struct {
	// Ifnet is the identifier of the network interface to use
	Ifnet string
}

// LoadOFIConfig reads the OFI configuration file and return the associated data structure.
func LoadOFIConfig(filepath string) (*OFIConfig, error) {
	kvs, err := kv.LoadKeyValueConfig(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to load key/value from %s: %s", filepath, err)
	}

	config := new(OFIConfig)

	for _, kv := range kvs {
		switch kv.Key {
		case "ifnet":
			if kv.Value == "<your network interface>" {
				return nil, fmt.Errorf("ifnet is not properly defined in %s, please update your configuration file", filepath)
			}
			config.Ifnet = kv.Value
		}

	}

	return config, nil
}
