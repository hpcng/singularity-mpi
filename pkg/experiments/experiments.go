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
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sylabs/singularity-mpi/internal/pkg/launcher"

	"github.com/sylabs/singularity-mpi/internal/pkg/app"
	"github.com/sylabs/singularity-mpi/internal/pkg/buildenv"
	"github.com/sylabs/singularity-mpi/internal/pkg/builder"
	"github.com/sylabs/singularity-mpi/internal/pkg/container"
	"github.com/sylabs/singularity-mpi/internal/pkg/implem"
	"github.com/sylabs/singularity-mpi/internal/pkg/jm"
	"github.com/sylabs/singularity-mpi/internal/pkg/job"
	"github.com/sylabs/singularity-mpi/internal/pkg/mpi"
	"github.com/sylabs/singularity-mpi/internal/pkg/results"
	"github.com/sylabs/singularity-mpi/internal/pkg/syexec"
	"github.com/sylabs/singularity-mpi/internal/pkg/sys"
	util "github.com/sylabs/singularity-mpi/internal/pkg/util/file"
	"github.com/sylabs/singularity-mpi/internal/pkg/util/sy"
)

// Config is a structure that represents the configuration of an experiment
type Config struct {
	HostMPI      implem.Info
	ContainerMPI implem.Info
	Container    container.Config
	BuildEnv     buildenv.Info
	App          app.Info
	Result       results.Result
	/*
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
	*/
}

