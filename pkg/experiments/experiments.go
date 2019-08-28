// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package experiments

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
	"regexp"
	"strings"
	"time"

	"github.com/sylabs/singularity-mpi/internal/pkg/checker"
	util "github.com/sylabs/singularity-mpi/internal/pkg/util/file"
)

// SysConfig captures some system configuration aspects that are necessary
// to run experiments
type SysConfig struct {
	ConfigFile         string // Path to the configuration file describing experiments
	TargetUbuntuDistro string // The version of Ubuntu that we will run in the containers
	BinPath            string // Path to the current binary
	CurPath            string // Current path
	EtcDir             string // Path to the directory with the configuration files
	TemplateDir        string // Where the template are
	ScratchDir         string // Where a copy generated files are saved for debugging
	SedBin             string // Path to the sed binary
	SingularityBin     string // Path to the singularity binary
	OutputFile         string // Path the output file
	NetPipe            bool   // Execute NetPipe as test
	IMB                bool   // Execute IMB as test
	OfiCfgFile         string // Absolute path to the OFI configuration file
	Ifnet              string // Network interface to use (e.g., required to setup OFI)
	Debug              bool   // Debug mode is active/inactive
}

type mpiConfig struct {
	mpiImplm   string // MPI implementation ID (e.g., OMPI, MPICH)
	mpiVersion string // Version of the MPI implementation to use
	url        string // URL to use to download the tarball
	tarball    string
	srcPath    string // Path to the downloaded tarball
	srcDir     string // Where the source has been untared
	buildDir   string // Directory where to compile
	installDir string // Directory where to install the compiled software
	//	m4SrcPath       string // Path to the m4 tarball
	//	autoconfSrcPath string // Path to the autoconf tarball
	//	automakeSrcPath string // Path to the automake tarball
	//	libtoolsSrcPath string // Path to the libtools tarball
	defFile       string // Definition file used to create MPI container
	containerPath string // Path to the container image
	testPath      string // Path to the test to run within the container
}

type compileConfig struct {
	mpiVersionTag string
	mpiURLTag     string
	mpiTarballTag string
	//	mpiTarArgsTag string
}

// Experiment is a structure that represents the configuration of an experiment
type Experiment struct {
	MPIImplm            string
	VersionHostMPI      string
	URLHostMPI          string
	VersionContainerMPI string
	URLContainerMPI     string
}

const (
	cmdTimeout = 10
)

func copyMPITarball(mpiCfg *mpiConfig) error {
	// Some sanity checks
	if mpiCfg.url == "" {
		return fmt.Errorf("invalid parameter(s)")
	}

	// Figure out the name of the file if we do not already have it
	if mpiCfg.tarball == "" {
		mpiCfg.tarball = path.Base(mpiCfg.url)
	}

	targetTarballPath := filepath.Join(mpiCfg.buildDir, mpiCfg.tarball)
	// The begining of the URL starts with 'file://' which we do not want
	err := util.CopyFile(mpiCfg.url[7:], targetTarballPath)
	if err != nil {
		return fmt.Errorf("cannot copy file %s to %s: %s", mpiCfg.url, targetTarballPath, err)
	}

	mpiCfg.srcPath = filepath.Join(mpiCfg.buildDir, mpiCfg.tarball)

	return nil
}

func downloadMPI(mpiCfg *mpiConfig) error {
	log.Println("- Downloading MPI...")

	// Sanity checks
	if mpiCfg.url == "" || mpiCfg.buildDir == "" {
		return fmt.Errorf("invalid parameter(s)")
	}

	// TODO do not assume wget
	binPath, err := exec.LookPath("wget")
	if err != nil {
		return fmt.Errorf("cannot find wget: %s", err)
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.Command(binPath, mpiCfg.url)
	cmd.Dir = mpiCfg.buildDir
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("command failed: %s - stdout: %s - stderr: %s", err, stdout.String(), stderr.String())
	}

	// FIXME we currently assume that we have one and only one file in the
	// directory This is not a fair assumption, especially while debugging
	// when we do not wipe out the temporary directories
	files, err := ioutil.ReadDir(mpiCfg.buildDir)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %s", mpiCfg.buildDir, err)
	}
	if len(files) != 1 {
		return fmt.Errorf("inconsistent temporary %s directory, %d files instead of 1", mpiCfg.buildDir, len(files))
	}
	mpiCfg.tarball = files[0].Name()
	mpiCfg.srcPath = filepath.Join(mpiCfg.buildDir, files[0].Name())

	return nil
}

