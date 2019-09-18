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
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/sylabs/singularity-mpi/internal/pkg/mpi"
	"github.com/sylabs/singularity-mpi/internal/pkg/sys"
	util "github.com/sylabs/singularity-mpi/internal/pkg/util/file"
)

func postExecutionDataMgt(exp mpi.Experiment, sysCfg *sys.Config, output string) (string, error) {
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

// GetImplemFromExperiments returns the MPI implementation that is associated
// to the experiments
func GetMPIImplemFromExperiments(experiments []mpi.Experiment) string {
	// Fair assumption: all experiments are based on the same MPI
	// implementation (we actually check for that and the implementation
	// is only included in the experiment structure so that the structure
	// is self-contained).
	if len(experiments) == 0 {
		return ""
	}

	return experiments[0].MPIImplm
}

func getEnvPath(mpiCfg *mpi.Config) string {
	// Intel MPI is installing the binaries and libraries in a quite complex setup
	if mpiCfg.MpiImplm == "intel" {
		return filepath.Join(mpiCfg.InstallDir, mpi.IntelInstallPathPrefix, "bin") + ":" + os.Getenv("PATH")
	}

	return filepath.Join(mpiCfg.InstallDir, "bin") + ":" + os.Getenv("PATH")
}

func getEnvLDPath(mpiCfg *mpi.Config) string {
	// Intel MPI is installing the binaries and libraries in a quite complex setup
	if mpiCfg.MpiImplm == "intel" {
		return filepath.Join(mpiCfg.InstallDir, mpi.IntelInstallPathPrefix, "lib") + ":" + os.Getenv("LD_LIBRARY_PATH")
	}

	return filepath.Join(mpiCfg.InstallDir, "lib") + ":" + os.Getenv("LD_LIBRARY_PATH")
}

func saveErrorDetails(exp mpi.Experiment, sysCfg *sys.Config, res mpi.ExecResult) error {
	experimentName := exp.VersionHostMPI + "-" + exp.VersionContainerMPI
	targetDir := filepath.Join(sysCfg.BinPath, "errors", exp.MPIImplm, experimentName)

	// If the directory exists, we delete it to start fresh
	err := util.DirInit(targetDir)
	if err != nil {
		return fmt.Errorf("impossible to initialize directory %s: %s", targetDir, err)
	}

	stderrFile := filepath.Join(targetDir, "stderr.txt")
	stdoutFile := filepath.Join(targetDir, "stdout.txt")

	fstderr, err := os.Create(stderrFile)
	if err != nil {
		return err
	}
	defer fstderr.Close()
	_, err = fstderr.WriteString(res.Stderr)
	if err != nil {
		return err
	}

	fstdout, err := os.Create(stdoutFile)
	if err != nil {
		return err
	}
	defer fstdout.Close()
	_, err = fstdout.WriteString(res.Stdout)
	if err != nil {
		return err
	}

	return nil
}

// Run configure, install and execute a given experiment
func Run(exp mpi.Experiment, sysCfg *sys.Config) (bool, string, error) {
	var myHostMPICfg mpi.Config
	var myContainerMPICfg mpi.Config
	var err error

	/* CREATE THE HOST MPI CONFIGURATION */

	// Create a temporary directory where to compile MPI
	myHostMPICfg.BuildDir, err = ioutil.TempDir("", "mpi_build_"+exp.VersionHostMPI+"-")
	if err != nil {
		return false, "", fmt.Errorf("failed to create compile directory: %s", err)
	}
	defer os.RemoveAll(myHostMPICfg.BuildDir)

	// Create a temporary directory where to install MPI
	myHostMPICfg.InstallDir, err = ioutil.TempDir("", "mpi_install_"+exp.VersionHostMPI+"-")
	if err != nil {
		return false, "", fmt.Errorf("failed to create install directory: %s", err)
	}
	defer os.RemoveAll(myHostMPICfg.InstallDir)

	myHostMPICfg.MpiImplm = exp.MPIImplm
	myHostMPICfg.URL = exp.URLHostMPI
	myHostMPICfg.MpiVersion = exp.VersionHostMPI

	log.Println("* Host MPI Configuration *")
	log.Println("-> Building MPI in", myHostMPICfg.BuildDir)
	log.Println("-> Installing MPI in", myHostMPICfg.InstallDir)
	log.Println("-> MPI implementation:", myHostMPICfg.MpiImplm)
	log.Println("-> MPI version:", myHostMPICfg.MpiVersion)
	log.Println("-> MPI URL:", myHostMPICfg.URL)

	/* CREATE THE CONTAINER MPI CONFIGURATION */

	// Create a temporary directory where the container will be built
	myContainerMPICfg.BuildDir, err = ioutil.TempDir("", "mpi_container_"+exp.VersionContainerMPI+"-")
	if err != nil {
		return false, "", fmt.Errorf("failed to create directory to build container: %s", err)
	}
	defer os.RemoveAll(myContainerMPICfg.BuildDir)

	myContainerMPICfg.MpiImplm = exp.MPIImplm
	myContainerMPICfg.URL = exp.URLContainerMPI
	myContainerMPICfg.MpiVersion = exp.VersionContainerMPI
	myContainerMPICfg.ContainerName = "singularity_mpi.sif"
	myContainerMPICfg.ContainerPath = filepath.Join(myContainerMPICfg.BuildDir, myContainerMPICfg.ContainerName)

	log.Println("* Container MPI configuration *")
	log.Println("-> Build container in", myContainerMPICfg.BuildDir)
	log.Println("-> MPI implementation:", myContainerMPICfg.MpiImplm)
	log.Println("-> MPI version:", myContainerMPICfg.MpiVersion)
	log.Println("-> MPI URL:", myContainerMPICfg.URL)

	/* INSTALL MPI ON THE HOST */

	res := mpi.InstallHost(&myHostMPICfg, sysCfg)
	if res.Err != nil {
		_ = saveErrorDetails(exp, sysCfg, res)
		return false, "", fmt.Errorf("failed to install host MPI: %s", res.Err)
	}
	defer func() {
		res = mpi.UninstallHost(&myHostMPICfg, sysCfg)
		if res.Err != nil {
			log.Fatal(err)
		}
	}()

	/* CREATE THE MPI CONTAINER */

	res = createMPIContainer(&myContainerMPICfg, sysCfg)
	if res.Err != nil {
		_ = saveErrorDetails(exp, sysCfg, res)
		return false, "", fmt.Errorf("failed to create container: %s", res.Err)
	}

	/* PREPARE THE COMMAND TO RUN THE ACTUAL TEST */

	log.Println("Running Test(s)...")
	// We only let the mpirun command run for 10 minutes max
	ctx, cancel := context.WithTimeout(context.Background(), sys.CmdTimeout*time.Minute)
	defer cancel()

	// Regex to catch errors where mpirun returns 0 but is known to have failed because displaying the help message
	var re = regexp.MustCompile(`^(\n?)Usage:`)

	var stdout, stderr bytes.Buffer
	newPath := getEnvPath(&myHostMPICfg)
	newLDPath := getEnvLDPath(&myHostMPICfg)

	mpirunBin := mpi.GetPathToMpirun(&myHostMPICfg)

	// We have to be careful: if we leave an empty argument in the slice, it may lead to a mpirun failure.
	var mpiCmd *exec.Cmd
	// We really do not want to do this but MPICH is being picky about args so for now, it will do the job.
	if myHostMPICfg.MpiImplm == "intel" {
		extraArgs := mpi.GetExtraMpirunArgs(&myHostMPICfg)

		args := []string{"-np", "2", "singularity", "exec", myContainerMPICfg.ContainerPath, myContainerMPICfg.TestPath}
		if len(extraArgs) > 0 {
			args = append(extraArgs, args...)
		}
		mpiCmd = exec.CommandContext(ctx, mpirunBin, args...)
		log.Printf("-> Running: %s %s", mpirunBin, strings.Join(args, " "))
	} else {
		mpiCmd = exec.CommandContext(ctx, mpirunBin, "-np", "2", "singularity", "exec", myContainerMPICfg.ContainerPath, myContainerMPICfg.TestPath)
		log.Printf("-> Running: %s %s", mpirunBin, strings.Join([]string{"-np", "2", "singularity", "exec", myContainerMPICfg.ContainerPath, myContainerMPICfg.TestPath}, " "))
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
		var res mpi.ExecResult
		res.Err = err
		res.Stderr = stderr.String()
		res.Stdout = stdout.String()
		err = saveErrorDetails(exp, sysCfg, res)
		return false, "", err
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

func createContainerImage(mpiCfg *mpi.Config, sysCfg *sys.Config) error {
	err := mpi.CreateContainer(mpiCfg, sysCfg)
	if err != nil {
		return fmt.Errorf("failed to create container image: %s", err)
	}

	mpiCfg.TestPath = filepath.Join("/", "opt", "mpitest")
	if sysCfg.NetPipe {
		mpiCfg.TestPath = filepath.Join("/", "opt", "NetPIPE-5.1.4", "NPmpi")
	}
	if sysCfg.IMB {
		mpiCfg.TestPath = filepath.Join("/", "opt", "mpi-benchmarks", "IMB-MPI1")
	}

	return nil
}

// GetOutputFilename returns the name of the file that is associated to the experiments
// to run
func GetOutputFilename(mpiImplem string, sysCfg *sys.Config) error {
	sysCfg.OutputFile = mpiImplem + "-init-results.txt"

	if sysCfg.NetPipe {
		sysCfg.OutputFile = mpiImplem + "-netpipe-results.txt"
	}

	if sysCfg.IMB {
		sysCfg.OutputFile = mpiImplem + "-imb-results.txt"
	}

	return nil
}

// createMPIContainer creates a container based on a specific configuration.
func createMPIContainer(myCfg *mpi.Config, sysCfg *sys.Config) mpi.ExecResult {
	var res mpi.ExecResult

	log.Println("Creating MPI container...")
	res.Err = mpi.GenerateDeffile(myCfg, sysCfg)
	if res.Err != nil {
		res.Stderr = fmt.Sprintf("failed to generate Singularity definition file: %s", res.Err)
	}

	res.Err = createContainerImage(myCfg, sysCfg)
	if res.Err != nil {
		res.Stderr = fmt.Sprintf("failed to create container image: %s", res.Err)
	}

	return res
}
