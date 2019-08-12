// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package util

import (
	"fmt"
	"io"
	"log"
	"os"
	"path"
)

// Constants defining the format of the MPI package
const (
	// FormatBZ2 represents a bz2 file
	FormatBZ2 = "bz2"

	// FormatGZ represents a GZ file
	FormatGZ = "gz"

	// FormatTAR represents a simple TAR file
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

func CopyFile(src string, dst string) error {
	log.Printf("* Copying %s to %s", src, dst)
	// Check that the source file is valid
	srcStat, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("cannot access file %s: %s", src, err)
	}

	if !srcStat.Mode().IsRegular() {
		return fmt.Errorf("invalid source file %s: %s", src, err)
	}

	// Actually do the copy
	s, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open %s: %s", src, err)
	}
	defer s.Close()

	d, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create %s: %s", dst, err)
	}
	defer d.Close()

	_, err = io.Copy(d, s)
	if err != nil {
		return fmt.Errorf("unabel to copy file from %s to %s: %s", src, dst, err)
	}

	// Check whether the copy succeeded by comparing the sizes of the two files
	dstStat, err := d.Stat()
	if err != nil {
		return fmt.Errorf("unable to get stat for %s: %s", d.Name(), err)
	}
	if srcStat.Size() != dstStat.Size() {
		return fmt.Errorf("file copy failed, size is %d instead of %d", srcStat.Size(), dstStat.Size())
	}

	return nil
}
