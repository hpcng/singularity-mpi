// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package util

import (
	"os"
	"path"
)

// Constants defining the format of the MPI package
const (
	FormatBZ2 = "bz2"
	FormatGZ  = "gz"
	FormatTAR = "tar"
)

func FileExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func DetectTarballFormat(filepath string) string {
	if path.Ext(filepath) == ".bz2" {
		return FormatBZ2
	}

	if path.Ext(filepath) == ".gz" {
		return FormatGZ
	}

	if path.Ext(filepath) == ".tar" {
		return FormatTAR
	}

	return ""
}
