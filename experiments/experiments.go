// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package experiments

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
)

type mpiConfig struct {
	URL             string // URL to use to download the tarball
	srcPath         string // Path to the downloaded tarball
	srcDir          string // Where the source has been untared
	buildDir        string // Directory where to compile
	installDir      string // Directory where to install the compiled software
	m4SrcPath       string // Path to the m4 tarball
	autoconfSrcPath string // Path to the autoconf tarball
	automakeSrcPath string // Path to the automake tarball
	libtoolsSrcPath string // Path to the libtools tarball
}

// Experiment is a structure that represents the configuration of an experiment
type Experiment struct {
	VersionHostMPI      string
	URLHostMPI          string
	VersionContainerMPI string
	URLContainerMPI     string
}

func downloadMPI(mpiCfg mpiConfig) error {
	// Sanity checks
	if mpiCfg.URL == "" || mpiCfg.buildDir == "" {
		return fmt.Errorf("imvalid parameter(s)")
	}

	// TODO do not assume wget
	binPath, err := exec.LookPath("wget")
	if err != nil {
		return fmt.Errorf("cannot find wget: %s", err)
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.Command(binPath, mpiCfg.URL)
	cmd.Dir = mpiCfg.buildDir
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("command failed: %s - stdout: %s - stderr: %s", err, stdout.String(), stderr.String())
	}

	// FIXME we currently assume that we one and only file in the directory
	// This is not a fair assumption, especially while debugging when we do
	// not wipe out the temporary directories
	files, err := ioutil.ReadDir(mpiCfg.buildDir)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %s", mpiCfg.buildDir, err)
	}
	if len(files) != 1 {
		return fmt.Errorf("inconsistent temporary %s directory, %d files instead of 1", mpiCfg.buildDir, len(files))
	}
	mpiCfg.srcPath = files[0].Name()

	return nil
}

const (
	formatBZ2 = "bz2"
)

func detectTarbalFormat(filepath string) string {
	if path.Ext(filepath) == "bz2" {
		return formatBZ2
	}

	return ""
}

func unpackMPI(mpiCfg mpiConfig) error {
	// Sanity checks
	if mpiCfg.srcPath == "" || mpiCfg.buildDir == "" {
		return fmt.Errorf("imvalid parameter(s)")
	}

	// Figure out the extension of the tarball
	format := detectTarbalFormat(mpiCfg.srcPath)
	if format == "" {
		return fmt.Errorf("failed to detect format of file %s", mpiCfg.srcPath)
	}

	// At the moment we always assume we have to use the tar command
	// (and it is a fair assumption for our current context)
	tarPath, err := exec.LookPath("tar")
	if err != nil {
		return fmt.Errorf("tar is not available: %s", err)
	}

	// Figure out the tar argument based on the format
	tarArg := ""
	switch format {
	case formatBZ2:
		tarArg = "-xjf"
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}

	// Untar the package
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(tarPath, tarArg, mpiCfg.srcPath)
	cmd.Dir = mpiCfg.buildDir
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("command failed: %s - stdout: %s - stderr: %s", err, stdout.String(), stderr.String())
	}

	// We do not need the package anymore, delete it
	err = os.Remove(mpiCfg.srcPath)
	if err != nil {
		return fmt.Errorf("failed to delete %s: %s", mpiCfg.srcPath, err)
	}

	// We save the directory created while untaring the tarball
	entries, err := ioutil.ReadDir(mpiCfg.buildDir)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %s", mpiCfg.buildDir, err)
	}
	if len(entries) != 1 {
		return fmt.Errorf("inconsistent temporary %s directory, %d files instead of 1", mpiCfg.buildDir, len(entries))
	}
	mpiCfg.srcDir = entries[0].Name()

	return nil
}

func configureMPI(mpiCfg mpiConfig) error {
	if mpiCfg.srcDir == "" || mpiCfg.installDir == "" {
		return fmt.Errorf("imvalid parameter(s)")
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.Command("configure", "--prefix="+mpiCfg.installDir)
	cmd.Dir = mpiCfg.srcDir
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("command failed: %s - stdout: %s - stderr: %s", err, stdout.String(), stderr.String())
	}

	return nil
}

func compileMPI(mpiCfg mpiConfig) error {
	if mpiCfg.srcDir == "" {
		return fmt.Errorf("imvalid parameter(s)")
	}

	// Because some packages are not as well implemented as they should,
	// we first run 'make -j4' and then 'make install'
	var stdout, stderr bytes.Buffer

	cmd := exec.Command("make", "-j", "4")
	cmd.Dir = mpiCfg.srcDir
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("command failed: %s - stdout: %s - stderr: %s", err, stdout.String(), stderr.String())
	}

	cmd = exec.Command("make", "install")
	cmd.Dir = mpiCfg.srcDir
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("command failed: %s - stdout: %s - stderr: %s", err, stdout.String(), stderr.String())
	}

	return nil
}

// Run configure, install and execute a given experiment
func Run(exp Experiment) (bool, error) {
	var myCfg mpiConfig
	var err error

	// Create a temporary directory where to compile MPI
	myCfg.buildDir, err = ioutil.TempDir("", "mpi_build_"+exp.VersionHostMPI)
	if err != nil {
		return false, fmt.Errorf("failed to create compile directory: %s", err)
	}
	defer os.RemoveAll(myCfg.buildDir)

	// Create a temporary directory where to install MPI
	myCfg.installDir, err = ioutil.TempDir("", "mpi_install_"+exp.VersionHostMPI)
	if err != nil {
		return false, fmt.Errorf("failed to create install directory: %s", err)
	}
	defer os.RemoveAll(myCfg.installDir)

	myCfg.URL = exp.URLHostMPI

	err = installHostMPI(myCfg)
	if err != nil {
		return false, fmt.Errorf("failed to install host MPI")
	}

	return true, nil
}

func installHostMPI(myCfg mpiConfig) error {
	err := downloadMPI(myCfg)
	if err != nil {
		return fmt.Errorf("failed to download MPI from %s: %s", myCfg.URL, err)
	}

	err = unpackMPI(myCfg)
	if err != nil {
		return fmt.Errorf("failed to unpack MPI: %s", err)
	}

	// Right now, we assume we do not have to install autotools, which is a bad assumption
	err = configureMPI(myCfg)
	if err != nil {
		return fmt.Errorf("failed to configure MPI")
	}

	err = compileMPI(myCfg)
	if err != nil {
		return fmt.Errorf("failed to compile MPI")
	}

	return nil
}
