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

	"github.com/sylabs/singularity-mpi/internal/pkg/jm"
	"github.com/sylabs/singularity-mpi/internal/pkg/util/sy"

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

func pullContainerImage(myContainerMPICfg *mpi.Config, exp mpi.Experiment, sysCfg *sys.Config, syConfig *sy.MPIToolConfig) error {
	// Sanity checks
	if myContainerMPICfg.URL == "" {
		return fmt.Errorf("undefined image URL")
	}

	if myContainerMPICfg.ImageURL == "" {
		return fmt.Errorf("undefined image URL")
	}

	if sysCfg.SingularityBin == "" {
		var err error
		sysCfg.SingularityBin, err = exec.LookPath("singularity")
		if err != nil {
			return fmt.Errorf("failed to find Singularity binary: %s", err)
		}
	}

	err := sy.Pull(myContainerMPICfg, sysCfg)
	if err != nil {
		return fmt.Errorf("failed to pull image: %s", err)
	}

	log.Println("* Container MPI configuration *")
	log.Println("-> Build container in", myContainerMPICfg.BuildDir)
	log.Println("-> MPI implementation:", myContainerMPICfg.MpiImplm)
	log.Println("-> MPI version:", myContainerMPICfg.MpiVersion)
	log.Println("-> Image URL:", myContainerMPICfg.URL)

	return nil
}

func createNewContainer(myContainerMPICfg *mpi.Config, exp mpi.Experiment, sysCfg *sys.Config, syConfig *sy.MPIToolConfig) error {
	/* CREATE THE CONTAINER MPI CONFIGURATION */
	if sysCfg.Persistent != "" && util.FileExists(myContainerMPICfg.ContainerPath) {
		log.Printf("* %s already exists, skipping...\n", myContainerMPICfg.ContainerPath)
		return nil
	}

	/* CREATE THE MPI CONTAINER */

	res := createMPIContainer(myContainerMPICfg, sysCfg)
	if res.Err != nil {
		_ = saveErrorDetails(exp, sysCfg, res)
		return fmt.Errorf("failed to create container: %s", res.Err)
	}

	return nil
}

