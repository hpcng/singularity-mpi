// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package jm

import (
	"testing"

	"github.com/gvallee/go_util/pkg/util"
	"github.com/sylabs/singularity-mpi/internal/pkg/buildenv"
	"github.com/sylabs/singularity-mpi/internal/pkg/container"
	"github.com/sylabs/singularity-mpi/internal/pkg/job"
	"github.com/sylabs/singularity-mpi/internal/pkg/sys"
)

// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

func TestDetect(t *testing.T) {
	jm := Detect()
	t.Logf("Selected job manager: %s\n", jm.ID)
}

func TestTempFile(t *testing.T) {
	var j job.Job
	var containerCfg container.Config
	var sysCfg sys.Config
	var env buildenv.Info

	containerCfg.Name = "containerName"
	j.Container = &containerCfg

	err := TempFile(&j, &env, &sysCfg)
	if err != nil {
		t.Fatalf("unable to create temporary file: %s", err)
	}
	if j.BatchScript == "" {
		t.Fatalf("temporary file path is undefined")
	}

	t.Logf("Temporary file is: %s\n", j.BatchScript)
	err = j.CleanUp()
	if err != nil {
		t.Fatalf("failed to clean up: %s", err)
	}

	if util.PathExists(j.BatchScript) {
		t.Fatalf("temporary file %s still exists even after cleanup", j.BatchScript)
	}
}
