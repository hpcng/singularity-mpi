// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package jm

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/sylabs/singularity-mpi/internal/pkg/mpi"

	"github.com/sylabs/singularity-mpi/internal/pkg/sys"
)

func TestSlurmSubmit(t *testing.T) {
	failed := false

	loaded, _ := LoadSlurm()
	if !loaded {
		t.Skip("slurm cannot be used on this platform")
	}

	var job Job
	var sysCfg sys.Config
	var mpiCfg mpi.Config
	job.ContainerCfg = &mpiCfg
	job.HostCfg = &mpiCfg
	installDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("failed to create temporary directory: %s", err)
	}
	defer os.RemoveAll(installDir)
	job.HostCfg.InstallDir = installDir
	sysCfg.ScratchDir, err = ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("unable to create scratch directory: %s", err)
	}
	defer os.RemoveAll(sysCfg.ScratchDir)

	launcher, err := SlurmSubmit(&job, &sysCfg)
	if err != nil {
		t.Fatalf("test failed: %s", err)
	}

	if launcher.Cmd != "sbatch" {
		failed = true
		t.Logf("wrong launcher returned")
	}

	t.Logf("Batch script: %s", job.BatchScript)
	// Display the content of the batch script
	if !failed {
		f, err := os.Open(job.BatchScript)
		if err != nil {
			failed = true
			t.Logf("failed to open batch script %s: %s", job.BatchScript, err)
		} else {
			b, err := ioutil.ReadAll(f)
			if err != nil {
				t.Logf("failed to read the batch script: %s", err)
			}
			t.Logf("Content of the batch script:\n")
			t.Logf(string(b))
		}
		defer f.Close()
	}

	/*
		err = job.CleanUp()
		if err != nil {
			t.Logf("failed to call the cleanup function: %s", err)
			failed = true
		}
	*/

	if failed {
		t.Fatalf("test failed")
	}
	t.Logf("Slurm launcher - cmd: %s; cmd args: %s\n", launcher.Cmd, launcher.CmdArgs)
	t.Logf("Slurm batch script: %s\n", job.BatchScript)

}
