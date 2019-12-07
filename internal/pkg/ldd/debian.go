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
	"runtime"
	"strings"
	"time"

	"github.com/sylabs/singularity-mpi/internal/pkg/sys"
)

// todo: should be in a util package
func isInSlice(s []string, w string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == w {
			return true
		}
	}

	return false
}

func parseDpkgOutput(dependencies []string, output string) []string {
	arch := runtime.GOARCH
	lines := strings.Split(output, "\n")
	for i := 0; i < len(lines); i++ {
		words := strings.Split(lines[i], ":")
		if len(words) >= 2 && words[1] == arch {
			if !isInSlice(dependencies, words[0]) {
				if len(dependencies) == 0 {
					dependencies = []string{words[0]}
				} else {
					dependencies = append(dependencies, words[0])
				}
			}
		}
	}

	return dependencies
}

// DebianGetDependencies parses the ldd output and figure out the required
// dependencies in term of Debian packages
func DebianGetDependencies(output string) []string {
	var dependencies []string

	// Get path to dpkg
	dpkgPath, err := exec.LookPath("dpkg")
	if err != nil {
		log.Println("[WARN] cannot find dpkg")
		return dependencies
	}

	lines := strings.Split(output, "\n")

	ctx, cancel := context.WithTimeout(context.Background(), sys.CmdTimeout*time.Minute)
	defer cancel()

	// the package of interest is the one for the current architecture
	for i := 0; i < len(lines); i++ {
		words := strings.Split(lines[i], " ")
		words[0] = strings.Trim(words[0], " \t")
		// Run dpkg -S <file>
		cmd := exec.CommandContext(ctx, dpkgPath, "-S", words[0])
		var dpkgStdout, dpkgStderr bytes.Buffer
		cmd.Stdout = &dpkgStdout
		cmd.Stderr = &dpkgStderr
		err = cmd.Run()
		if err != nil {
			log.Printf("dpkg returned an error for %s, skipping... (%s; stdout: %s; stderr: %s)", words[0], err, dpkgStdout.String(), dpkgStderr.String())
			continue
		}

		dependencies = parseDpkgOutput(dependencies, dpkgStdout.String())
	}

	return dependencies
}

// DebianLoad is the function called to see if the module is usable on the
// current system. If so, the module structure returned has all the functions
// required for Debian based systems.
func DebianLoad() (bool, Module) {
	var Debian Module
	Debian.GetDependencies = DebianGetDependencies

	// Get path to dpkg
	_, err := exec.LookPath("dpkg")
	if err != nil {
		return false, Debian
	}

	return true, Debian
}
