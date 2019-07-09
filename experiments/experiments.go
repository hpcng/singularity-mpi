// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package experiments

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// SysConfig captures some system configuration aspects that are necessary
// to run experiments
type SysConfig struct {
	ConfigFile     string // Path to the configuration file describing experiments
	BinPath        string // Path to the current binary
	CurPath        string // Current path
	EtcDir         string // Path to the directory with the configuration files
	TemplateDir    string // Where the template are
	SedBin         string // Path to the sed binary
	SingularityBin string // Path to the singularity binary
	OutputFile     string // Path the output file
	NetPipe        bool   // Execute NetPipe as test
	OfiCfgFile     string // Absolute path to the OFI configuration file
	Ifnet          string // Network interface to use (e.g., required to setup OFI)
}

type mpiConfig struct {
	mpiImplm        string // MPI implementation ID (e.g., OMPI, MPICH)
	mpiVersion      string // Version of the MPI implementation to use
	url             string // URL to use to download the tarball
	tarball         string
	srcPath         string // Path to the downloaded tarball
	srcDir          string // Where the source has been untared
	buildDir        string // Directory where to compile
	installDir      string // Directory where to install the compiled software
	m4SrcPath       string // Path to the m4 tarball
	autoconfSrcPath string // Path to the autoconf tarball
	automakeSrcPath string // Path to the automake tarball
	libtoolsSrcPath string // Path to the libtools tarball
	defFile         string // Definition file used to create MPI container
	containerPath   string // Path to the container image
	testPath        string // Path to the test to run within the container
}

type compileConfig struct {
	mpiVersionTag string
	mpiURLTag     string
	mpiTarballTag string
	mpiTarArgsTag string
}

// Experiment is a structure that represents the configuration of an experiment
type Experiment struct {
	MPIImplm            string
	VersionHostMPI      string
	URLHostMPI          string
	VersionContainerMPI string
	URLContainerMPI     string
}

// Constants defining the URL types
const (
	fileURL = "file"
	httpURL = "http"
)

// Constants defining the format of the MPI package
const (
	formatBZ2 = "bz2"
	formatGZ  = "gz"
	formatTAR = "tar"
)

// Constants related to Intel MPI
const (
	intelInstallPathPrefix         = "compilers_and_libraries/linux/mpi/intel64"
	intelInstallConfFile           = "silent_install.cfg"
	intelUninstallConfFile         = "silent_uninstall.cfg"
	intelInstallConfFileTemplate   = intelInstallConfFile + ".tmpl"
	intelUninstallConfFileTemplate = intelUninstallConfFile + ".tmpl"
)

const (
	cmdTimeout = 10
)

func detectURLType(url string) string {
	if url[:7] == "file://" {
		return "file"
	}

	if url[:4] == "http" {
		return "http"
	}

	// Unsupported type
	return ""
}

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
	err := copyFile(mpiCfg.url[7:], targetTarballPath)
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
	fmt.Println("- Getting MPI...")

	// Sanity checks
	if mpiCfg.url == "" {
		return fmt.Errorf("invalid parameter(s)")
	}

	// Detect the type of URL, e.g., file vs. http*
	urlFormat := detectURLType(mpiCfg.url)
	if urlFormat == "" {
		return fmt.Errorf("impossible to detect type from URL %s", mpiCfg.url)
	}

	switch urlFormat {
	case fileURL:
		err := copyMPITarball(mpiCfg)
		if err != nil {
			return fmt.Errorf("impossible to copy the MPI tarball: %s", err)
		}
	case httpURL:
		err := downloadMPI(mpiCfg)
		if err != nil {
			return fmt.Errorf("impossible to download MPI: %s", err)
		}
	default:
		return fmt.Errorf("impossible to detect URL type: %s", mpiCfg.url)
	}

	return nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func detectTarballFormat(filepath string) string {
	if path.Ext(filepath) == ".bz2" {
		return formatBZ2
	}

	if path.Ext(filepath) == ".gz" {
		return formatGZ
	}

	if path.Ext(filepath) == ".tar" {
		return formatTAR
	}

	return ""
}

