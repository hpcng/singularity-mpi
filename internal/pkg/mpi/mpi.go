// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package mpi

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

	"github.com/sylabs/singularity-mpi/internal/pkg/checker"
	"github.com/sylabs/singularity-mpi/internal/pkg/deffile"
	"github.com/sylabs/singularity-mpi/internal/pkg/sys"
	util "github.com/sylabs/singularity-mpi/internal/pkg/util/file"
)

// Config represents a configuration of MPI for a target platform
type Config struct {
	// MpiImplm is the MPI implementation ID (e.g., OMPI, MPICH)
	MpiImplm string
	// MpiVersion is the Version of the MPI implementation to use
	MpiVersion string
	// URL to use to download the tarball
	URL string
	// tarball is the file name of the source code file for the target MPI implementation
	tarball string
	// srcPath is the path to the downloaded tarball
	srcPath string
	// srcDir is the path to the directory where the source has been untared
	srcDir string
	// BuildDir is the path to the directory where to compile
	BuildDir string
	// InstallDirectory is the path to the directory where to install the compiled software
	InstallDir string
	// Deffile is the path to the definition file used to create MPI container
	DefFile string
	// ContainerName is the name of the container's image file
	ContainerName string
	// ContainerPath is the Path to the container image
	ContainerPath string
	// TestPath is the path to the test to run within the container
	TestPath string
	// Distro is the ID of the Linux distro to use in the container
	Distro string
	// ImageURL is the URL to use to pull an image
	ImageURL string

	//	m4SrcPath       string // Path to the m4 tarball
	//	autoconfSrcPath string // Path to the autoconf tarball
	//	automakeSrcPath string // Path to the automake tarball
	//	libtoolsSrcPath string // Path to the libtools tarball
}

type compileConfig struct {
	mpiVersionTag string
	mpiURLTag     string
	mpiTarballTag string
	//	mpiTarArgsTag string
}

// Experiment is a structure that represents the configuration of an experiment
type Experiment struct {
	// MPIImnplm is the string identifying the MPI implementation, e.g., openmpi or mpich
	MPIImplm string
	// VersionHostMPI is the version of the MPI implementation to use on the host
	VersionHostMPI string
	// URLHostMPI is the URL to use for downloading MPI that is to be installed on the host
	URLHostMPI string
	// VersionContainerMPI is the version of the MPI implementation to use in the container
	VersionContainerMPI string
	// URLContainerMPI is the URL to use for downloading MPI that is to be installed in the container
	URLContainerMPI string
}

// ExecResult represents the result of the execution of a command
type ExecResult struct {
	// Err is the Go error associated to the command execution
	Err error
	// Stdout is the messages that were displayed on stdout during the execution of the command
	Stdout string
	// Stderr is the messages that were displayed on stderr during the execution of the command
	Stderr string
}

func copyTarball(mpiCfg *Config) error {
	// Some sanity checks
	if mpiCfg.URL == "" {
		return fmt.Errorf("invalid parameter(s)")
	}

	// Figure out the name of the file if we do not already have it
	if mpiCfg.tarball == "" {
		mpiCfg.tarball = path.Base(mpiCfg.URL)
	}

	targetTarballPath := filepath.Join(mpiCfg.BuildDir, mpiCfg.tarball)
	// The begining of the URL starts with 'file://' which we do not want
	err := util.CopyFile(mpiCfg.URL[7:], targetTarballPath)
	if err != nil {
		return fmt.Errorf("cannot copy file %s to %s: %s", mpiCfg.URL, targetTarballPath, err)
	}

	mpiCfg.srcPath = filepath.Join(mpiCfg.BuildDir, mpiCfg.tarball)

	return nil
}

