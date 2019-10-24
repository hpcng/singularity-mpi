// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package autotools

import (
	"bytes"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"

	util "github.com/sylabs/singularity-mpi/internal/pkg/util/file"
)

// Config represents the configuration of the autotools-compliant software to configure/compile/install
type Config struct {
	// Install is the path to the directory where the software should be installed
	Install string

	// Source is the path to the directory where the source code is
	Source string

	// ExtraConfigureArgs is a set of string that are passed to configure
	ExtraConfigureArgs []string
}

// Configure handles the classic configure commands
func Configure(cfg *Config) error {
	configurePath := filepath.Join(cfg.Source, "configure")
	if !util.FileExists(configurePath) {
		fmt.Printf("-> %s does not exist, skipping the configuration step\n", configurePath)
		return fmt.Errorf("%s does not exist, skipping the configuration step\n", configurePath)
	}

	var cmdArgs []string
	if cfg.Install != "" {
		cmdArgs = append(cmdArgs, "--prefix")
		cmdArgs = append(cmdArgs, cfg.Install)
	}
	if len(cfg.ExtraConfigureArgs) > 0 {
		cmdArgs = append(cmdArgs, cfg.ExtraConfigureArgs...)
	}

	log.Printf("-> Running 'configure': %s %s\n", configurePath, cmdArgs)
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(configurePath)
	if len(cmdArgs) > 0 {
		cmd = exec.Command(configurePath, cmdArgs...)
	}
	cmd.Dir = cfg.Source
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("command failed: %s - stdout: %s - stderr: %s", err, stdout.String(), stderr.String())
	}

	return nil
}
