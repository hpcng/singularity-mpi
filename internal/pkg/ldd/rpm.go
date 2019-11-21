// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package ldd

import (
	"bytes"
	"context"
	"log"
	"os/exec"
	"strings"
	"time"

	"github.com/sylabs/singularity-mpi/internal/pkg/sys"
)

// RPMGetDependencies parses the ldd output and figure out the required
// dependencies in term of Debian packages
func RPMGetDependencies(output string) []string {
	var dependencies []string

	// Get path to rpm
	rpmPath, err := exec.LookPath("rpm")
	if err != nil {
		log.Println("[WARN] cannot find rpm")
		return dependencies
	}

	lines := strings.Split(output, "\n")

	ctx, cancel := context.WithTimeout(context.Background(), sys.CmdTimeout*time.Second)
	defer cancel()

	// the package of interest is the one for the current architecture
	for i := 0; i < len(lines); i++ {
		words := strings.Split(lines[i], " ")
		words[0] = strings.Trim(words[0], " \t")
		// Run rpm -qf <file>
		cmd := exec.CommandContext(ctx, rpmPath, "--qf", "'%{NAME}", "-qf", words[0])
		var rpmStdout, rpmStderr bytes.Buffer
		cmd.Stdout = &rpmStdout
		cmd.Stderr = &rpmStderr
		err = cmd.Run()
		if err != nil {
			log.Printf("rpm returned an error for %s, skipping... (%s; stdout: %s; stderr: %s)", words[0], err, rpmStdout.String(), rpmStderr.String())
			continue
		}

		if rpmStdout.String() != "" {
			dependencies = append(dependencies, rpmStdout.String())
		}
	}

	return dependencies
}

// RpmLoad is the function called to see if the module is usable on the
// current system. If so, the module structure returned has all the functions
// required for RPM-based systems.
func RPMLoad() (bool, Module) {
	var RPM Module
	RPM.GetDependencies = RPMGetDependencies

	// Get path to rpm
	_, err := exec.LookPath("rpm")
	if err != nil {
		return false, RPM
	}

	return true, RPM
}
