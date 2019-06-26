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
	TemplateDir    string // Where the template are
	SedBin         string // Path to the sed binary
	SingularityBin string // Path to the singularity binary
	OutputFile     string // Path the output file
	NetPipe        bool   // Execute NetPipe as test
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

func downloadMPI(mpiCfg *mpiConfig) error {
	fmt.Println("- Downloading MPI...")

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

const (
	formatBZ2 = "bz2"
	formatGZ  = "gz"
)

func detectTarballFormat(filepath string) string {
	if path.Ext(filepath) == ".bz2" {
		return formatBZ2
	}

	if path.Ext(filepath) == ".gz" {
		return formatGZ
	}

	return ""
}

func unpackMPI(mpiCfg *mpiConfig) error {
	fmt.Println("- Unpacking MPI...")

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
	fmt.Println("- Configuring MPI...")

	// Some sanity checks
	if mpiCfg.srcDir == "" || mpiCfg.installDir == "" {
		return fmt.Errorf("invalid parameter(s)")
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
	fmt.Println("- Compiling MPI...")

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

	fmt.Println("* Host MPI Configuration *")
	fmt.Println("-> Building MPI in", myHostMPICfg.buildDir)
	fmt.Println("-> Installing MPI in", myHostMPICfg.installDir)
	fmt.Println("-> MPI implementation:", myHostMPICfg.mpiImplm)
	fmt.Println("-> MPI version:", myHostMPICfg.mpiVersion)
	fmt.Println("-> MPI URL:", myHostMPICfg.url)

	/* CREATE THE CONTAINER MPI CONFIGURATION */

	// Cretae a temporary directory where the container will be built
	myContainerMPICfg.buildDir, err = ioutil.TempDir("", "mpi_container_"+exp.VersionContainerMPI+"-")
	if err != nil {
		return false, "", fmt.Errorf("failed to create directory to build container: %s", err)
	}
	defer os.RemoveAll(myContainerMPICfg.buildDir)

	myContainerMPICfg.mpiImplm = exp.MPIImplm
	myContainerMPICfg.url = exp.URLContainerMPI
	myContainerMPICfg.mpiVersion = exp.VersionContainerMPI

	fmt.Println("* Container MPI configuration *")
	fmt.Println("-> Build container in", myContainerMPICfg.buildDir)
	fmt.Println("-> MPI implementation:", myContainerMPICfg.mpiImplm)
	fmt.Println("-> MPI version:", myContainerMPICfg.mpiVersion)
	fmt.Println("-> MPI URL:", myContainerMPICfg.url)

	err = installHostMPI(&myHostMPICfg)
	if err != nil {
		return false, "", fmt.Errorf("failed to install host MPI: %s", err)
	}

	err = createMPIContainer(&myContainerMPICfg, sysCfg)
	if err != nil {
		return false, "", fmt.Errorf("failed to create container: %s", err)
	}

	/* PREPARE THE COMMAND TO RUN THE ACTUAL TEST */

	fmt.Println("Running Test(s)...")
	// We only let the mpirun command run for 10 minutes max
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	var stdout, stderr bytes.Buffer
	newPath := myHostMPICfg.installDir + "/bin:" + os.Getenv("PATH")
	newLDPath := myHostMPICfg.installDir + "/lib:" + os.Getenv("LD_LIBRARY_PATH")

	mpirunBin := filepath.Join(myHostMPICfg.installDir, "bin", "mpirun")
	mpiCmd := exec.CommandContext(ctx, mpirunBin, "-np", "2", "singularity", "exec", myContainerMPICfg.containerPath, myContainerMPICfg.testPath)
	mpiCmd.Env = append([]string{"LD_LIBRARY_PATH=" + newLDPath}, os.Environ()...)
	mpiCmd.Env = append([]string{"PATH=" + newPath}, os.Environ()...)
	mpiCmd.Stdout = &stdout
	mpiCmd.Stderr = &stderr
	err = mpiCmd.Run()
	if err != nil || ctx.Err() == context.DeadlineExceeded {
		fmt.Printf("[INFO] mpirun command failed - stdout: %s - stderr: %s - err: %s\n", stdout.String(), stderr.String(), err)
		return false, "", nil
	}

	fmt.Printf("Successful run - stdout: %s; stderr: %s\n", stdout.String(), stderr.String())

	fmt.Println("Handling data...")
	note, err := postExecutionDataMgt(exp, sysCfg, stdout.String())
	if err != nil {
		return true, "", fmt.Errorf("failed to handle data: %s", err)
	}

	fmt.Println("NOTE: ", note)

	return true, note, nil
}

func installHostMPI(myCfg *mpiConfig) error {
	fmt.Println("Installing MPI on host...")
	err := downloadMPI(myCfg)
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

	err = compileMPI(myCfg)
	if err != nil {
		return fmt.Errorf("failed to compile MPI")
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

	if sysCfg.SedBin == "" {
		sysCfg.SedBin, err = exec.LookPath("sed")
		if err != nil {
			return fmt.Errorf("failed to find path for sed: %s", err)
		}
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
	compileCfg.mpiURLTag = "OMPIURL"
	compileCfg.mpiTarballTag = "OMPITARBALL"

	err := doUpdateDefFile(myCfg, sysCfg, &compileCfg)
	if err != nil {
		return fmt.Errorf("failed to update Open MPI definition file")
	}

	return nil
}

func generateDefFile(myCfg *mpiConfig, sysCfg *SysConfig) error {
	fmt.Println("- Generating Singularity defintion file...")
	// Sanity checks
	if myCfg.buildDir == "" {
		return fmt.Errorf("invalid parameter(s)")
	}

	var defFileName string
	var templateFileName string
	switch myCfg.mpiImplm {
	case "openmpi":
		if sysCfg.NetPipe {
			templateFileName = "ubuntu_ompi_netpipe.def.tmpl"
			defFileName = "ubuntu_ompi_netpipe.def"
		} else {
			templateFileName = "ubuntu_ompi.def.tmpl"
			defFileName = "ubuntu_ompi.def"
		}
	case "mpich":
		if sysCfg.NetPipe {
			templateFileName = "ubuntu_mpich_netpipe.def.tmpl"
			defFileName = "ubuntu_mpich_netpipe.def"
		} else {
			templateFileName = "ubuntu_mpich.def.tmpl"
			defFileName = "ubuntu_mpich.def"
		}
	default:
		return fmt.Errorf("unsupported MPI implementation: %s", myCfg.mpiImplm)
	}
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
	default:
		return fmt.Errorf("unsupported MPI implementation: %s", myCfg.mpiImplm)
	}

	return nil
}

func createContainerImage(myCfg *mpiConfig, sysCfg *SysConfig) error {
	var err error

	fmt.Println("- Creating image...")
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

	// The definition file is ready so we simple build the container using the Singularity command
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(sudoBin, sysCfg.SingularityBin, "build", imgName, myCfg.defFile)
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
	fmt.Println("Creating MPI container...")
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
