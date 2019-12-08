// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
)

func getFileHash(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	hasher := sha256.New()
	_, err = io.Copy(hasher, f)
	if err != nil {
		return ""
	}

	return hex.EncodeToString(hasher.Sum(nil))
}

// Hash files returns the hash for a list of files (absolute path)
func HashFiles(files []string) []string {
	var hashData []string

	for _, file := range files {
		hash := getFileHash(file)
		hashData = append(hashData, file+": "+hash)
	}

	return hashData
}

// Create a new manifest
func Create(filepath string, entries []string) error {
	f, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("failed to create %s: %s", filepath, err)
	}

	_, err = f.WriteString(strings.Join(entries, "\n"))
	if err != nil {
		return fmt.Errorf("failed to write to %s: %s", filepath, err)
	}

	return nil
}