func getMPI(mpiCfg *mpiConfig) error {
	log.Println("- Getting MPI...")

	// Sanity checks
	if mpiCfg.url == "" {
		return fmt.Errorf("invalid parameter(s)")
	}

	// Detect the type of URL, e.g., file vs. http*
	urlFormat := util.DetectURLType(mpiCfg.url)
	if urlFormat == "" {
		return fmt.Errorf("impossible to detect type from URL %s", mpiCfg.url)
	}

	switch urlFormat {
	case util.FileURL:
		err := copyMPITarball(mpiCfg)
		if err != nil {
			return fmt.Errorf("impossible to copy the MPI tarball: %s", err)
		}
	case util.HttpURL:
		err := downloadMPI(mpiCfg)
		if err != nil {
			return fmt.Errorf("impossible to download MPI: %s", err)
		}
	default:
		return fmt.Errorf("impossible to detect URL type: %s", mpiCfg.url)
	}

	return nil
}

func unpackMPI(mpiCfg *mpiConfig) error {
	log.Println("- Unpacking MPI...")

	// Sanity checks
	if mpiCfg.srcPath == "" || mpiCfg.buildDir == "" {
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

	// Figure out the tar argument based on the format
	tarArg := ""
	switch format {
	case util.FormatBZ2:
		tarArg = "-xjf"
	case util.FormatGZ:
		tarArg = "-xzf"
	case util.FormatTAR:
		tarArg = "-xf"
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
	mpiCfg.srcDir = filepath.Join(mpiCfg.buildDir, entries[0].Name())

	return nil
}

func configureMPI(mpiCfg *mpiConfig) error {
	log.Println("- Configuring MPI...")

	// Some sanity checks
	if mpiCfg.srcDir == "" || mpiCfg.installDir == "" {
		return fmt.Errorf("invalid parameter(s)")
	}

	// If the source code does not have a configure file, we simply skip the step
	configurePath := filepath.Join(mpiCfg.srcDir, "configure")
	if !util.FileExists(configurePath) {
		fmt.Printf("-> %s does not exist, skipping the configuration step\n", configurePath)
		return nil
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.Command("./configure", "--prefix="+mpiCfg.installDir)
	cmd.Dir = mpiCfg.srcDir
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("command failed: %s - stdout: %s - stderr: %s", err, stdout.String(), stderr.String())
	}

	return nil
}

func runMake(mpiCfg *mpiConfig) error {
	// Some sanity checks
	if mpiCfg.srcDir == "" {
		return fmt.Errorf("invalid parameter(s)")
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

func compileMPI(mpiCfg *mpiConfig, sysCfg *SysConfig) error {
	log.Println("- Compiling MPI...")
	if mpiCfg.srcDir == "" {
		return fmt.Errorf("invalid parameter(s)")
	}

	makefilePath := filepath.Join(mpiCfg.srcDir, "Makefile")
	if util.FileExists(makefilePath) {
		return runMake(mpiCfg)
	}

	fmt.Println("-> No Makefile, trying to figure out how to compile/install MPI...")
	if mpiCfg.mpiImplm == "intel" {
		err := setupIntelInstallScript(mpiCfg, sysCfg)
		if err != nil {
			return err
		}
		return runIntelScript(mpiCfg, sysCfg, "install")
	}

	return nil
}

func postExecutionDataMgt(exp Experiment, sysCfg *SysConfig, output string) (string, error) {
	if sysCfg.NetPipe {
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			if strings.Contains(line, "Completed with") {
				tokens := strings.Split(line, " ")
				note := "max bandwidth: " + util.CleanupString(tokens[13]) + " " + util.CleanupString(tokens[14]) + "; latency: " + util.CleanupString(tokens[20]) + " " + util.CleanupString(tokens[21])
				return note, nil
			}
		}
	}
	return "", nil
}

func getEnvPath(mpiCfg *mpiConfig) string {
	// Intel MPI is installing the binaries and libraries in a quite complex setup
	if mpiCfg.mpiImplm == "intel" {
		return filepath.Join(mpiCfg.installDir, intelInstallPathPrefix, "bin") + ":" + os.Getenv("PATH")
	}

	return filepath.Join(mpiCfg.installDir, "bin") + ":" + os.Getenv("PATH")
}

func getEnvLDPath(mpiCfg *mpiConfig) string {
	// Intel MPI is installing the binaries and libraries in a quite complex setup
	if mpiCfg.mpiImplm == "intel" {
		return filepath.Join(mpiCfg.installDir, intelInstallPathPrefix, "lib") + ":" + os.Getenv("LD_LIBRARY_PATH")
	}

	return filepath.Join(mpiCfg.installDir, "lib") + ":" + os.Getenv("LD_LIBRARY_PATH")
}

func getPathToMpirun(mpiCfg *mpiConfig) string {
	// Intel MPI is installing the binaries and libraries in a quite complex setup
	if mpiCfg.mpiImplm == "intel" {
		return filepath.Join(mpiCfg.buildDir, intelInstallPathPrefix, "bin/mpiexec")
	}

	return filepath.Join(mpiCfg.installDir, "bin", "mpirun")
}

func getExtraMpirunArgs(mpiCfg *mpiConfig) []string {
	// Intel MPI is based on OFI so even for a simple TCP test, we need some extra arguments
	if mpiCfg.mpiImplm == "intel" {
		return []string{"-env", "FI_PROVIDER", "socket", "-env", "I_MPI_FABRICS", "ofi"}
	}

	return []string{""}
}

// Run configure, install and execute a given experiment
func Run(exp Experiment, sysCfg *SysConfig) (bool, string, error) {
	var myHostMPICfg mpiConfig
	var myContainerMPICfg mpiConfig
	var err error

	/* CREATE THE HOST MPI CONFIGURATION */

	// Create a temporary directory where to compile MPI
	myHostMPICfg.buildDir, err = ioutil.TempDir("", "mpi_build_"+exp.VersionHostMPI+"-")
	if err != nil {
		return false, "", fmt.Errorf("failed to create compile directory: %s", err)
	}
	defer os.RemoveAll(myHostMPICfg.buildDir)

	// Create a temporary directory where to install MPI
	myHostMPICfg.installDir, err = ioutil.TempDir("", "mpi_install_"+exp.VersionHostMPI+"-")
	if err != nil {
		return false, "", fmt.Errorf("failed to create install directory: %s", err)
	}
	defer os.RemoveAll(myHostMPICfg.installDir)

	myHostMPICfg.mpiImplm = exp.MPIImplm
	myHostMPICfg.url = exp.URLHostMPI
	myHostMPICfg.mpiVersion = exp.VersionHostMPI

	log.Println("* Host MPI Configuration *")
	log.Println("-> Building MPI in", myHostMPICfg.buildDir)
	log.Println("-> Installing MPI in", myHostMPICfg.installDir)
	log.Println("-> MPI implementation:", myHostMPICfg.mpiImplm)
	log.Println("-> MPI version:", myHostMPICfg.mpiVersion)
	log.Println("-> MPI URL:", myHostMPICfg.url)

	/* CREATE THE CONTAINER MPI CONFIGURATION */

	// Create a temporary directory where the container will be built
	myContainerMPICfg.buildDir, err = ioutil.TempDir("", "mpi_container_"+exp.VersionContainerMPI+"-")
	if err != nil {
		return false, "", fmt.Errorf("failed to create directory to build container: %s", err)
	}
	defer os.RemoveAll(myContainerMPICfg.buildDir)

	myContainerMPICfg.mpiImplm = exp.MPIImplm
	myContainerMPICfg.url = exp.URLContainerMPI
	myContainerMPICfg.mpiVersion = exp.VersionContainerMPI

	log.Println("* Container MPI configuration *")
	log.Println("-> Build container in", myContainerMPICfg.buildDir)
	log.Println("-> MPI implementation:", myContainerMPICfg.mpiImplm)
	log.Println("-> MPI version:", myContainerMPICfg.mpiVersion)
	log.Println("-> MPI URL:", myContainerMPICfg.url)

	/* INSTALL MPI ON THE HOST */

	err = installHostMPI(&myHostMPICfg, sysCfg)
	if err != nil {
		return false, "", fmt.Errorf("failed to install host MPI: %s", err)
	}
	defer func() {
		err = uninstallHostMPI(&myHostMPICfg, sysCfg)
		if err != nil {
			log.Fatal(err)
		}
	}()

	/* CREATE THE MPI CONTAINER */

	err = createMPIContainer(&myContainerMPICfg, sysCfg)
	if err != nil {
		return false, "", fmt.Errorf("failed to create container: %s", err)
	}

	/* PREPARE THE COMMAND TO RUN THE ACTUAL TEST */

	log.Println("Running Test(s)...")
	// We only let the mpirun command run for 10 minutes max
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout*time.Minute)
	defer cancel()

	// Regex to catch errors where mpirun returns 0 but is known to have failed because displaying the help message
	var re = regexp.MustCompile(`^(\n?)Usage:`)

	var stdout, stderr bytes.Buffer
	newPath := getEnvPath(&myHostMPICfg)
	newLDPath := getEnvLDPath(&myHostMPICfg)

	mpirunBin := getPathToMpirun(&myHostMPICfg)

	// We have to be careful: if we leave an empty argument in the slice, it may lead to a mpirun failure.
	var mpiCmd *exec.Cmd
	// We really do not want to do this but MPICH is being picky about args so for now, it will do the job.
	if myHostMPICfg.mpiImplm == "intel" {
		extraArgs := getExtraMpirunArgs(&myHostMPICfg)

		args := []string{"-np", "2", "singularity", "exec", myContainerMPICfg.containerPath, myContainerMPICfg.testPath}
		if len(extraArgs) > 0 {
			args = append(extraArgs, args...)
		}
		mpiCmd = exec.CommandContext(ctx, mpirunBin, args...)
		log.Printf("-> Running: %s %s", mpirunBin, strings.Join(args, " "))
	} else {
		mpiCmd = exec.CommandContext(ctx, mpirunBin, "-np", "2", "singularity", "exec", myContainerMPICfg.containerPath, myContainerMPICfg.testPath)
		log.Printf("-> Running: %s %s", mpirunBin, strings.Join([]string{"-np", "2", "singularity", "exec", myContainerMPICfg.containerPath, myContainerMPICfg.testPath}, " "))
	}
	mpiCmd.Env = append([]string{"LD_LIBRARY_PATH=" + newLDPath}, os.Environ()...)
	mpiCmd.Env = append([]string{"PATH=" + newPath}, os.Environ()...)
	mpiCmd.Stdout = &stdout
	mpiCmd.Stderr = &stderr
	log.Printf("-> PATH=%s", newPath)
	log.Printf("-> LD_LIBRARY_PATH=%s\n", newLDPath)
	err = mpiCmd.Run()
	if err != nil || ctx.Err() == context.DeadlineExceeded || re.Match(stdout.Bytes()) {
		log.Printf("[INFO] mpirun command failed - stdout: %s - stderr: %s - err: %s\n", stdout.String(), stderr.String(), err)
		return false, "", nil
	}

	log.Printf("Successful run - stdout: %s; stderr: %s\n", stdout.String(), stderr.String())

	log.Println("Handling data...")
	note, err := postExecutionDataMgt(exp, sysCfg, stdout.String())
	if err != nil {
		return true, "", fmt.Errorf("failed to handle data: %s", err)
	}

	log.Println("NOTE: ", note)

	return true, note, nil
}

func uninstallHostMPI(mpiCfg *mpiConfig, sysCfg *SysConfig) error {
	log.Println("Uninstalling MPI on host...")

	if mpiCfg.mpiImplm == "intel" {
		return runIntelScript(mpiCfg, sysCfg, "uninstall")
	}

	return nil
}

func installHostMPI(myCfg *mpiConfig, sysCfg *SysConfig) error {
	log.Println("Installing MPI on host...")
	err := getMPI(myCfg)
	if err != nil {
		return fmt.Errorf("failed to download MPI from %s: %s", myCfg.url, err)
	}

	err = unpackMPI(myCfg)
	if err != nil {
		return fmt.Errorf("failed to unpack MPI: %s", err)
	}

	// Right now, we assume we do not have to install autotools, which is a bad assumption
	err = configureMPI(myCfg)
	if err != nil {
		return fmt.Errorf("failed to configure MPI: %s", err)
	}

	err = compileMPI(myCfg, sysCfg)
	if err != nil {
		return fmt.Errorf("failed to compile MPI: %s", err)
	}

	return nil
}

func createContainerImage(myCfg *mpiConfig, sysCfg *SysConfig) error {
	var err error

	log.Println("- Creating image...")
	// Some sanity checks
	if myCfg.buildDir == "" {
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

	imgName := "singularity_mpi.sif"

	myCfg.containerPath = filepath.Join(myCfg.buildDir, imgName)

	// We only let the mpirun command run for 10 minutes max
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout*2*time.Minute)
	defer cancel()

	// The definition file is ready so we simple build the container using the Singularity command
	if sysCfg.Debug {
		err = checker.CheckDefFile(myCfg.defFile)
		if err != nil {
			return err
		}
	}

	log.Printf("-> Using definition file %s", myCfg.defFile)
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, sudoBin, sysCfg.SingularityBin, "build", imgName, myCfg.defFile)
	cmd.Dir = myCfg.buildDir
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to execute command - stdout: %s; stderr: %s; err: %s", stdout.String(), stderr.String(), err)
	}

	myCfg.testPath = filepath.Join("/", "opt", "mpitest")
	if sysCfg.NetPipe {
		myCfg.testPath = filepath.Join("/", "opt", "NetPIPE-5.1.4", "NPmpi")
	}
	if sysCfg.IMB {
		myCfg.testPath = filepath.Join("/", "opt", "mpi-benchmarks", "IMB-MPI1")
	}

	return nil
}