func download(mpiCfg *Config) error {
	log.Println("- Downloading MPI...")

	// Sanity checks
	if mpiCfg.URL == "" || mpiCfg.BuildDir == "" {
		return fmt.Errorf("invalid parameter(s)")
	}

	// TODO do not assume wget
	binPath, err := exec.LookPath("wget")
	if err != nil {
		return fmt.Errorf("cannot find wget: %s", err)
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.Command(binPath, mpiCfg.URL)
	cmd.Dir = mpiCfg.BuildDir
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("command failed: %s - stdout: %s - stderr: %s", err, stdout.String(), stderr.String())
	}

	// FIXME we currently assume that we have one and only one file in the
	// directory This is not a fair assumption, especially while debugging
	// when we do not wipe out the temporary directories
	files, err := ioutil.ReadDir(mpiCfg.BuildDir)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %s", mpiCfg.BuildDir, err)
	}
	if len(files) != 1 {
		return fmt.Errorf("inconsistent temporary %s directory, %d files instead of 1", mpiCfg.BuildDir, len(files))
	}
	mpiCfg.tarball = files[0].Name()
	mpiCfg.srcPath = filepath.Join(mpiCfg.BuildDir, files[0].Name())

	return nil
}

func get(mpiCfg *Config) error {
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
		err := copyTarball(mpiCfg)
		if err != nil {
			return fmt.Errorf("impossible to copy the MPI tarball: %s", err)
		}
	case util.HttpURL:
		err := download(mpiCfg)
		if err != nil {
			return fmt.Errorf("impossible to download MPI: %s", err)
		}
	default:
		return fmt.Errorf("impossible to detect URL type: %s", mpiCfg.URL)
	}

	return nil
}

func unpack(mpiCfg *Config) error {
	log.Println("- Unpacking MPI...")

	// Sanity checks
	if mpiCfg.srcPath == "" || mpiCfg.BuildDir == "" {
		return fmt.Errorf("invalid parameter(s)")
	}

	// Figure out the extension of the tarball
	format := util.DetectTarballFormat(mpiCfg.srcPath)

	if format == "" {
		return fmt.Errorf("failed to detect format of file %s", mpiCfg.srcPath)
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
	cmd := exec.Command(tarPath, tarArg, mpiCfg.srcPath)
	cmd.Dir = mpiCfg.BuildDir
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
	entries, err := ioutil.ReadDir(mpiCfg.BuildDir)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %s", mpiCfg.BuildDir, err)
	}
	if len(entries) != 1 {
		return fmt.Errorf("inconsistent temporary %s directory, %d files instead of 1", mpiCfg.BuildDir, len(entries))
	}
	mpiCfg.srcDir = filepath.Join(mpiCfg.BuildDir, entries[0].Name())

	return nil
}

func compile(mpiCfg *Config, sysCfg *sys.Config) ExecResult {
	var res ExecResult

	log.Println("- Compiling MPI...")
	if mpiCfg.srcDir == "" {
		res.Err = fmt.Errorf("invalid parameter(s)")
		return res
	}

	makefilePath := filepath.Join(mpiCfg.srcDir, "Makefile")
	if util.FileExists(makefilePath) {
		res.Err = runMake(mpiCfg, "")
		return res
	}

	fmt.Println("-> No Makefile, trying to figure out how to compile/install MPI...")
	if mpiCfg.MpiImplm == "intel" {
		res.Err = SetupIntelInstallScript(mpiCfg, sysCfg)
		if res.Err != nil {
			return res
		}
		return runIntelScript(mpiCfg, sysCfg, "install")
	}

	return res
}

func install(mpiCfg *Config, sysCfg *sys.Config) ExecResult {
	var res ExecResult

	if mpiCfg.MpiImplm == "intel" {
		fmt.Println("-> Intel MPI detected, no install step, compile step installed the software...")
	}

	log.Printf("- Installing MPI in %s...", mpiCfg.InstallDir)
	if mpiCfg.InstallDir == "" || mpiCfg.BuildDir == "" {
		res.Err = fmt.Errorf("invalid parameter(s)")
		return res
	}

	makefilePath := filepath.Join(mpiCfg.srcDir, "Makefile")
	if util.FileExists(makefilePath) {
		res.Err = runMake(mpiCfg, "install")
		return res
	}

	return res
}

