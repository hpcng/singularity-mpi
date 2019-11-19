// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package sy

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/sylabs/singularity-mpi/internal/pkg/buildenv"
	"github.com/sylabs/singularity-mpi/internal/pkg/manifest"
	"github.com/sylabs/singularity-mpi/internal/pkg/sys"
	util "github.com/sylabs/singularity-mpi/internal/pkg/util/file"
)

func updateEnviron(buildEnv *buildenv.Info) []string {
	var newEnv []string

	env := os.Environ()
	if len(buildEnv.Env) > 0 {
		env = buildEnv.Env
	}

	tokens := strings.Split(buildEnv.SrcDir, "/")
	newGoPath := tokens[:len(tokens)-4]
	for _, e := range env {
		tokens := strings.Split(e, "=")
		if tokens[0] != "GOPATH" {
			newEnv = append(newEnv, e)
		}
	}

	newEnv = append(newEnv, "GOPATH=/"+filepath.Join(newGoPath...))
	return newEnv
}

// Configure is the function to call to configure Singularity
func Configure(env *buildenv.Info, sysCfg *sys.Config, extraArgs []string) error {
	// Singularity changed the mconfig flags over time so we need to figure out how the prefix is specified
	ctx, cancel := context.WithTimeout(context.Background(), sys.CmdTimeout*time.Minute)
	defer cancel()
	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, "./mconfig", "-h")
	cmd.Dir = env.SrcDir
	cmd.Stdout = &stdout
	cmd.Run() // mconfig -h always returns 2 (no idea why, it just does)

	args := []string{"--prefix=" + env.InstallDir}
	if strings.Contains(stdout.String(), "-p prefix") {
		args = []string{"-p", env.InstallDir}
	}

	if sysCfg.Nopriv {
		args = append(args, "--without-suid")
	}

	// At the point the install directory may not exist since it may be assumed it will
	// be created during the install command. If it is not there, we create it now so we
	// can store the manifest
	if !util.PathExists(env.InstallDir) {
		err := os.MkdirAll(env.InstallDir, 0755)
		if err != nil {
			return fmt.Errorf("failed to create %s: %s", env.InstallDir, err)
		}
	}
	singularityManifestPath := filepath.Join(env.InstallDir, "install.MANIFEST")
	err := manifest.Create(singularityManifestPath, args)
	if err != nil {
		return fmt.Errorf("failed to create installation manifest: %s", err)
	}

	// Run mconfig
	log.Printf("-> Executing from %s: ./mconfig %s\n", env.SrcDir, strings.Join(args, " "))
	newEnv := updateEnviron(env)
	env.Env = newEnv
	log.Printf("-> Using env: %s\n", strings.Join(newEnv, "\n"))
	var stderr bytes.Buffer
	cmd = exec.CommandContext(ctx, "./mconfig", args...)
	cmd.Dir = env.SrcDir
	cmd.Env = newEnv
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to run mconfig: %s (stderr: %s; stdout: %s)", err, stderr.String(), stdout.String())
	}

	return nil
}
