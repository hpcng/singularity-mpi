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

// FileExists is a utility function that checks whether a file exists or not.
// If the file exits, FileExists return true; otherwise it returns false.
func FileExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// DectectTarballFormat detects the format of a tarball so we can know how
// to untar it.
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

// CopyFile is a utility functions that copies a file
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

// OpenResultsFile opens the result files associated to the experiments
func OpenResultsFile(filepath string) *os.File {
	mode := os.O_APPEND | os.O_RDWR
	if !FileExists(filepath) {
		mode = os.O_RDWR | os.O_CREATE
	}
	f, err := os.OpenFile(filepath, mode, 0755)
	if err != nil {
		log.Printf("failed to open file %s: %s", filepath, err)
		return nil
	}

	return f
}

// OpenLogFile opens the log file for the execution of a command
func OpenLogFile(mpiImplem string) *os.File {
	if mpiImplem == "" {
		return nil
	}

	filename := "singularity-" + mpiImplem + ".log"
	logFile, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Printf("failed to create log file: %s", err)
		return nil
	}

	return logFile
}

// DirInit ensures that the target directory is clean/empty
func DirInit(path string) error {
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		os.RemoveAll(path)
	}
	err := os.MkdirAll(path, 0755)
	if err != nil {
		log.Fatalf("failed to create scratch directory: %s", err)
	}

	return nil
}