// GetPathToMpirun returns the path to mpirun based a configuration of MPI
func GetPathToMpirun(mpiCfg *Config) string {
	// Intel MPI is installing the binaries and libraries in a quite complex setup
	if mpiCfg.MpiImplm == "intel" {
		return filepath.Join(mpiCfg.BuildDir, IntelInstallPathPrefix, "bin/mpiexec")
	}

	return filepath.Join(mpiCfg.InstallDir, "bin", "mpirun")
}

// GetExtraMpirunArgs returns all the required additional arguments required to use
// mpirun for a given configuration of MPI
func GetExtraMpirunArgs(mpiCfg *Config) []string {
	// Intel MPI is based on OFI so even for a simple TCP test, we need some extra arguments
	if mpiCfg.MpiImplm == "intel" {
		return []string{"-env", "FI_PROVIDER", "socket", "-env", "I_MPI_FABRICS", "ofi"}
	}

	return []string{""}
}

// UninstallHost uninstalls a version of MPI on the host that was previously installed by our tool
func UninstallHost(mpiCfg *Config, sysCfg *sys.Config) ExecResult {
	var res ExecResult

	log.Println("Uninstalling MPI on host...")

	if mpiCfg.MpiImplm == "intel" {
		return runIntelScript(mpiCfg, sysCfg, "uninstall")
	}

	return res
}

// InstallHost installs a specific version of MPI on the host
func InstallHost(myCfg *Config, sysCfg *sys.Config) ExecResult {
	var res ExecResult

	log.Println("Installing MPI on host...")
	res.Err = get(myCfg)
	if res.Err != nil {
		res.Err = fmt.Errorf("failed to download MPI from %s: %s", myCfg.URL, res.Err)
		return res
	}

	res.Err = unpack(myCfg)
	if res.Err != nil {
		res.Err = fmt.Errorf("failed to unpack MPI: %s", res.Err)
		return res
	}

	// Right now, we assume we do not have to install autotools, which is a bad assumption
	res.Err = configure(myCfg)
	if res.Err != nil {
		res.Err = fmt.Errorf("failed to configure MPI: %s", res.Err)
		return res
	}

	res = compile(myCfg, sysCfg)
	if res.Err != nil {
		res.Stderr = fmt.Sprintf("failed to compile MPI: %s", res.Err)
		return res
	}

	res = install(myCfg, sysCfg)
	if res.Err != nil {
		res.Stderr = fmt.Sprintf("failed to install MPI: %s", res.Err)
		return res
	}

	return res
}

func configure(mpiCfg *Config) error {
	log.Printf("- Configuring MPI for installation in %s...", mpiCfg.InstallDir)

	// Some sanity checks
	if mpiCfg.srcDir == "" || mpiCfg.InstallDir == "" {
		return fmt.Errorf("invalid parameter(s)")
	}

	// If the source code does not have a configure file, we simply skip the step
	configurePath := filepath.Join(mpiCfg.srcDir, "configure")
	if !util.FileExists(configurePath) {
		fmt.Printf("-> %s does not exist, skipping the configuration step\n", configurePath)
		return nil
	}

	var stdout, stderr bytes.Buffer
	log.Printf("* Execution (from %s): ./configure --prefix=%s", mpiCfg.srcDir, mpiCfg.InstallDir)
	cmd := exec.Command("./configure", "--prefix="+mpiCfg.InstallDir)
	cmd.Dir = mpiCfg.srcDir
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("command failed: %s - stdout: %s - stderr: %s", err, stdout.String(), stderr.String())
	}

	return nil
}

