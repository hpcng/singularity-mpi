// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package jm

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sylabs/singularity-mpi/internal/pkg/mpi"
	"github.com/sylabs/singularity-mpi/internal/pkg/sympierr"

	"github.com/sylabs/singularity-mpi/internal/pkg/kv"
	"github.com/sylabs/singularity-mpi/internal/pkg/sys"

	"github.com/sylabs/singularity-mpi/internal/pkg/util/sy"
)

const (
	SlurmPartitionKey = "slurm_partition"
)

func LoadSlurm() (bool, JM) {
	var jm JM

	_, err := exec.LookPath("sbatch")
	if err != nil {
		log.Println("* Slurm not detected")
		return false, jm
	}

	jm.ID = SlurmID
	jm.Set = SlurmSetConfig
	jm.Get = SlurmGetConfig
	jm.Submit = SlurmSubmit

	return true, jm
}

func SlurmGetConfig() error {
	return nil
}

func SlurmSetConfig() error {
	log.Println("* Slurm detected, updating singularity-mpi configuration file")
	configFile := sy.GetPathToSyMPIConfigFile()

	err := sy.ConfigFileUpdateEntry(configFile, sys.SlurmEnabledKey, "true")
	if err != nil {
		return fmt.Errorf("failed to update entry %s in %s: %s", sys.SlurmEnabledKey, configFile, err)
	}
	return nil
}

const (
	slurmScriptCmdPrefix = "#SBATCH"
)

func generateJobScript(j *Job, sysCfg *sys.Config, kvs []kv.KV) error {
	// Sanity checks
	if j == nil {
		return fmt.Errorf("undefined job")
	}

	// Some sanity checks
	if j.HostCfg == nil {
		return fmt.Errorf("undefined host configuration")
	}

	if j.HostCfg.InstallDir == "" {
		return fmt.Errorf("undefined host installation directory")
	}

	if sysCfg.ScratchDir == "" {
		return fmt.Errorf("undefined scratch directory")
	}

	if j.AppBin == "" {
		return fmt.Errorf("application binary is undefined")
	}

	// Create the batch script
	err := TempFile(j, sysCfg)
	if err != nil {
		if err == sympierr.ErrFileExists {
			log.Printf("* Script %s already esists, skipping\n", j.BatchScript)
			return nil
		}
		return fmt.Errorf("unable to create temporary file: %s", err)
	}

	scriptText := "#!/bin/bash\n#\n"
	partition := kv.GetValue(kvs, SlurmPartitionKey)
	if partition != "" {
		scriptText += slurmScriptCmdPrefix + " --partition=" + partition + "\n"
	}

	if j.NNodes > 0 {
		scriptText += slurmScriptCmdPrefix + " --nodes=" + strconv.FormatInt(j.NNodes, 10) + "\n"
	}

	if j.NP > 0 {
		scriptText += slurmScriptCmdPrefix + " --ntasks=" + strconv.FormatInt(j.NP, 10) + "\n"
	}

	errorFilename := j.ContainerCfg.ContainerName + ".err"
	scriptText += slurmScriptCmdPrefix + " --error=" + filepath.Join(sysCfg.ScratchDir, errorFilename) + "\n"
	outputFilename := j.ContainerCfg.ContainerName + ".out"
	scriptText += slurmScriptCmdPrefix + " --output=" + filepath.Join(sysCfg.ScratchDir, outputFilename) + "\n"

	// Set PATH and LD_LIBRARY_PATH
	scriptText += "\nexport PATH=" + j.HostCfg.InstallDir + "/bin:$PATH\n"
	scriptText += "export LD_LIBRARY_PATH=" + j.HostCfg.InstallDir + "/lib:$LD_LIBRARY_PATH\n\n"

	// Add the mpirun command
	mpirunPath := filepath.Join(j.HostCfg.InstallDir, "bin", "mpirun")
	mpirunArgs, err := mpi.GetMpirunArgs(j.HostCfg, j.ContainerCfg)
	if err != nil {
		return fmt.Errorf("unable to get mpirun arguments: %s", err)
	}
	scriptText += "\n" + mpirunPath + " " + strings.Join(mpirunArgs, " ") + " " + j.AppBin + "\n"

	err = ioutil.WriteFile(j.BatchScript, []byte(scriptText), 0644)
	if err != nil {
		return fmt.Errorf("unable to write to file %s: %s", j.BatchScript, err)
	}

	return nil
}

// SlurmSubmit prepares the batch script necessary to start a given job.
//
// Note that a script does not need any specific environment to be submitted
func SlurmSubmit(j *Job, sysCfg *sys.Config) (Launcher, error) {
	var l Launcher
	l.Cmd = "sbatch"
	l.CmdArgs = append(l.CmdArgs, "-W") // We always wait until the submitted job terminates

	// Sanity checks
	if j == nil {
		return l, fmt.Errorf("job is undefined")
	}

	kvs, err := sy.LoadMPIConfigFile()
	if err != nil {
		return l, fmt.Errorf("unable to load configuration: %s", err)
	}

	err = generateJobScript(j, sysCfg, kvs)
	if err != nil {
		return l, fmt.Errorf("unable to generate Slurm script: %s", err)
	}
	l.CmdArgs = append(l.CmdArgs, j.BatchScript)

	return l, nil
}

func SlurmCleanUp(ctx context.Context, j Job) error {
	err := j.CleanUp()
	if err != nil {
		return fmt.Errorf("job cleanup failed: %s", err)
	}
	return nil
}
