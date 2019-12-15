// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package distro

import (
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/gvallee/go_util/pkg/util"
	"github.com/sylabs/singularity-mpi/internal/pkg/sys"
)

// ID represents a Linux distribution
type ID struct {
	// Name is the name of the Linux distribution, e.g., ubuntu
	Name string

	// Version is the version of the Linux distribution, e.g., 7, 19.04
	Version string

	// Codename is the codename of the Linux distribution, e.g., disco (can be empty)
	Codename string
}

// GetBaseImageLibraryURL returns the library URL to use as base image (when possible)
func GetBaseImageLibraryURL(linuxDistro ID, sysCfg *sys.Config) string {
	configFile := filepath.Join(sysCfg.EtcDir, "sympi_"+linuxDistro.Name+".conf")

	if !util.FileExists(configFile) {
		return ""
	}

	data, err := ioutil.ReadFile(configFile)
	if err != nil {
		return ""
	}

	content := string(data)
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		words := strings.Split(line, "\t")
		// The configuration file may be based on the codename or version
		if len(words) == 2 && words[0] == linuxDistro.Version || words[0] == linuxDistro.Codename {
			return words[1]
		}
	}

	return ""
}

// ParseDescr parses the description string of a Linux distribution
// (e.g., centos:6) to a ID structure
func ParseDescr(descr string) ID {
	id := ID{
		Name:     "",
		Version:  "",
		Codename: "",
	}
	tokens := strings.Split(descr, ":")
	if len(tokens) != 2 {
		return id
	}

	id.Name = tokens[0]
	if id.Name == "ubuntu" {
		id.Codename = tokens[1]
		id.Version = ubuntuCodenameToVersion(id.Codename)
	} else {
		id.Version = tokens[1]
	}

	return id
}
