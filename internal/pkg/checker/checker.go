// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package checker

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/sylabs/singularity-mpi/internal/pkg/sympierr"
)

const (
	cmdTimeout     = 10
	prereqBinaries = "wget gfortran gcc g++ make file"
)

// CheckDefFile does some checking on a definition file to ensure it can be used
func CheckDefFile(path string) error {
	log.Printf("* Checking definition file %s...", path)
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open %s: %s", path, err)
	}
	defer f.Close()

	// For now we just check the first line that should start with "Bootstrap:"
	scanner := bufio.NewScanner(f)
	tok := scanner.Scan()
	if tok {
		line := scanner.Text()
		if !strings.HasPrefix(line, "Bootstrap:") {
			return fmt.Errorf("%s is an invalid definition file", path)
		}
	} else {
		return fmt.Errorf("* Error while scanning the definition file: %s", scanner.Err())
	}

	log.Println("... successfully checked the definition file.")
	return nil
}

// checkSingularityInstall makes sure that Singularity is correctly installed and works properly
func checkSingularityInstall() error {

	binPath, err := exec.LookPath("singularity")
	if err != nil {
		log.Printf("* Checking for Singularity\tfail")
		return sympierr.ErrSingularityNotInstalled
	}

	// Now we try to build a very simple image
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		log.Printf("* Checking for Singularity\tfail")
		return fmt.Errorf("failed to create temporary directory: %s", err)
	}
	defer os.RemoveAll(dir)

	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout*time.Minute)
	defer cancel()
	singularityCmd := exec.CommandContext(ctx, binPath, "build", "alpine.sif", "library://sylabsed/examples/alpine")
	singularityCmd.Dir = dir
	err = singularityCmd.Run()
	if err != nil {
		log.Printf("* Checking for Singularity\tfail")
		return fmt.Errorf("failed to build test image: %s", err)
	}

	log.Printf("* Checking for Singularity\tpass")

	return nil
}

func checkPrereqBinaries() error {
	binaries := strings.Split(prereqBinaries, " ")

	for _, b := range binaries {
		_, err := exec.LookPath(b)
		if err != nil {
			log.Printf("* Checking for %s\tfail", b)
			return fmt.Errorf("%s not found: %s", b, err)
		}
		log.Printf("* Checking for %s\tpass", b)
	}
	return nil
}

// CheckSystemConfig checks the system configuration to ensure that the tool can run correctly
func CheckSystemConfig() error {
	err := checkSingularityInstall()
	if err != nil && err != sympierr.ErrSingularityNotInstalled {
		return err
	}

	prereqErr := checkPrereqBinaries()
	if prereqErr != nil {
		return prereqErr
	}

	return err
}

// CheckBuildPrivilege checks if we can build an image for a definition file on the system
func CheckBuildPrivilege() error {
	binPath, err := exec.LookPath("sudo")
	if err != nil {
		return fmt.Errorf("failed to find sudo: %s", err)
	}

	// Now we try to build a very simple image
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		return fmt.Errorf("failed to create temporary directory: %s", err)
	}
	defer os.RemoveAll(dir)

	// Create a dummy def file
	dummyDefFile := filepath.Join(dir, "test.def")
	defFileContent := []byte("Bootstrap: docker\nFrom: alpine")
	// The file is deteled when the temporary directory is deleted
	err = ioutil.WriteFile(dummyDefFile, defFileContent, 0644)
	if err != nil {
		return fmt.Errorf("Impossible to create definition file: %s", err)
	}

	// Try to run the Singularity command
	testImg := filepath.Join(dir, "test.sif")
	log.Printf("* Trying to create image with: sudo singularity build %s %s\n", testImg, dummyDefFile)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute) // We try only for one minute
	defer cancel()
	singularityCmd := exec.CommandContext(ctx, binPath, "singularity", "build", testImg, dummyDefFile)
	singularityCmd.Dir = dir
	err = singularityCmd.Run()
	if err != nil {
		return fmt.Errorf("failed to build test image: %s", err)
	}

	return nil
}
