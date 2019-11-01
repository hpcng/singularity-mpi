// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

/*
 * buildenv is a package that provides all the capabilities to deal with a build environment,
 * from defining where the software should be compiled and install, to the actual configuration,
 * compilation and installation of software.
 */
package buildenv

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/sylabs/singularity-mpi/internal/pkg/implem"

	"github.com/sylabs/singularity-mpi/internal/pkg/persistent"
	"github.com/sylabs/singularity-mpi/internal/pkg/sys"
	util "github.com/sylabs/singularity-mpi/internal/pkg/util/file"
)

// SoftwarePackage gathers all the information related to the software package to prepare in the build environment
type SoftwarePackage struct {
	// Name is the name with which the software package is recognized
	Name string

	// URL is the source of the software
	URL string

	// InstallCmd is the command used to install the software
	InstallCmd string

	tarball string
}

// Info gathers the details of the build environment
type Info struct {
	// SrcPath is the path to the downloaded tarball
	SrcPath string

	// SrcDir is the directory where the source code is
	SrcDir string

	// ScratchDir is the directory where we can store temporary data
	ScratchDir string

	// InstallDir is the directory where the software needs to be installed
	InstallDir string

	// BuildDir is the directory where the software is built
	BuildDir string

	// Env is the environment to use with the build environment
	Env []string
}

// Unpack extracts the source code from a package/tarball/zip file.
func (env *Info) Unpack() error {
	log.Println("- Unpacking software...")

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
	log.Printf("-> Executing from %s: %s %s %s \n", env.BuildDir, tarPath, tarArg, env.SrcPath)
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

// RunMake executes the appropriate command to build the software
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

func (env *Info) copyTarball(p *SoftwarePackage) error {
	// Some sanity checks
	if p.URL == "" {
		return fmt.Errorf("invalid parameter(s)")
	}

	// Figure out the name of the file if we do not already have it
	if p.tarball == "" {
		p.tarball = path.Base(p.URL)
	}

	targetTarballPath := filepath.Join(env.BuildDir, p.tarball)
	// The begining of the URL starts with 'file://' which we do not want
	err := util.CopyFile(p.URL[7:], targetTarballPath)
	if err != nil {
		return fmt.Errorf("cannot copy file %s to %s: %s", p.URL, targetTarballPath, err)
	}

	env.SrcPath = filepath.Join(env.BuildDir, p.tarball)

	return nil
}

// Get is the function to get a given source code
func (env *Info) Get(p *SoftwarePackage) error {
	log.Printf("- Getting %s from %s...\n", p.Name, p.URL)

	// Sanity checks
	if p.URL == "" {
		return fmt.Errorf("invalid parameter(s)")
	}

	// Detect the type of URL, e.g., file vs. http*
	urlFormat := util.DetectURLType(p.URL)
	if urlFormat == "" {
		return fmt.Errorf("impossible to detect type from URL %s", p.URL)
	}

	switch urlFormat {
	case util.FileURL:
		err := env.copyTarball(p)
		if err != nil {
			return fmt.Errorf("impossible to copy the tarball: %s", err)
		}
	case util.HttpURL:
		err := env.download(p)
		if err != nil {
			return fmt.Errorf("impossible to download %s: %s", p.Name, err)
		}
	default:
		return fmt.Errorf("impossible to detect URL type: %s", p.URL)
	}

	return nil
}

func (env *Info) download(p *SoftwarePackage) error {
	// Sanity checks
	if p.URL == "" || env.BuildDir == "" {
		return fmt.Errorf("invalid parameter(s)")
	}

	log.Printf("- Downloading %s from %s...", p.Name, p.URL)

	// todo: do not assume wget
	binPath, err := exec.LookPath("wget")
	if err != nil {
		return fmt.Errorf("cannot find wget: %s", err)
	}

	log.Printf("* Executing from %s: %s %s", env.BuildDir, binPath, p.URL)
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(binPath, p.URL)
	cmd.Dir = env.BuildDir
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("command failed: %s - stdout: %s - stderr: %s", err, stdout.String(), stderr.String())
	}

	// todo: we currently assume that we have one and only one file in the
	// directory This is not a fair assumption, especially while debugging
	// when we do not wipe out the temporary directories
	files, err := ioutil.ReadDir(env.BuildDir)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %s", env.BuildDir, err)
	}
	if len(files) != 1 {
		return fmt.Errorf("inconsistent temporary %s directory, %d files instead of 1", env.BuildDir, len(files))
	}
	p.tarball = files[0].Name()
	env.SrcPath = filepath.Join(env.BuildDir, files[0].Name())

	return nil
}

