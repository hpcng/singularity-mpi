// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package sy

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"time"

	"github.com/sylabs/singularity-mpi/internal/pkg/buildenv"
	"github.com/sylabs/singularity-mpi/internal/pkg/sys"
)

// Configure is the function to call to configure Singularity
func Configure(env *buildenv.Info, sysCfg *sys.Config, extraArgs []string) error {
	// Run mconfig
	log.Printf("-> Executing from %s: ./mconfig --prefix=%s\n", env.BuildDir, env.InstallDir)
	ctx, cancel := context.WithTimeout(context.Background(), sys.CmdTimeout*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "./mconfig", "--prefix="+env.InstallDir)
	cmd.Dir = env.SrcDir
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to run mconfig: %s", err)
	}

	return nil
}