func runMake(mpiCfg *Config, stage string) error {
	// Some sanity checks
	if mpiCfg.srcDir == "" {
		return fmt.Errorf("invalid parameter(s)")
	}

	var stdout, stderr bytes.Buffer

	logMsg := "make -j4"
	makeCmd := exec.Command("make", "-j4")
	if stage == "install" {
		logMsg = "make -j4 install"
		makeCmd = exec.Command("make", "-j4", "install")
	}
	log.Printf("* Executing (from %s): %s", mpiCfg.srcDir, logMsg)

	makeCmd.Dir = mpiCfg.srcDir
	makeCmd.Stderr = &stderr
	makeCmd.Stdout = &stdout
	err := makeCmd.Run()
	if err != nil {
		return fmt.Errorf("command failed: %s - stdout: %s - stderr: %s", err, stdout.String(), stderr.String())
	}

	return nil
}

func updateDeffile(myCfg *Config, sysCfg *sys.Config, compileCfg *compileConfig) error {
	// Sanity checks
	if myCfg.MpiVersion == "" || myCfg.BuildDir == "" || myCfg.URL == "" ||
		myCfg.DefFile == "" || compileCfg.mpiVersionTag == "" ||
		compileCfg.mpiURLTag == "" || compileCfg.mpiTarballTag == "" ||
		sysCfg.TargetUbuntuDistro == "" {
		return fmt.Errorf("invalid parameter(s)")
	}

	if myCfg.tarball == "" {
		myCfg.tarball = path.Base(myCfg.URL)
	}

	data, err := ioutil.ReadFile(myCfg.DefFile)
	if err != nil {
		return fmt.Errorf("failed to read %s: %s", myCfg.DefFile, err)
	}

	var tarArgs string
	format := util.DetectTarballFormat(myCfg.tarball)
	switch format {
	case util.FormatBZ2:
		tarArgs = "-xjf"
	case util.FormatGZ:
		tarArgs = "-xzf"
	case util.FormatTAR:
		tarArgs = "-xf"
	default:
		return fmt.Errorf("un-supported tarball format for %s", myCfg.tarball)
	}

	if sysCfg.Debug {
		log.Printf("--> Replacing %s with %s", compileCfg.mpiVersionTag, myCfg.MpiVersion)
		log.Printf("--> Replacing %s with %s", compileCfg.mpiURLTag, myCfg.URL)
		log.Printf("--> Replacing %s with %s", compileCfg.mpiTarballTag, myCfg.tarball)
		log.Printf("--> Replacing TARARGS with %s", tarArgs)
	}

	content := string(data)
	content = strings.Replace(content, compileCfg.mpiVersionTag, myCfg.MpiVersion, -1)
	content = strings.Replace(content, compileCfg.mpiURLTag, myCfg.URL, -1)
	content = strings.Replace(content, compileCfg.mpiTarballTag, myCfg.tarball, -1)
	content = strings.Replace(content, "TARARGS", tarArgs, -1)
	content = deffile.UpdateDistroCodename(content, sysCfg.TargetUbuntuDistro)

	err = ioutil.WriteFile(myCfg.DefFile, []byte(content), 0)
	if err != nil {
		return fmt.Errorf("failed to write file %s: %s", myCfg.DefFile, err)
	}

	return nil
}