// IsInstalled checks whether a specific software package is already installed in a specific build environment
func (env *Info) IsInstalled(p *SoftwarePackage) bool {
	switch util.DetectURLType(p.URL) {
	case util.FileURL:
		filename := path.Base(p.URL)
		filePathInBuildDir := filepath.Join(env.BuildDir, filename)
		filePathInInstallDir := filepath.Join(env.InstallDir, filename)
		return util.FileExists(filePathInBuildDir) || util.FileExists(filePathInInstallDir)
	case util.HttpURL:
		// todo: do not assume that a package downloaded from the web is always a tarball
		filename := path.Base(p.URL)
		filePath := filepath.Join(env.BuildDir, filename)
		log.Printf("* Checking whether %s exists...\n", filePath)
		return util.FileExists(filePath)
	case util.GitURL:
		dirname := path.Base(p.URL)
		dirname = strings.Replace(dirname, ".git", "", -1)
		path := filepath.Join(env.BuildDir, dirname)
		return util.PathExists(path)
	}

	return false
}

// GetEnvPath returns the string representing the value for the PATH environment
// variable to use
func (env *Info) GetEnvPath() string {
	return filepath.Join(env.InstallDir, "bin") + ":" + os.Getenv("PATH")
}

// GetEnvLDPath returns the string representing the value for the LD_LIBRARY_PATH
// environment variable to use
func (env *Info) GetEnvLDPath() string {
	return filepath.Join(env.InstallDir, "lib") + ":" + os.Getenv("LD_LIBRARY_PATH")
}

// Install is a generic function to install a software
func (env *Info) Install(p *SoftwarePackage) error {
	ctx, cancel := context.WithTimeout(context.Background(), sys.CmdTimeout*time.Second)
	defer cancel()

	cmdElts := strings.Split(p.InstallCmd, " ")
	binPath, err := exec.LookPath(cmdElts[0])
	if err != nil {
		// If we cannot find the executable, it is specific to the package
		binPath = cmdElts[0]
	}
	log.Printf("Executing from %s: %s %s.", env.SrcDir, binPath, strings.Join(cmdElts[1:], " "))
	cmd := exec.CommandContext(ctx, binPath, cmdElts[1:]...)
	cmd.Dir = env.SrcDir
	cmd.Env = env.Env
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to install %s: %s; stdout: %s; stderr: %s", p.Name, err, stdout.String(), stderr.String())
	}

	return nil
}

func CreateDefaultHostEnvCfg(env *Info, mpi *implem.Info, sysCfg *sys.Config) error {
	/* SET THE BUILD DIRECTORY */

	// The build directory is always in the scratch
	env.BuildDir = filepath.Join(sysCfg.ScratchDir, "mpi_build_"+mpi.ID+"_"+mpi.Version)
	// We always initialize the build directory for MPI on the host
	err := util.DirInit(env.BuildDir)
	if err != nil {
		return fmt.Errorf("failed to initialize directory %s: %s", env.BuildDir, err)
	}

	/* SET THE INSTALL DIRECTORY */

	if sysCfg.Persistent == "" {
		// Create a temporary directory where to install MPI
		env.InstallDir = filepath.Join(sysCfg.ScratchDir, "mpi_install_"+mpi.ID+"-"+mpi.Version)
		err := util.DirInit(env.InstallDir)
		if err != nil {
			return fmt.Errorf("failed to initialize directory %s: %s", env.InstallDir, err)
		}
	} else {
		env.InstallDir = persistent.GetPersistentHostMPIInstallDir(mpi, sysCfg)
	}

	/* SET THE SCRATCH DIRECTORY */

	env.ScratchDir = filepath.Join(sysCfg.ScratchDir, "scratch_"+mpi.ID+"_"+mpi.Version)
	// We always initialize the scratch directory for MPI on the host
	err = util.DirInit(env.ScratchDir)
	if err != nil {
		return fmt.Errorf("failed to initialize directory %s: %s", env.ScratchDir, err)
	}

	return nil
}

func GetDefaultScratchDir(mpi *implem.Info) string {
	return filepath.Join(sys.GetSympiDir(), "scratch-"+mpi.ID)
}
