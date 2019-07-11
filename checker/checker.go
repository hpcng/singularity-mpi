// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package checker

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	cmdTimeout     = 10
	prereqBinaries = "wget gfortran gcc g++ make file"
)

// checkSingularityInstall makes sure that Singularity is correctly installed and works properly
func checkSingularityInstall() error {

	binPath, err := exec.LookPath("singularity")
	if err != nil {
		log.Printf("* Checking Singularity\tfail")
		return fmt.Errorf("failed to find singularity; please make sure Singularity is correctly installed: %s", err)
	}

	// Now we try to build a very simple image
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		log.Printf("* Checking Singularity\tfail")
		return fmt.Errorf("failed to create temporary directory: %s", err)
	}
	defer os.RemoveAll(dir)

	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout*time.Minute)
	defer cancel()
	singularityCmd := exec.CommandContext(ctx, binPath, "build", "alpine.sif", "library://sylabsed/examples/alpine")
	singularityCmd.Dir = dir
	err = singularityCmd.Run()
	if err != nil {
		log.Printf("* Checking Singularity\tfail")
		return fmt.Errorf("failed to build test image: %s", err)
	}

	log.Printf("* Checking Singularity\tsuccess")

	return nil
}

func checkPrereqBinaries() error {
	binaries := strings.Split(prereqBinaries, " ")

	for _, b := range binaries {
		_, err := exec.LookPath(b)
		if err != nil {
			log.Printf("* Checking %s\tfail", b)
			return fmt.Errorf("%s not found: %s", b, err)
		}
		log.Printf("* Checking %s\tsuccess", b)
	}
	return nil
}

// CheckSystemConfig checks the system configuration to ensure that the tool can run correctly
func CheckSystemConfig() error {
	err := checkSingularityInstall()
	if err != nil {
		return err
	}

	err = checkPrereqBinaries()
	if err != nil {
		return err
	}

	return nil
}
