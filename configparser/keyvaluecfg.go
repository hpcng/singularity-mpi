// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package configparser

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// OFIConfig is the structure gathering all the configuration details relevant for OFI.
// These details are loaded from the tool's OFI configuration file.
type OFIConfig struct {
	Ifnet string
}

// LoadOFIConfig reads the OFI configuration file and return the associated data structure.
func LoadOFIConfig(filepath string) (*OFIConfig, error) {
	f, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("cannot open %s: %s", filepath, err)
	}
	defer f.Close()

	config := new(OFIConfig)

	lineReader := bufio.NewScanner(f)
	for lineReader.Scan() {
		line := lineReader.Text()

		// We skip comments
		re1 := regexp.MustCompile(`\s*#`)
		commentMatch := re1.FindStringSubmatch(line)
		if len(commentMatch) != 0 {
			continue
		}

		// For valid lines, we separate the key from the value
		words := strings.Split(line, "=")
		if len(words) != 2 {
			return nil, fmt.Errorf("invalid entry format: %s", line)
		}
		key := words[0]
		value := words[1]
		key = strings.Trim(key, " \t")
		value = strings.Trim(value, " \t")

		switch key {
		case "ifnet":
			if value == "<your network interface" {
				return nil, fmt.Errorf("ifnet is not properly defined in %s, please update your configuration file", filepath)
			}
			config.Ifnet = value
		}
	}

	return config, nil
}