// CreateMPIContainer creates a container based on a specific configuration.
func createMPIContainer(myCfg *mpiConfig, sysCfg *SysConfig) error {
	log.Println("Creating MPI container...")
	err := generateDefFile(myCfg, sysCfg)
	if err != nil {
		return fmt.Errorf("failed to generate Singularity definition file: %s", err)
	}

	err = createContainerImage(myCfg, sysCfg)
	if err != nil {
		return fmt.Errorf("failed to create container image: %s", err)
	}

	return nil
}

// GetMPIImplemFromExperiments returns the MPI implementation that is associated
// to the experiments
func GetMPIImplemFromExperiments(experiments []Experiment) string {
	// Fair assumption: all experiments are based on the same MPI
	// implementation (we actually check for that and the implementation
	// is only included in the experiment structure so that the structure
	// is self-contained).
	if len(experiments) == 0 {
		return ""
	}

	return experiments[0].MPIImplm
}

// GetOutputFilename returns the name of the file that is associated to the experiments
// to run
func GetOutputFilename(mpiImplem string, sysCfg *SysConfig) error {
	sysCfg.OutputFile = mpiImplem + "-init-results.txt"

	if sysCfg.NetPipe {
		sysCfg.OutputFile = mpiImplem + "-netpipe-results.txt"
	}

	if sysCfg.IMB {
		sysCfg.OutputFile = mpiImplem + "-imb-results.txt"
	}

	return nil
}
