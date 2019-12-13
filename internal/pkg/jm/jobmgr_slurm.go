// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package jm

import (
	"fmt"
	"io/ioutil"
	"log"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sylabs/singularity-mpi/internal/pkg/buildenv"
	"github.com/sylabs/singularity-mpi/internal/pkg/job"
	"github.com/sylabs/singularity-mpi/internal/pkg/kv"
	"github.com/sylabs/singularity-mpi/internal/pkg/mpi"
	"github.com/sylabs/singularity-mpi/internal/pkg/slurm"
	"github.com/sylabs/singularity-mpi/internal/pkg/sy"
	"github.com/sylabs/singularity-mpi/internal/pkg/syexec"
	"github.com/sylabs/singularity-mpi/internal/pkg/sympierr"
	"github.com/sylabs/singularity-mpi/internal/pkg/sys"
)

// LoadSlurm is the function used by our job management framework to figure out if Slurm can be used and
// if so return a JM structure with all the "function pointers" to interact with Slurm through our generic
// API.
func SlurmDetect() (bool, JM) {
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
	jm.Load = SlurmLoad

	return true, jm
}

// SlurmGetOutput reads the content of the Slurm output file that is associated to a job
func SlurmGetOutput(j *job.Job, sysCfg *sys.Config) string {
	outputFile := getJobOutputFilePath(j, sysCfg)
	output, err := ioutil.ReadFile(outputFile)
	if err != nil {
		return ""
	}

	return string(output)
}

// SlurmGetError reads the content of the Slurm error file that is associated to a job
func SlurmGetError(j *job.Job, sysCfg *sys.Config) string {
	errorFile := getJobErrorFilePath(j, sysCfg)
	errorTxt, err := ioutil.ReadFile(errorFile)
	if err != nil {
		return ""
	}

	return string(errorTxt)
}

// SlurmGetConfig is the Slurm function to get the configuration of the job manager
func SlurmGetConfig() error {
	return nil
}

// SlurmSetConfig is the Slurm function to set the configuration of the job manager
func SlurmSetConfig() error {
	configFile := sy.GetPathToSyMPIConfigFile()

	err := sy.ConfigFileUpdateEntry(configFile, slurm.EnabledKey, "true")
	if err != nil {
		return fmt.Errorf("failed to update entry %s in %s: %s", slurm.EnabledKey, configFile, err)
	}
	return nil
}

// SlurmLoad is the function called when trying to load a JM module
func SlurmLoad(jm *JM, sysCfg *sys.Config) error {
	log.Println("* Slurm detected, updating the configuration file")
	kvs, err := kv.LoadKeyValueConfig(sysCfg.SyConfigFile)
	if err != nil {
		return fmt.Errorf("unable to load configuration from %s: %s", sysCfg.SyConfigFile, err)
	}
	if kv.GetValue(kvs, slurm.EnabledKey) == "" {
		err := SlurmSetConfig()
		if err != nil {
			return fmt.Errorf("unable to add Slurm entry in configuration file: %s", err)
		}
	}

	return nil
}

func getJobOutFilenamePrefix(j *job.Job) string {
	return "host-" + j.HostCfg.ID + "-" + j.HostCfg.Version + "_container-" + j.Container.Name
}

func getJobOutputFilePath(j *job.Job, sysCfg *sys.Config) string {
	errorFilename := getJobOutFilenamePrefix(j) + ".out"
	path := filepath.Join(sysCfg.ScratchDir, errorFilename)
	if sysCfg.Persistent != "" {
		path = filepath.Join(j.Container.InstallDir, errorFilename)
	}
	return path
}

func getJobErrorFilePath(j *job.Job, sysCfg *sys.Config) string {
	outputFilename := getJobOutFilenamePrefix(j) + ".err"
	path := filepath.Join(sysCfg.ScratchDir, outputFilename)
	if sysCfg.Persistent != "" {
		path = filepath.Join(j.Container.InstallDir, outputFilename)
	}
	return path
}

