// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package buildenv

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"

	"github.com/sylabs/singularity-mpi/internal/pkg/implem"
	util "github.com/sylabs/singularity-mpi/internal/pkg/util/file"
)

type Info struct {
	// SrcPath is the path to the downloaded tarball
	SrcPath string
	// SrcDir is the directory where the source code is
	SrcDir     string
	InstallDir string
	BuildDir   string
}

func (env *Info) Unpack() error {
	log.Println("- Unpacking MPI...")

	// Sanity checks
	if env.SrcPath == "" || env.BuildDir == "" {
		return fmt.Errorf("invalid parameter(s)")
	}

	// Figure out the extension of the tarball
	format := util.DetectTarballFormat(env.SrcPath)

	if format == "" {
		return fmt.Errorf("failed to detect format of file %s", env.SrcPath)
	}

	// At the moment we always assume we have to use the tar command
	// (and it is a fair assumption for our current context)
	tarPath, err := exec.LookPath("tar")
	if err != nil {
		return fmt.Errorf("tar is not available: %s", err)
	}

	tarArg := util.GetTarArgs(format)
	if tarArg == "" {
		return fmt.Errorf("unsupported format: %s", format)
	}

	// Untar the package
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(tarPath, tarArg, env.SrcPath)
	cmd.Dir = env.BuildDir
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("command failed: %s - stdout: %s - stderr: %s", err, stdout.String(), stderr.String())
	}

	// We do not need the package anymore, delete it
	err = os.Remove(env.SrcPath)
	if err != nil {
		return fmt.Errorf("failed to delete %s: %s", env.SrcPath, err)
	}

	// We save the directory created while untaring the tarball
	entries, err := ioutil.ReadDir(env.BuildDir)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %s", env.BuildDir, err)
	}
	if len(entries) != 1 {
		return fmt.Errorf("inconsistent temporary %s directory, %d files instead of 1", env.BuildDir, len(entries))
	}
	env.SrcDir = filepath.Join(env.BuildDir, entries[0].Name())

	return nil
}

func (env *Info) RunMake(stage string) error {
	// Some sanity checks
	if env.SrcDir == "" {
		return fmt.Errorf("invalid parameter(s)")
	}

	var stdout, stderr bytes.Buffer

	logMsg := "make -j4"
	makeCmd := exec.Command("make", "-j4")
	if stage == "install" {
		logMsg = "make -j4 install"
		makeCmd = exec.Command("make", "-j4", "install")
	}
	log.Printf("* Executing (from %s): %s", env.SrcDir, logMsg)

	makeCmd.Dir = env.SrcDir
	makeCmd.Stderr = &stderr
	makeCmd.Stdout = &stdout
	err := makeCmd.Run()
	if err != nil {
		return fmt.Errorf("command failed: %s - stdout: %s - stderr: %s", err, stdout.String(), stderr.String())
	}

	return nil
}

func (env *Info) copyTarball(mpiCfg *implem.Info) error {
	// Some sanity checks
	if mpiCfg.URL == "" {
		return fmt.Errorf("invalid parameter(s)")
	}

	// Figure out the name of the file if we do not already have it
	if mpiCfg.Tarball == "" {
		mpiCfg.Tarball = path.Base(mpiCfg.URL)
	}

	targetTarballPath := filepath.Join(env.BuildDir, mpiCfg.Tarball)
	// The begining of the URL starts with 'file://' which we do not want
	err := util.CopyFile(mpiCfg.URL[7:], targetTarballPath)
	if err != nil {
		return fmt.Errorf("cannot copy file %s to %s: %s", mpiCfg.URL, targetTarballPath, err)
	}

	env.SrcPath = filepath.Join(env.BuildDir, mpiCfg.Tarball)

	return nil
}

func (env *Info) Get(mpiCfg *implem.Info) error {
	log.Println("- Getting MPI...")

	// Sanity checks
	if mpiCfg.URL == "" {
		return fmt.Errorf("invalid parameter(s)")
	}

	// Detect the type of URL, e.g., file vs. http*
	urlFormat := util.DetectURLType(mpiCfg.URL)
	if urlFormat == "" {
		return fmt.Errorf("impossible to detect type from URL %s", mpiCfg.URL)
	}

	switch urlFormat {
	case util.FileURL:
		err := env.copyTarball(mpiCfg)
		if err != nil {
			return fmt.Errorf("impossible to copy the MPI tarball: %s", err)
		}
	case util.HttpURL:
		err := env.download(mpiCfg)
		if err != nil {
			return fmt.Errorf("impossible to download MPI: %s", err)
		}
	default:
		return fmt.Errorf("impossible to detect URL type: %s", mpiCfg.URL)
	}

	return nil
}

func (env *Info) download(mpiCfg *implem.Info) error {
	log.Println("- Downloading MPI...")

	// Sanity checks
	if mpiCfg.URL == "" || env.BuildDir == "" {
		return fmt.Errorf("invalid parameter(s)")
	}

	// TODO do not assume wget
	binPath, err := exec.LookPath("wget")
	if err != nil {
		return fmt.Errorf("cannot find wget: %s", err)
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.Command(binPath, mpiCfg.URL)
	cmd.Dir = env.BuildDir
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("command failed: %s - stdout: %s - stderr: %s", err, stdout.String(), stderr.String())
	}

	// FIXME we currently assume that we have one and only one file in the
	// directory This is not a fair assumption, especially while debugging
	// when we do not wipe out the temporary directories
	files, err := ioutil.ReadDir(env.BuildDir)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %s", env.BuildDir, err)
	}
	if len(files) != 1 {
		return fmt.Errorf("inconsistent temporary %s directory, %d files instead of 1", env.BuildDir, len(files))
	}
	mpiCfg.Tarball = files[0].Name()
	env.SrcPath = filepath.Join(env.BuildDir, files[0].Name())

	return nil
}