// Run configure, install and execute a given experiment
func Run(exp mpi.Experiment, sysCfg *sys.Config, syConfig *sy.MPIToolConfig) (bool, string, error) {
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

	hostInstallDirPrefix := "mpi_install_" + exp.VersionHostMPI
	if sysCfg.Persistent == "" {
		// Create a temporary directory where to install MPI
		myHostMPICfg.InstallDir, err = ioutil.TempDir("", hostInstallDirPrefix+"-")
		if err != nil {
			return false, "", fmt.Errorf("failed to create install directory: %s", err)
		}
		defer os.RemoveAll(myHostMPICfg.InstallDir)
	} else {
		myHostMPICfg.InstallDir = filepath.Join(sysCfg.Persistent, "mpi_install_"+exp.VersionHostMPI)
	}

	myHostMPICfg.MpiImplm = exp.MPIImplm
	myHostMPICfg.URL = exp.URLHostMPI
	myHostMPICfg.MpiVersion = exp.VersionHostMPI

	log.Println("* Host MPI Configuration *")
	log.Println("-> Building MPI in", myHostMPICfg.BuildDir)
	log.Println("-> Installing MPI in", myHostMPICfg.InstallDir)
	log.Println("-> MPI implementation:", myHostMPICfg.MpiImplm)
	log.Println("-> MPI version:", myHostMPICfg.MpiVersion)
	log.Println("-> MPI URL:", myHostMPICfg.URL)

	/* INSTALL MPI ON THE HOST */

	res := mpi.InstallHost(&myHostMPICfg, sysCfg)
	if res.Err != nil {
		_ = saveErrorDetails(exp, sysCfg, res)
		return false, "", fmt.Errorf("failed to install host MPI: %s", res.Err)
	}
	if sysCfg.Persistent != "" {
		defer func() {
			res = mpi.UninstallHost(&myHostMPICfg, sysCfg)
			if res.Err != nil {
				log.Fatal(err)
			}
		}()
	}

	// Create a temporary directory where the container will be built
	myContainerMPICfg.URL = exp.URLContainerMPI
	log.Println("* Container MPI configuration *")
	log.Println("-> Build container in", myContainerMPICfg.BuildDir)
	log.Println("-> MPI implementation:", myContainerMPICfg.MpiImplm)
	log.Println("-> MPI version:", myContainerMPICfg.MpiVersion)
	log.Println("-> MPI URL:", myContainerMPICfg.URL)
	containerInstallDir := "mpi_container_" + exp.VersionContainerMPI
	if sysCfg.Persistent == "" {
		myContainerMPICfg.BuildDir, err = ioutil.TempDir("", containerInstallDir+"-")
		if err != nil {
			return false, "", fmt.Errorf("failed to create directory to build container: %s", err)
		}
		defer os.RemoveAll(myContainerMPICfg.BuildDir)
	} else {
		myContainerMPICfg.BuildDir = filepath.Join(sysCfg.Persistent, containerInstallDir)
		myContainerMPICfg.InstallDir = myContainerMPICfg.BuildDir
		if !util.PathExists(myContainerMPICfg.BuildDir) {
			err := os.MkdirAll(myContainerMPICfg.BuildDir, 0755)
			if err != nil {
				return false, "", fmt.Errorf("failed to create %s: %s", myContainerMPICfg.BuildDir, err)
			}
		}
	}
	myContainerMPICfg.ContainerName = "singularity_mpi.sif"
	myContainerMPICfg.ContainerPath = filepath.Join(myContainerMPICfg.BuildDir, myContainerMPICfg.ContainerName)
	myContainerMPICfg.MpiImplm = exp.MPIImplm
	myContainerMPICfg.MpiVersion = exp.VersionContainerMPI
	myContainerMPICfg.ImageURL = sy.GetImageURL(&myContainerMPICfg, sysCfg)
	getAppData(&myContainerMPICfg, sysCfg)

	// Pull or build the image
	if syConfig.BuildPrivilege {
		err = createNewContainer(&myContainerMPICfg, exp, sysCfg, syConfig)
		if err != nil {
			return false, "", fmt.Errorf("failed to create container: %s", err)
		}
	} else {
		err = pullContainerImage(&myContainerMPICfg, exp, sysCfg, syConfig)
		if err != nil {
			return false, "", fmt.Errorf("failed to pull container: %s", err)
		}
	}

	/* PREPARE THE COMMAND TO RUN THE ACTUAL TEST */

	log.Println("Running Test(s)...")

	// Regex to catch errors where mpirun returns 0 but is known to have failed because displaying the help message
	var re = regexp.MustCompile(`^(\n?)Usage:`)

	// mpiJob describes the job
	var mpiJob jm.Job
	mpiJob.HostCfg = &myHostMPICfg
	mpiJob.ContainerCfg = &myContainerMPICfg
	mpiJob.AppBin = myContainerMPICfg.TestPath
	mpiJob.NNodes = 2
	mpiJob.NP = 2

	// We submit the job
	submitCmd, err := jm.PrepareLaunchCmd(&mpiJob, sysCfg)
	var stdout, stderr bytes.Buffer
	submitCmd.Cmd.Stdout = &stdout
	submitCmd.Cmd.Stderr = &stderr
	if err != nil {
		return false, "", fmt.Errorf("failed to prepare the launch command: %s", err)
	}
	defer submitCmd.CancelFn()
	err = submitCmd.Cmd.Run()
	if err != nil || submitCmd.Ctx.Err() == context.DeadlineExceeded || re.Match(stdout.Bytes()) {
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

func getAppData(mpiCfg *mpi.Config, sysCfg *sys.Config) {
	mpiCfg.TestPath = filepath.Join("/", "opt", "mpitest")
	if sysCfg.NetPipe {
		mpiCfg.TestPath = filepath.Join("/", "opt", "NetPIPE-5.1.4", "NPmpi")
	}
	if sysCfg.IMB {
		mpiCfg.TestPath = filepath.Join("/", "opt", "mpi-benchmarks", "IMB-MPI1")
	}
}

func createContainerImage(mpiCfg *mpi.Config, sysCfg *sys.Config) error {
	err := mpi.CreateContainer(mpiCfg, sysCfg)
	if err != nil {
		return fmt.Errorf("failed to create container image: %s", err)
	}

	getAppData(mpiCfg, sysCfg)

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
