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
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/sylabs/singularity/pkg/syfs"

	"github.com/sylabs/singularity-mpi/internal/pkg/checker"
	"github.com/sylabs/singularity-mpi/internal/pkg/mpi"
	"github.com/sylabs/singularity-mpi/internal/pkg/sys"
	util "github.com/sylabs/singularity-mpi/internal/pkg/util/file"
)

type MPIToolConfig struct {
	// BuildPrivilege specifies whether or not we can build images on the platform
	BuildPrivilege bool
}

const (
	// BuildPrivilegeKey is the key used in the tool's configuration file to specify if the tool can create images on the platform
	BuildPrivilegeKey = "build_privilege"
)

const (
	// KeyPassphrase is the name of the environment variable used to specify the passphrase of the key to be used to sign images
	KeyPassphrase = "SY_KEY_PASSPHRASE"

	// KeyIndex is the index of the key to use to sign images
	KeyIndex = "SY_KEY_INDEX"
)

func Pull(mpiCfg *mpi.Config, sysCfg *sys.Config) error {
	var stdout, stderr bytes.Buffer

	if sysCfg.SingularityBin == "" || mpiCfg.ContainerPath == "" || mpiCfg.ImageURL == "" || mpiCfg.BuildDir == "" {
		return fmt.Errorf("invalid parameter(s)")
	}

	log.Printf("-> Pulling image: %s pull %s %s", sysCfg.SingularityBin, mpiCfg.ContainerPath, mpiCfg.ImageURL)

	ctx, cancel := context.WithTimeout(context.Background(), sys.CmdTimeout*2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, sysCfg.SingularityBin, "pull", mpiCfg.ContainerPath, mpiCfg.ImageURL)
	cmd.Dir = mpiCfg.BuildDir
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to execute command - stdout: %s; stderr: %s; err: %s", stdout.String(), stderr.String(), err)
	}

	return nil
}

func Sign(mpiCfg *mpi.Config, sysCfg *sys.Config) error {
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

func Upload(mpiCfg *mpi.Config, sysCfg *sys.Config) error {
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

func getPathToSyMPIConfigFile() string {
	return filepath.Join(syfs.ConfigDir(), "singularity-mpi.conf")
}

func initMPIConfigFile(path string) error {
	buildPrivilegeEntry := BuildPrivilegeKey + " = true"
	err := checker.CheckBuildPrivilege()
	if err != nil {
		log.Printf("* [INFO] Cannot build singularity images: %s", err)
		buildPrivilegeEntry = BuildPrivilegeKey + " = false"
	}

	data := []byte(buildPrivilegeEntry + "\n")

	err = ioutil.WriteFile(path, data, 0644)
	if err != nil {
		return fmt.Errorf("Impossible to create configuration file %s :%s", path, err)
	}

	return nil
}

// CreateMPIConfigFile ensures that the configuration file of the tool is correctly created
func CreateMPIConfigFile() (string, error) {
	syDir := syfs.ConfigDir()
	if !util.PathExists(syDir) {
		return "", fmt.Errorf("%s does not exist. Is Singularity installed?", syDir)
	}

	syMPIConfigFile := getPathToSyMPIConfigFile()
	log.Printf("-> Creating MPI configuration file: %s", syMPIConfigFile)
	if !util.PathExists(syMPIConfigFile) {
		err := initMPIConfigFile(syMPIConfigFile)
		if err != nil {
			return "", fmt.Errorf("failed to initialize MPI configuration file: %s", err)
		}
	}

	return syMPIConfigFile, nil
}