func unpackMPI(mpiCfg *mpiConfig) error {
	log.Println("- Unpacking MPI...")

	// Sanity checks
	if mpiCfg.srcPath == "" || mpiCfg.buildDir == "" {
		return fmt.Errorf("invalid parameter(s)")
	}

	// Figure out the extension of the tarball
	format := detectTarballFormat(mpiCfg.srcPath)

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
	case formatGZ:
		tarArg = "-xzf"
	case formatTAR:
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
	if !fileExists(configurePath) {
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

func compileMPI(mpiCfg *mpiConfig) error {
	log.Println("- Compiling MPI...")

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

func setupIntelInstallScript(mpiCfg *mpiConfig, sysCfg *SysConfig) error {
	// Copy silent script templates to install Intel MPI
	intelSilentInstallTemplate := filepath.Join(sysCfg.TemplateDir, "intel", intelInstallConfFileTemplate)
	intelSilentInstallConfig := filepath.Join(mpiCfg.srcDir, intelInstallConfFile)
	fmt.Printf("Copying %s to %s\n", intelSilentInstallTemplate, intelSilentInstallConfig)
	err := copyFile(intelSilentInstallTemplate, intelSilentInstallConfig)
	if err != nil {
		return fmt.Errorf("failed to copy %s to %s: %s", intelSilentInstallTemplate, intelSilentInstallConfig, err)
	}

	intelSilentUninstallTemplate := filepath.Join(sysCfg.TemplateDir, "intel", intelUninstallConfFileTemplate)
	intelSilentUninstallConfig := filepath.Join(mpiCfg.srcDir, intelUninstallConfFile)
	err = copyFile(intelSilentUninstallTemplate, intelSilentUninstallConfig)
	if err != nil {
		return fmt.Errorf("failed to copy %s to %s: %s", intelSilentUninstallTemplate, intelSilentUninstallConfig, err)
	}

	// Update the templates
	err = updateIntelTemplates(mpiCfg, sysCfg)
	if err != nil {
		return fmt.Errorf("unable to update Intel templates: %s", err)
	}

	return nil
}

func runIntelScript(mpiCfg *mpiConfig, sysCfg *SysConfig, phase string) error {
	var configFile string

	fmt.Printf("Running %s script...\n", phase)

	switch phase {
	case "install":
		configFile = intelInstallConfFile
	case "uninstall":
		configFile = intelUninstallConfFile
	default:
		return fmt.Errorf("unknown phase: %s", phase)
	}

	// Run the install or uninstall script
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("./install.sh", "--silent", configFile)
	cmd.Dir = mpiCfg.srcDir
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("command failed: %s - stdout: %s - stderr: %s", err, stdout.String(), stderr.String())
	}

	return nil
}

func compileMPI(mpiCfg *mpiConfig, sysCfg *SysConfig) error {
	fmt.Println("- Compiling MPI...")
	if mpiCfg.srcDir == "" {
		return fmt.Errorf("invalid parameter(s)")
	}

	makefilePath := filepath.Join(mpiCfg.srcDir, "Makefile")
	if fileExists(makefilePath) {
		return runMake(mpiCfg)
	}

	fmt.Println("-> No Makefile, trying to figure out how to compile/install MPI...")
	if mpiCfg.mpiImplm == "intel" {
		setupIntelInstallScript(mpiCfg, sysCfg)
		return runIntelScript(mpiCfg, sysCfg, "install")
	}

	return nil
}

func cleanupString(str string) string {
	// Remove all color escape sequences from string
	reg := regexp.MustCompile(`\\x1b\[[0-9]+m`)
	str = reg.ReplaceAllString(str, "")

	str = strings.Replace(str, `\x1b`+"[0m", "", -1)
	return strings.Replace(str, `\x1b`+"[33m", "", -1)
}

func postExecutionDataMgt(exp Experiment, sysCfg *SysConfig, output string) (string, error) {
	if sysCfg.NetPipe {
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			if strings.Contains(line, "Completed with") {
				tokens := strings.Split(line, " ")
				note := "max bandwidth: " + cleanupString(tokens[13]) + " " + cleanupString(tokens[14]) + "; latency: " + cleanupString(tokens[20]) + " " + cleanupString(tokens[21])
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
	defer uninstallHostMPI(&myHostMPICfg, sysCfg)

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
	extraArgs := getExtraMpirunArgs(&myHostMPICfg)

	cmdArgs := append(extraArgs, "-np", "2", "singularity", "exec", myContainerMPICfg.containerPath, myContainerMPICfg.testPath)
	//	mpiCmd := exec.CommandContext(ctx, mpirunBin, "-env", "FI_PROVIDER", "socket", "-env", "I_MPI_FABRICS", "ofi", "-np", "2", "singularity", "exec", myContainerMPICfg.containerPath, myContainerMPICfg.testPath)
	mpiCmd := exec.CommandContext(ctx, mpirunBin, cmdArgs...)
	mpiCmd.Env = append([]string{"LD_LIBRARY_PATH=" + newLDPath}, os.Environ()...)
	mpiCmd.Env = append([]string{"PATH=" + newPath}, os.Environ()...)
	mpiCmd.Stdout = &stdout
	mpiCmd.Stderr = &stderr
	log.Printf("-> Running: %s %s", mpirunBin, strings.Join(cmdArgs, " "))
	log.Printf("-> PATH=%s", newPath)
	log.Printf("-> LD_LIBRARY_PATH=%s\n", newLDPath)
	err = mpiCmd.Run()
	if err != nil || ctx.Err() == context.DeadlineExceeded || re.Match([]byte(stdout.String())) {
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

func copyFile(src string, dst string) error {
	// Check that the source file is valid
	srcStat, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("cannot access file %s: %s", src, err)
	}

	if !srcStat.Mode().IsRegular() {
		return fmt.Errorf("invalid source file %s: %s", src, err)
	}

	// Actually do the copy
	s, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open %s: %s", src, err)
	}
	defer s.Close()

	d, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create %s: %s", dst, err)
	}
	defer d.Close()

	_, err = io.Copy(d, s)
	if err != nil {
		return fmt.Errorf("unabel to copy file from %s to %s: %s", src, dst, err)
	}

	return nil
}

func doUpdateDefFile(myCfg *mpiConfig, sysCfg *SysConfig, compileCfg *compileConfig) error {
	var err error

	// Sanity checks
	if myCfg.mpiVersion == "" || myCfg.buildDir == "" || myCfg.url == "" || myCfg.defFile == "" {
		return fmt.Errorf("invalid parameter(s)")
	}

	if myCfg.tarball == "" {
		myCfg.tarball = path.Base(myCfg.url)
	}

	data, err := ioutil.ReadFile(myCfg.defFile)
	if err != nil {
		return fmt.Errorf("failed to read %s: %s", myCfg.defFile, err)
	}

	var tarArgs string
	format := detectTarballFormat(myCfg.tarball)
	switch format {
	case formatBZ2:
		tarArgs = "-xjf"
	case formatGZ:
		tarArgs = "-xzf"
	case formatTAR:
		tarArgs = "-xf"
	default:
		return fmt.Errorf("un-supported tarball format for %s", myCfg.tarball)
	}

	content := string(data)
	content = strings.Replace(content, compileCfg.mpiVersionTag, myCfg.mpiVersion, -1)
	content = strings.Replace(content, compileCfg.mpiURLTag, myCfg.url, -1)
	content = strings.Replace(content, compileCfg.mpiTarballTag, myCfg.tarball, -1)
	content = strings.Replace(content, "TARARGS", tarArgs, -1)

	err = ioutil.WriteFile(myCfg.defFile, []byte(content), 0)
	if err != nil {
		return fmt.Errorf("failed to write file %s: %s", myCfg.defFile, err)
	}

	return nil
}

func updateMPICHDefFile(myCfg *mpiConfig, sysCfg *SysConfig) error {
	var compileCfg compileConfig
	compileCfg.mpiVersionTag = "MPICHVERSION"
	compileCfg.mpiURLTag = "MPICHURL"
	compileCfg.mpiTarballTag = "MPICHTARBALL"

	err := doUpdateDefFile(myCfg, sysCfg, &compileCfg)
	if err != nil {
		return fmt.Errorf("failed to update MPICH definition file")
	}

	return nil
}

func updateOMPIDefFile(myCfg *mpiConfig, sysCfg *SysConfig) error {
	var compileCfg compileConfig
	compileCfg.mpiVersionTag = "OMPIVERSION"
	compileCfg.mpiTarballTag = "OMPITARBALL"

	err := doUpdateDefFile(myCfg, sysCfg, &compileCfg)
	if err != nil {
		return fmt.Errorf("failed to update Open MPI definition file")
	}

	return nil
}

func updateIntelMPIDefFile(mpiCfg *mpiConfig, sysCfg *SysConfig) error {
	// Intel MPI is very special so we have IMPI-specific code and it is okay

	// First we need to specialy prepare install & uninstall configuration file
	// Assumptions:
	// - code unpacked in /tmp/impi
	// - code installed in /opt/impi (but remember that the binaries and libraries are deep a sub-directory)
	const (
		containerIMPIInstallDir = "/opt/impi"
	)

	if mpiCfg.tarball == "" {
		mpiCfg.tarball = path.Base(mpiCfg.url)
	}

	// Sanity checks
	if mpiCfg.mpiVersion == "" || mpiCfg.tarball == "" || mpiCfg.defFile == "" {
		return fmt.Errorf("invalid parameter(s)")
	}

	// Copy the install & uninstall configuration file to the temporary directory used to build the container
	// These install &uninstall configuation file will be used wihtin the container to install IMPI
	srcInstallConfFile := filepath.Join(sysCfg.TemplateDir, "intel", intelInstallConfFileTemplate)
	destInstallConfFile := filepath.Join(mpiCfg.buildDir, intelInstallConfFile)
	srcUninstallConfFile := filepath.Join(sysCfg.TemplateDir, "intel", intelUninstallConfFileTemplate)
	destUninstallConfFile := filepath.Join(mpiCfg.buildDir, intelUninstallConfFile)
	err := copyFile(srcInstallConfFile, destInstallConfFile)
	if err != nil {
		return fmt.Errorf("enable to copy %s to %s: %s", srcInstallConfFile, destInstallConfFile, err)
	}
	err = copyFile(srcUninstallConfFile, destUninstallConfFile)
	if err != nil {
		return fmt.Errorf("enable to copy %s to %s: %s", srcUninstallConfFile, destUninstallConfFile, err)
	}

	err = updateIntelTemplate(destInstallConfFile, containerIMPIInstallDir)
	if err != nil {
		return fmt.Errorf("unable to update IMPI template %s: %s", destInstallConfFile, err)
	}

	err = updateIntelTemplate(destUninstallConfFile, containerIMPIInstallDir)
	if err != nil {
		return fmt.Errorf("unable to update IMPI template %s: %s", destUninstallConfFile, err)
	}

	// Then we have to put together a valid def file
	data, err := ioutil.ReadFile(mpiCfg.defFile)
	if err != nil {
		return fmt.Errorf("failed to read %s: %s", mpiCfg.defFile, err)
	}

	content := string(data)
	content = strings.Replace(content, "IMPIVERSION", mpiCfg.mpiVersion, -1)
	content = strings.Replace(content, "IMPITARBALL", mpiCfg.url[7:], -1)
	content = strings.Replace(content, "IMPIDIR", filepath.Join(containerIMPIInstallDir, mpiCfg.mpiVersion, intelInstallPathPrefix), -1)
	content = strings.Replace(content, "IMPIINSTALLCONFFILE", intelInstallConfFile, -1)
	content = strings.Replace(content, "IMPIUNINSTALLCONFFILE", intelUninstallConfFile, -1)
	content = strings.Replace(content, "NETWORKINTERFACE", sysCfg.Ifnet, -1)

	err = ioutil.WriteFile(mpiCfg.defFile, []byte(content), 0)
	if err != nil {
		return fmt.Errorf("failed to write file %s: %s", mpiCfg.defFile, err)
	}

	return nil
}

func updateIntelTemplate(filepath string, destMPIInstall string) error {
	data, err := ioutil.ReadFile(filepath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %s", filepath, err)
	}
	content := string(data)
	content = strings.Replace(content, "MPIINSTALLDIR", destMPIInstall, -1)
	err = ioutil.WriteFile(filepath, []byte(content), 0)
	if err != nil {
		return fmt.Errorf("failed to write file %s: %s", filepath, err)
	}
	return nil
}

// This function updated the Intel install/uninstall scripts for the installation on the host
func updateIntelTemplates(mpiCfg *mpiConfig, sysCfg *SysConfig) error {
	// Sanity checks
	if mpiCfg.srcDir == "" || mpiCfg.buildDir == "" {
		return fmt.Errorf("Invalid parameter(s)")
	}

	intelSilentInstallConfig := filepath.Join(mpiCfg.srcDir, intelInstallConfFile)
	intelSilentUninstallConfig := filepath.Join(mpiCfg.srcDir, intelUninstallConfFile)

	err := updateIntelTemplate(intelSilentInstallConfig, mpiCfg.buildDir)
	if err != nil {
		return fmt.Errorf("failed to update template %s: %s", intelSilentInstallConfig, err)
	}

	err = updateIntelTemplate(intelSilentUninstallConfig, mpiCfg.buildDir)
	if err != nil {
		return fmt.Errorf("failed to update template %s: %s", intelSilentUninstallConfig, err)
	}

	return nil
}

func generateDefFile(myCfg *mpiConfig, sysCfg *SysConfig) error {
	log.Println("- Generating Singularity defintion file...")
	// Sanity checks
	if myCfg.buildDir == "" {
		return fmt.Errorf("invalid parameter(s)")
	}

	var defFileName string
	var templateFileName string
	switch myCfg.mpiImplm {
	case "openmpi":
		if sysCfg.NetPipe {
			defFileName = "ubuntu_ompi_netpipe.def"
		} else {
			defFileName = "ubuntu_ompi.def"
		}

	case "mpich":
		if sysCfg.NetPipe {
			defFileName = "ubuntu_mpich_netpipe.def"
		} else {
			defFileName = "ubuntu_mpich.def"
		}
	case "intel":
		if sysCfg.NetPipe {
			defFileName = "ubuntu_intel_netpipe.def"
		} else {
			defFileName = "ubuntu_intel.def"
		}
	default:
		return fmt.Errorf("unsupported MPI implementation: %s", myCfg.mpiImplm)
	}

	templateFileName = defFileName + ".tmpl"

	templateDefFile := filepath.Join(sysCfg.TemplateDir, templateFileName)
	myCfg.defFile = filepath.Join(myCfg.buildDir, defFileName)

	// Copy the definition file template to the temporary directory
	err := copyFile(templateDefFile, myCfg.defFile)
	if err != nil {
		return fmt.Errorf("Failed to copy %s to %s: %s", templateDefFile, myCfg.defFile, err)
	}

	// Copy the test file
	testFile := filepath.Join(sysCfg.TemplateDir, "mpitest.c")
	destTestFile := filepath.Join(myCfg.buildDir, "mpitest.c")
	err = copyFile(testFile, destTestFile)
	if err != nil {
		return fmt.Errorf("Failed to copy %s to %s: %s", testFile, destTestFile, err)
	}

	// Update the definition file for the specific version of MPI we are testing
	switch myCfg.mpiImplm {
	case "openmpi":
		err := updateOMPIDefFile(myCfg, sysCfg)
		if err != nil {
			return fmt.Errorf("failed to update OMPI template: %s", err)
		}
	case "mpich":
		err := updateMPICHDefFile(myCfg, sysCfg)
		if err != nil {
			return fmt.Errorf("failed to update MPICH template: %s", err)
		}
	case "intel":
		err := updateIntelMPIDefFile(myCfg, sysCfg)
		if err != nil {
			return fmt.Errorf("failed to update IMPI template: %s", err)
		}
	default:
		return fmt.Errorf("unsupported MPI implementation: %s", myCfg.mpiImplm)
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
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, sudoBin, sysCfg.SingularityBin, "build", imgName, myCfg.defFile)
	cmd.Dir = myCfg.buildDir
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to execute command - stdout: %s; stderr: %s; err: %s", stdout.String(), stderr.String(), err)
	}

	if sysCfg.NetPipe {
		myCfg.testPath = filepath.Join("/", "opt", "NetPIPE-5.1.4", "NPmpi")
	} else {
		myCfg.testPath = filepath.Join("/", "opt", "mpitest")
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
