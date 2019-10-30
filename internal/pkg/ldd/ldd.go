// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package ldd

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os/exec"
	"time"

	"github.com/sylabs/singularity-mpi/internal/pkg/sys"
)

// GetDependenciesFn is a function "pointer" for a distribution-specific
// function that parses the output of ldd and find the binary packages associated
// to the dependencies expressed in the ldd output.
type GetDependenciesFn func(string) []string

// Module represents a distribution-specific module that can handle output
// from ldd.
type Module struct {
	GetDependencies GetDependenciesFn
}

// GetPackageDependenciesForFile finds all the binary-package dependencies
// for a specific file, by running ldd and the appropriate module for the
// target linux distribution
func (m *Module) GetPackageDependenciesForFile(file string) []string {
	var dependencies []string

	// Get the path to ldd
	lddPath, err := exec.LookPath("ldd")
	if err != nil {
		log.Println("[WARN] cannot find ldd")
		return dependencies
	}

	// Run ldd against the binary
	ctx, cancel := context.WithTimeout(context.Background(), sys.CmdTimeout*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, lddPath, file)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err != nil {
		log.Printf("failed to execute dpkg: %s; stdout: %s; stderr: %s", err, stdout.String(), stderr.String())
		return dependencies
	}

	// Parse the result
	dependencies = m.GetDependencies(stdout.String())

	return dependencies
}

// Detect finds the ldd module applicable to the current system
func Detect() (Module, error) {
	loaded, mod := DebianLoad()
	if loaded {
		return mod, nil
	}

	var dummyModule Module
	return dummyModule, fmt.Errorf("unable to find usable ldd module")
}
