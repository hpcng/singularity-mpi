// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package container

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/sylabs/singularity-mpi/internal/pkg/app"

	"github.com/sylabs/singularity-mpi/internal/pkg/checker"
	"github.com/sylabs/singularity-mpi/internal/pkg/implem"
	"github.com/sylabs/singularity-mpi/internal/pkg/sys"
	util "github.com/sylabs/singularity-mpi/internal/pkg/util/file"
	"github.com/sylabs/singularity-mpi/internal/pkg/util/sy"
)

const (
	// KeyPassphrase is the name of the environment variable used to specify the passphrase of the key to be used to sign images
	KeyPassphrase = "SY_KEY_PASSPHRASE"

	// KeyIndex is the index of the key to use to sign images
	KeyIndex = "SY_KEY_INDEX"

	// HybridModel is the identifier used to identify the hybrid model
	HybridModel = "hybrid"

	// BindModel is the identifier used to identify the bind-mount model
	BindModel = "bind"
)

// Config is a structure representing a container
type Config struct {
	// Name of the container
	Name string

	// Path to the container's image
	Path string

	// BuildDir is the path to the directory from where the image must be built
	BuildDir string

	// InstallDir is the directory where the container needs to be stored
	InstallDir string

	// DefFile is the path to the definition file associated to the container
	DefFile string

	// Distro is the ID of the Linux distribution to use in the container
	Distro string

	// URL is the URL of the container image to use when pulling the image from a registry
	URL string

	// Model specifies the model to follow for MPI inside the container
	Model string
}

// CreateContainer creates a container based on a MPI configuration
func Create(container *Config, sysCfg *sys.Config) error {
	var err error

	// Some sanity checks
	if container.BuildDir == "" {
		return fmt.Errorf("invalid parameter(s)")
	}

	if sysCfg.SingularityBin == "" {
		sysCfg.SingularityBin, err = exec.LookPath("singularity")
		if err != nil {
			return fmt.Errorf("singularity not available: %s", err)
		}
	}

	sudoBin, err := exec.LookPath("sudo")
	if err != nil {
		return fmt.Errorf("sudo not available: %s", err)
	}

	if container.Name == "" {
		container.Name = "singularity_mpi.sif"
	}

	if container.Path == "" {
		container.Path = filepath.Join(container.InstallDir, container.Name)
	}

	log.Printf("- Creating image %s...", container.Path)

	// We only let the mpirun command run for 10 minutes max
	ctx, cancel := context.WithTimeout(context.Background(), sys.CmdTimeout*2*time.Minute)
	defer cancel()

	// The definition file is ready so we simple build the container using the Singularity command
	if sysCfg.Debug {
		err = checker.CheckDefFile(container.DefFile)
		if err != nil {
			return fmt.Errorf("unable to check definition file: %s", err)
		}
	}

	log.Printf("-> Using definition file %s", container.DefFile)
	log.Printf("-> Running %s %s %s %s %s\n", sudoBin, sysCfg.SingularityBin, "build", container.Path, container.DefFile)
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, sudoBin, sysCfg.SingularityBin, "build", container.Path, container.DefFile)
	cmd.Dir = container.BuildDir
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to execute command - stdout: %s; stderr: %s; err: %s", stdout.String(), stderr.String(), err)
	}

	return nil
}

// PullContainerImage pulls from a registry the appropriate image
func PullContainerImage(cfg *Config, mpiImplm *implem.Info, sysCfg *sys.Config, syConfig *sy.MPIToolConfig) error {
	// Sanity checks
	if cfg.URL == "" {
		return fmt.Errorf("undefined image URL")
	}

	if sysCfg.SingularityBin == "" {
		var err error
		sysCfg.SingularityBin, err = exec.LookPath("singularity")
		if err != nil {
			return fmt.Errorf("failed to find Singularity binary: %s", err)
		}
	}

	log.Println("* Pulling container with the following MPI configuration *")
	log.Println("-> Build container in", cfg.BuildDir)
	log.Println("-> MPI implementation:", mpiImplm.ID)
	log.Println("-> MPI version:", mpiImplm.Version)
	log.Println("-> Image URL:", cfg.URL)

	err := Pull(cfg, sysCfg)
	if err != nil {
		return fmt.Errorf("failed to pull image: %s", err)
	}

	return nil
}

// Pull retieves an image from the registry
func Pull(containerInfo *Config, sysCfg *sys.Config) error {
	var stdout, stderr bytes.Buffer

	log.Printf("* Singularity binary: %s\n", sysCfg.SingularityBin)
	log.Printf("* Container path: %s\n", containerInfo.Path)
	log.Printf("* Image URL: %s\n", containerInfo.URL)
	log.Printf("* Build directory: %s\n", containerInfo.BuildDir)
	log.Printf("-> Pulling image: %s pull %s %s", sysCfg.SingularityBin, containerInfo.Path, containerInfo.URL)

	if sysCfg.SingularityBin == "" || containerInfo.Path == "" || containerInfo.URL == "" || containerInfo.BuildDir == "" {
		return fmt.Errorf("invalid parameter(s)")
	}

	if sysCfg.Persistent != "" && util.PathExists(containerInfo.Path) {
		log.Printf("* Persistent mode, %s already available, skipping...", containerInfo.Path)
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), sys.CmdTimeout*2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, sysCfg.SingularityBin, "pull", containerInfo.Path, containerInfo.URL)
	cmd.Dir = containerInfo.BuildDir
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to execute command - stdout: %s; stderr: %s; err: %s", stdout.String(), stderr.String(), err)
	}

	return nil
}

// Sign signs a given image
func Sign(container *Config, sysCfg *sys.Config) error {
	var stdout, stderr bytes.Buffer

	log.Printf("-> Signing container (%s)", container.Path)
	ctx, cancel := context.WithTimeout(context.Background(), sys.CmdTimeout*2*time.Minute)
	defer cancel()

	indexIdx := "0"
	if os.Getenv(KeyIndex) != "" {
		indexIdx = os.Getenv(KeyIndex)
	}

	cmd := exec.CommandContext(ctx, sysCfg.SingularityBin, "sign", "--keyidx", indexIdx, container.Path)

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
	cmd.Dir = container.BuildDir
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to execute command - stdout: %s; stderr: %s; err: %s", stdout.String(), stderr.String(), err)
	}

	return nil
}

// Upload uploads an image to a registry
func Upload(containerInfo *Config, sysCfg *sys.Config) error {
	var stdout, stderr bytes.Buffer

	log.Printf("-> Uploading container %s to %s", containerInfo.Path, sysCfg.Registry)
	ctx, cancel := context.WithTimeout(context.Background(), sys.CmdTimeout*2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, sysCfg.SingularityBin, "push", containerInfo.Path, sysCfg.Registry)
	cmd.Dir = containerInfo.BuildDir
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to execute command - stdout: %s; stderr: %s; err: %s", stdout.String(), stderr.String(), err)
	}

	return nil
}

func GetContainerInstallDir(appInfo *app.Info) string {
	return "mpi_container_" + appInfo.Name
}