func postExecutionDataMgt(sysCfg *sys.Config, output string) (string, error) {
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
func GetMPIImplemFromExperiments(experiments []Config) string {
	// Fair assumption: all experiments are based on the same MPI
	// implementation (we actually check for that and the implementation
	// is only included in the experiment structure so that the structure
	// is self-contained).
	if len(experiments) == 0 {
		return ""
	}

	return experiments[0].HostMPI.ID
}

func saveErrorDetails(exp Config, sysCfg *sys.Config, res *syexec.Result) error {
	experimentName := exp.HostMPI.Version + "-" + exp.ContainerMPI.Version
	targetDir := filepath.Join(sysCfg.BinPath, "errors", exp.HostMPI.ID, experimentName)

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

func createNewContainer(myContainerMPICfg *mpi.Config, exp Config, sysCfg *sys.Config, syConfig *sy.MPIToolConfig) syexec.Result {
	var res syexec.Result

	/* CREATE THE CONTAINER MPI CONFIGURATION */
	if sysCfg.Persistent != "" && util.FileExists(myContainerMPICfg.Container.Path) {
		log.Printf("* %s already exists, skipping...\n", myContainerMPICfg.Container.Path)
		return res
	}

	/* CREATE THE MPI CONTAINER */

	res = createMPIContainer(myContainerMPICfg, &exp.BuildEnv, sysCfg)
	if res.Err != nil {
		err := saveErrorDetails(exp, sysCfg, &res)
		if err != nil {
			res.Err = fmt.Errorf("failed to save error details: %s", err)
			return res
		}
		res.Err = fmt.Errorf("failed to create container: %s", res.Err)
		return res
	}

	return res
}

// Run configure, install and execute a given experiment
func Run(exp Config, sysCfg *sys.Config, syConfig *sy.MPIToolConfig) (bool, results.Result, syexec.Result) {
	var myHostMPICfg mpi.Config
	var myContainerMPICfg mpi.Config
	var execRes syexec.Result
	var expRes results.Result
	var err error

	/* CREATE THE HOST MPI CONFIGURATION */

	// Create a temporary directory where to compile MPI
	myHostMPICfg.Buildenv.BuildDir, err = ioutil.TempDir("", "mpi_build_"+exp.HostMPI.Version+"-")
	if err != nil {
		execRes.Err = fmt.Errorf("failed to create compile directory: %s", err)
		expRes.Pass = false
		return false, expRes, execRes
	}
	defer os.RemoveAll(myHostMPICfg.Buildenv.BuildDir)

	hostInstallDirPrefix := "mpi_install_" + exp.HostMPI.Version
	if sysCfg.Persistent == "" {
		// Create a temporary directory where to install MPI
		myHostMPICfg.Buildenv.InstallDir, err = ioutil.TempDir("", hostInstallDirPrefix+"-")
		if err != nil {
			execRes.Err = fmt.Errorf("failed to create install directory: %s", err)
			expRes.Pass = false
			return false, expRes, execRes
		}
		defer os.RemoveAll(myHostMPICfg.Buildenv.InstallDir)
	} else {
		myHostMPICfg.Buildenv.InstallDir = filepath.Join(sysCfg.Persistent, "mpi_install_"+exp.HostMPI.Version)
	}

	myHostMPICfg.Implem.ID = exp.HostMPI.ID
	myHostMPICfg.Implem.URL = exp.HostMPI.URL
	myHostMPICfg.Implem.Version = exp.HostMPI.Version
	exp.BuildEnv.BuildDir = myHostMPICfg.Buildenv.BuildDir
	exp.BuildEnv.InstallDir = myHostMPICfg.Buildenv.InstallDir

	log.Println("* Host MPI Configuration *")
	log.Println("-> Building MPI in", myHostMPICfg.Buildenv.BuildDir)
	log.Println("-> Installing MPI in", myHostMPICfg.Buildenv.InstallDir)
	log.Println("-> MPI implementation:", myHostMPICfg.Implem.ID)
	log.Println("-> MPI version:", myHostMPICfg.Implem.Version)
	log.Println("-> MPI URL:", myHostMPICfg.Implem.URL)

	/* INSTALL MPI ON THE HOST */

	jobmgr := jm.Detect()
	b, err := builder.Load(&myHostMPICfg.Implem)
	if err != nil {
		execRes.Err = fmt.Errorf("unable to load a builder: %s", err)
		return false, expRes, execRes
	}

	execRes = b.InstallHost(&myHostMPICfg.Implem, &jobmgr, &myHostMPICfg.Buildenv, sysCfg)
	if execRes.Err != nil {
		execRes.Err = fmt.Errorf("failed to install host MPI: %s", execRes.Err)
		err = saveErrorDetails(exp, sysCfg, &execRes)
		if err != nil {
			execRes.Err = fmt.Errorf("failed to save error details: %s", err)
		}
		expRes.Pass = false
		return false, expRes, execRes
	}
	if sysCfg.Persistent != "" {
		defer func() {
			execRes = b.UninstallHost(&myHostMPICfg.Implem, &myHostMPICfg.Buildenv, sysCfg)
			if execRes.Err != nil {
				log.Fatalf("failed to uninstall MPI: %s", err)
			}
		}()
	}

	// Create a temporary directory where the container will be built
	myContainerMPICfg.Implem.URL = exp.ContainerMPI.URL
	containerInstallDir := "mpi_container_" + exp.ContainerMPI.Version
	if sysCfg.Persistent == "" {
		myContainerMPICfg.Buildenv.BuildDir, err = ioutil.TempDir("", containerInstallDir+"-")
		//myContainerMPICfg.Buildenv.InstallDir = myContainerMPICfg.Buildenv.BuildDir
		if err != nil {
			execRes.Err = fmt.Errorf("failed to create directory to build container: %s", err)
			expRes.Pass = false
			return false, expRes, execRes
		}
		defer os.RemoveAll(myContainerMPICfg.Buildenv.BuildDir)
	} else {
		myContainerMPICfg.Buildenv.BuildDir = filepath.Join(sysCfg.Persistent, containerInstallDir)
		myContainerMPICfg.Buildenv.InstallDir = myContainerMPICfg.Buildenv.BuildDir
		if !util.PathExists(myContainerMPICfg.Buildenv.BuildDir) {
			err := os.MkdirAll(myContainerMPICfg.Buildenv.BuildDir, 0755)
			if err != nil {
				execRes.Err = fmt.Errorf("failed to create %s: %s", myContainerMPICfg.Buildenv.BuildDir, err)
				expRes.Pass = false
				return false, expRes, execRes
			}
		}
	}
	myContainerMPICfg.Container.Name = exp.ContainerMPI.ID + "-" + exp.ContainerMPI.Version + ".sif"
	myContainerMPICfg.Container.Path = filepath.Join(myContainerMPICfg.Buildenv.BuildDir, myContainerMPICfg.Container.Name)
	myContainerMPICfg.Container.BuildDir = myContainerMPICfg.Buildenv.BuildDir
	myContainerMPICfg.Implem.ID = exp.ContainerMPI.ID
	myContainerMPICfg.Implem.Version = exp.ContainerMPI.Version
	myContainerMPICfg.Container.URL = sy.GetImageURL(&myContainerMPICfg.Implem, sysCfg)
	exp.App.BinPath = getAppData(&myContainerMPICfg, sysCfg)
	log.Println("* Container MPI configuration *")
	log.Println("-> Build container in", exp.BuildEnv.BuildDir)
	log.Println("-> MPI implementation:", myContainerMPICfg.Implem.ID)
	log.Println("-> MPI version:", myContainerMPICfg.Implem.Version)
	log.Println("-> MPI URL:", myContainerMPICfg.Implem.URL)

	// Pull or build the image
	if syConfig.BuildPrivilege {
		execRes = createNewContainer(&myContainerMPICfg, exp, sysCfg, syConfig)
		if execRes.Err != nil {
			execRes.Err = fmt.Errorf("failed to create container: %s", err)
			expRes.Pass = false
			return false, expRes, execRes
		}
	} else {
		err = container.PullContainerImage(&myContainerMPICfg.Container, &myContainerMPICfg.Implem, sysCfg, syConfig)
		if err != nil {
			execRes.Err = fmt.Errorf("failed to pull container: %s", err)
			expRes.Pass = false
			return false, expRes, execRes
		}
	}

	/* PREPARE THE COMMAND TO RUN THE ACTUAL TEST */

	log.Println("Running Test(s)...")

	// Regex to catch errors where mpirun returns 0 but is known to have failed because displaying the help message
	var re = regexp.MustCompile(`^(\n?)Usage:`)

	// mpiJob describes the job
	var mpiJob job.Job
	mpiJob.HostCfg = &myHostMPICfg.Implem
	mpiJob.Container = &myContainerMPICfg.Container
	mpiJob.App.BinPath = exp.App.BinPath
	mpiJob.NNodes = 2
	mpiJob.NP = 2

	// We submit the job
	var submitCmd syexec.SyCmd
	submitCmd, execRes.Err = launcher.PrepareLaunchCmd(&mpiJob, &jobmgr, &exp.BuildEnv, sysCfg)
	if execRes.Err != nil {
		execRes.Err = fmt.Errorf("failed to prepare the launch command: %s", execRes.Err)
		return false, expRes, execRes
	}

	var stdout, stderr bytes.Buffer
	submitCmd.Cmd.Stdout = &stdout
	submitCmd.Cmd.Stderr = &stderr
	if err != nil {
		execRes.Err = fmt.Errorf("failed to prepare the launch command: %s", err)
		expRes.Pass = false
		return false, expRes, execRes
	}
	defer submitCmd.CancelFn()
	err = submitCmd.Cmd.Run()
	// Get the command out/err
	execRes.Stderr = stderr.String()
	execRes.Stdout = stdout.String()
	// And add the job out/err (for when we actually use a real job manager such as Slurm)
	execRes.Stdout += mpiJob.GetOutput(&mpiJob, sysCfg)
	execRes.Stderr += mpiJob.GetError(&mpiJob, sysCfg)
	if err != nil || submitCmd.Ctx.Err() == context.DeadlineExceeded || re.Match(stdout.Bytes()) {
		log.Printf("[INFO] mpirun command failed - stdout: %s - stderr: %s - err: %s\n", stdout.String(), stderr.String(), err)
		execRes.Err = err
		err = saveErrorDetails(exp, sysCfg, &execRes)
		if err != nil {
			execRes.Err = fmt.Errorf("impossible to cleanly handle error: %s", err)
			expRes.Pass = false
			return false, expRes, execRes
		}
		expRes.Pass = false
		return false, expRes, execRes
	}

	log.Printf("Successful run - stdout: %s; stderr: %s\n", execRes.Stdout, execRes.Stderr)

	log.Println("Handling data...")
	expRes.Note, err = postExecutionDataMgt(sysCfg, execRes.Stdout)
	if err != nil {
		execRes.Err = fmt.Errorf("failed to handle data: %s", err)
		expRes.Pass = false
		return false, expRes, execRes
	}

	log.Println("NOTE: ", expRes.Note)

	expRes.Pass = true
	return false, expRes, execRes
}

func getAppData(mpiCfg *mpi.Config, sysCfg *sys.Config) string {
	path := filepath.Join("/", "opt", "mpitest")
	if sysCfg.NetPipe {
		path = filepath.Join("/", "opt", "NetPIPE-5.1.4", "NPmpi")
	}
	if sysCfg.IMB {
		path = filepath.Join("/", "opt", "mpi-benchmarks", "IMB-MPI1")
	}

	return path
}

func createContainerImage(mpiCfg *mpi.Config, buildEnv *buildenv.Info, sysCfg *sys.Config) error {
	var containerCfg container.Config
	containerCfg.BuildDir = buildEnv.BuildDir
	containerCfg.InstallDir = buildEnv.InstallDir
	err := container.Create(&containerCfg, sysCfg)
	if err != nil {
		return fmt.Errorf("failed to create container image: %s", err)
	}

	//getAppData(mpiCfg, sysCfg)

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
func createMPIContainer(mpiCfg *mpi.Config, env *buildenv.Info, sysCfg *sys.Config) syexec.Result {
	var res syexec.Result
	var b builder.Builder

	containerCfg := &mpiCfg.Container

	b, res.Err = builder.Load(&mpiCfg.Implem)
	if res.Err != nil {
		return res
	}

	log.Println("Creating MPI container...")
	res.Err = b.GenerateDeffile(&mpiCfg.Implem, env, containerCfg, sysCfg)
	if res.Err != nil {
		res.Stderr = fmt.Sprintf("failed to generate Singularity definition file: %s", res.Err)
	}

	res.Err = createContainerImage(mpiCfg, env, sysCfg)
	if res.Err != nil {
		res.Stderr = fmt.Sprintf("failed to create container image: %s", res.Err)
	}

	return res
}

// Pruning removes the experiments for which we already have results
func Pruning(experiments []Config, existingResults []results.Result) []Config {
	// No optimization at the moment, double loop and creation of a new array
	var experimentsToRun []Config
	//	for j := 0; j < len(experiments); j++ {
	for _, experiment := range experiments {
		found := false
		for _, result := range existingResults {
			if experiment.HostMPI.Version == result.HostMPI.Version && experiment.ContainerMPI.Version == result.ContainerMPI.Version {
				log.Printf("We already have results for %s on the host and %s in a container, skipping...\n", experiment.HostMPI.Version, experiment.ContainerMPI.Version)
				found = true
				break
			}
		}
		if !found {
			// No associated results
			experimentsToRun = append(experimentsToRun, experiment)
		}
	}

	return experimentsToRun
}