func generateJobScript(j *job.Job, env *buildenv.Info, sysCfg *sys.Config, kvs []kv.KV) error {
	// Sanity checks
	if j == nil {
		return fmt.Errorf("undefined job")
	}

	// Some sanity checks
	if j.HostCfg == nil {
		return fmt.Errorf("undefined host configuration")
	}

	if env.InstallDir == "" {
		return fmt.Errorf("undefined host installation directory")
	}

	if sysCfg.ScratchDir == "" {
		return fmt.Errorf("undefined scratch directory")
	}

	if j.App.BinPath == "" {
		return fmt.Errorf("application binary is undefined")
	}

	// Create the batch script
	err := TempFile(j, env, sysCfg)
	if err != nil {
		if err == sympierr.ErrFileExists {
			log.Printf("* Script %s already esists, skipping\n", j.BatchScript)
			return nil
		}
		return fmt.Errorf("unable to create temporary file: %s", err)
	}

	// TempFile is supposed to set the path to the batch script
	if j.BatchScript == "" {
		return fmt.Errorf("Batch script path is undefined")
	}

	scriptText := "#!/bin/bash\n#\n"
	partition := kv.GetValue(kvs, slurm.PartitionKey)
	if partition != "" {
		scriptText += slurm.ScriptCmdPrefix + " --partition=" + partition + "\n"
	}

	if j.NNodes > 0 {
		scriptText += slurm.ScriptCmdPrefix + " --nodes=" + strconv.Itoa(j.NNodes) + "\n"
	}

	if j.NP > 0 {
		scriptText += slurm.ScriptCmdPrefix + " --ntasks=" + strconv.Itoa(j.NP) + "\n"
	}

	scriptText += slurm.ScriptCmdPrefix + " --error=" + getJobErrorFilePath(j, sysCfg) + "\n"
	scriptText += slurm.ScriptCmdPrefix + " --output=" + getJobOutputFilePath(j, sysCfg) + "\n"

	// Set PATH and LD_LIBRARY_PATH
	scriptText += "\nexport PATH=" + env.InstallDir + "/bin:$PATH\n"
	scriptText += "export LD_LIBRARY_PATH=" + env.InstallDir + "/lib:$LD_LIBRARY_PATH\n\n"

	// Add the mpirun command
	mpirunPath := filepath.Join(env.InstallDir, "bin", "mpirun")
	mpirunArgs, err := mpi.GetMpirunArgs(j.HostCfg, env, &j.App, j.Container, sysCfg)
	if err != nil {
		return fmt.Errorf("unable to get mpirun arguments: %s", err)
	}
	scriptText += "\n" + mpirunPath + " " + strings.Join(mpirunArgs, " ") + "\n"

	err = ioutil.WriteFile(j.BatchScript, []byte(scriptText), 0644)
	if err != nil {
		return fmt.Errorf("unable to write to file %s: %s", j.BatchScript, err)
	}

	return nil
}

// SlurmSubmit prepares the batch script necessary to start a given job.
//
// Note that a script does not need any specific environment to be submitted
func SlurmSubmit(j *job.Job, hostBuildEnv *buildenv.Info, sysCfg *sys.Config) (syexec.SyCmd, error) {
	var sycmd syexec.SyCmd
	sycmd.BinPath = "sbatch"
	sycmd.CmdArgs = append(sycmd.CmdArgs, "-W") // We always wait until the submitted job terminates

	// Sanity checks
	if j == nil {
		return sycmd, fmt.Errorf("job is undefined")
	}

	kvs, err := sy.LoadMPIConfigFile()
	if err != nil {
		return sycmd, fmt.Errorf("unable to load configuration: %s", err)
	}

	err = generateJobScript(j, hostBuildEnv, sysCfg, kvs)
	if err != nil {
		return sycmd, fmt.Errorf("unable to generate Slurm script: %s", err)
	}
	sycmd.CmdArgs = append(sycmd.CmdArgs, j.BatchScript)

	j.GetOutput = SlurmGetOutput
	j.GetError = SlurmGetError

	return sycmd, nil
}