// GenerateDeffile generates the definition file for a MPI container.
func GenerateDeffile(mpiCfg *Config, sysCfg *sys.Config) error {
	log.Println("- Generating Singularity defintion file...")
	// Sanity checks
	if mpiCfg.BuildDir == "" {
		return fmt.Errorf("invalid parameter(s)")
	}

	var defFileName string
	var templateFileName string
	switch mpiCfg.MpiImplm {
	case "openmpi":
		defFileName = "ubuntu_ompi.def"
		if sysCfg.NetPipe {
			defFileName = "ubuntu_ompi_netpipe.def"
		}
		if sysCfg.IMB {
			defFileName = "ubuntu_ompi_imb.def"
		}
	case "mpich":
		defFileName = "ubuntu_mpich.def"
		if sysCfg.NetPipe {
			defFileName = "ubuntu_mpich_netpipe.def"
		}
		if sysCfg.IMB {
			defFileName = "ubuntu_mpich_imb.def"
		}
	case "intel":
		defFileName = "ubuntu_intel.def"
		if sysCfg.NetPipe {
			defFileName = "ubuntu_intel_netpipe.def"
		}
		if sysCfg.IMB {
			defFileName = "ubuntu_intel_imb.def"
		}
	default:
		return fmt.Errorf("unsupported MPI implementation: %s", mpiCfg.MpiImplm)
	}

	templateFileName = defFileName + ".tmpl"

	templateDefFile := filepath.Join(sysCfg.TemplateDir, templateFileName)
	mpiCfg.DefFile = filepath.Join(mpiCfg.BuildDir, defFileName)

	// Copy the definition file template to the temporary directory
	err := util.CopyFile(templateDefFile, mpiCfg.DefFile)
	if err != nil {
		return fmt.Errorf("failed to copy %s to %s: %s", templateDefFile, mpiCfg.DefFile, err)
	}

	// Copy the test file
	testFile := filepath.Join(sysCfg.TemplateDir, "mpitest.c")
	destTestFile := filepath.Join(mpiCfg.BuildDir, "mpitest.c")
	err = util.CopyFile(testFile, destTestFile)
	if err != nil {
		return fmt.Errorf("failed to copy %s to %s: %s", testFile, destTestFile, err)
	}

	// Update the definition file for the specific version of MPI we are testing
	switch mpiCfg.MpiImplm {
	case "openmpi":
		err := updateOMPIDefFile(mpiCfg, sysCfg)
		if err != nil {
			return fmt.Errorf("failed to update OMPI template: %s", err)
		}
	case "mpich":
		err := updateMPICHDefFile(mpiCfg, sysCfg)
		if err != nil {
			return fmt.Errorf("failed to update MPICH template: %s", err)
		}
	case "intel":
		err := updateIntelMPIDefFile(mpiCfg, sysCfg)
		if err != nil {
			return fmt.Errorf("failed to update IMPI template: %s", err)
		}
	default:
		return fmt.Errorf("unsupported MPI implementation: %s", mpiCfg.MpiImplm)
	}

	// In debug mode, we save the def file that was generated to the scratch directory
	if sysCfg.Debug {
		backupFile := filepath.Join(sysCfg.ScratchDir, defFileName)
		log.Printf("-> Backing up %s to %s", mpiCfg.DefFile, backupFile)
		err = util.CopyFile(mpiCfg.DefFile, backupFile)
		if err != nil {
			log.Printf("-> error while backing up %s to %s", mpiCfg.DefFile, backupFile)
		}
	}

	return nil
}

// CreateContainer creates a container based on a MPI configuration
func CreateContainer(mpiCfg *Config, sysCfg *sys.Config) error {
	var err error

	// Some sanity checks
	if mpiCfg.BuildDir == "" {
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

	if mpiCfg.ContainerName == "" {
		mpiCfg.ContainerName = "singularity_mpi.sif"
	}

	if mpiCfg.ContainerPath == "" {
		mpiCfg.ContainerPath = filepath.Join(mpiCfg.InstallDir, mpiCfg.ContainerName)
	}

	log.Printf("- Creating image %s...", mpiCfg.ContainerPath)

	// We only let the mpirun command run for 10 minutes max
	ctx, cancel := context.WithTimeout(context.Background(), sys.CmdTimeout*2*time.Minute)
	defer cancel()

	// The definition file is ready so we simple build the container using the Singularity command
	if sysCfg.Debug {
		err = checker.CheckDefFile(mpiCfg.DefFile)
		if err != nil {
			return err
		}
	}

	log.Printf("-> Using definition file %s", mpiCfg.DefFile)
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, sudoBin, sysCfg.SingularityBin, "build", mpiCfg.ContainerPath, mpiCfg.DefFile)
	cmd.Dir = mpiCfg.BuildDir
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to execute command - stdout: %s; stderr: %s; err: %s", stdout.String(), stderr.String(), err)
	}

	return nil
}
