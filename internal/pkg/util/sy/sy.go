// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package sy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/sylabs/singularity-mpi/internal/pkg/mpi"
	"github.com/sylabs/singularity-mpi/internal/pkg/sys"
)

const (
	// KeyPassphrase is the name of the environment variable used to specify the passphrase of the key to be used to sign images
	KeyPassphrase = "SY_KEY_PASSPHRASE"

	// KeyIndex is the index of the key to use to sign images
	KeyIndex = "SY_KEY_INDEX"
)

func Sign(mpiCfg mpi.Config, sysCfg *sys.Config) error {
	var stdout, stderr bytes.Buffer

	log.Printf("-> Signing container (%s)", mpiCfg.ContainerPath)
	ctx, cancel := context.WithTimeout(context.Background(), sys.CmdTimeout*2*time.Minute)
	defer cancel()

	indexIdx := "0"
	if os.Getenv(KeyIndex) != "" {
		indexIdx = os.Getenv(KeyIndex)
	}

	cmd := exec.CommandContext(ctx, sysCfg.SingularityBin, "sign", "--keyidx", indexIdx, mpiCfg.ContainerPath)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		defer stdin.Close()
		passphrase := os.Getenv(KeyPassphrase)
		_, err := io.WriteString(stdin, passphrase)
		if err != nil {
			log.Fatal(err)
		}
	}()
	cmd.Dir = mpiCfg.BuildDir
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to execute command - stdout: %s; stderr: %s; err: %s", stdout.String(), stderr.String(), err)
	}

	return nil
}

func Upload(mpiCfg mpi.Config, sysCfg *sys.Config) error {
	var stdout, stderr bytes.Buffer

	log.Printf("-> Uploading container %s to %s", mpiCfg.ContainerPath, sysCfg.Registry)
	ctx, cancel := context.WithTimeout(context.Background(), sys.CmdTimeout*2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, sysCfg.SingularityBin, "push", mpiCfg.ContainerPath, sysCfg.Registry)
	cmd.Dir = mpiCfg.BuildDir
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to execute command - stdout: %s; stderr: %s; err: %s", stdout.String(), stderr.String(), err)
	}

	return nil
}
