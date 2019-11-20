// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/sylabs/singularity-mpi/internal/pkg/app"
	"github.com/sylabs/singularity-mpi/internal/pkg/buildenv"
	"github.com/sylabs/singularity-mpi/internal/pkg/checker"
	"github.com/sylabs/singularity-mpi/internal/pkg/configparser"
	cfg "github.com/sylabs/singularity-mpi/internal/pkg/configparser"
	"github.com/sylabs/singularity-mpi/internal/pkg/container"
	"github.com/sylabs/singularity-mpi/internal/pkg/kv"
	"github.com/sylabs/singularity-mpi/internal/pkg/launcher"
	"github.com/sylabs/singularity-mpi/internal/pkg/results"
	"github.com/sylabs/singularity-mpi/internal/pkg/sy"
	"github.com/sylabs/singularity-mpi/internal/pkg/syexec"
	"github.com/sylabs/singularity-mpi/internal/pkg/sys"
	util "github.com/sylabs/singularity-mpi/internal/pkg/util/file"
	exp "github.com/sylabs/singularity-mpi/pkg/experiments"
)

func getListExperiments(config *configparser.Config) []exp.Config {
	var experiments []exp.Config
	for mpi1, mpi1url := range config.MpiMap {
		for mpi2, mpi2url := range config.MpiMap {
			var newExperiment exp.Config
			newExperiment.HostMPI.Version = mpi1
			newExperiment.HostMPI.URL = mpi1url
			newExperiment.HostMPI.ID = config.MPIImplem
			newExperiment.ContainerMPI.Version = mpi2
			newExperiment.ContainerMPI.URL = mpi2url
			newExperiment.ContainerMPI.ID = config.MPIImplem
			experiments = append(experiments, newExperiment)
		}
	}

	return experiments
}

func runExperiment(e exp.Config, sysCfg *sys.Config, syConfig *sy.MPIToolConfig) (results.Result, error) {
	var expRes results.Result
	var execRes syexec.Result

	expRes.HostMPI = e.HostMPI
	expRes.ContainerMPI = e.ContainerMPI
	expRes.Pass, expRes, execRes = exp.Run(e, sysCfg, syConfig)
	if execRes.Err != nil {
		return expRes, fmt.Errorf("failure during the execution of the experiment: %s", execRes.Err)
	}

	return expRes, nil
}

func createContainerEnvCfg(e *exp.Config, sysCfg *sys.Config) error {
	/* SET THE INSTALL DIRECTORY */

	containerName := container.GetContainerDefaultName(e.Container.Distro, e.ContainerMPI.ID, e.ContainerMPI.Version, e.App.Name, container.HybridModel)
	containerDirName := sys.ContainerInstallDirPrefix + containerName
	if sysCfg.Persistent == "" {
		e.ContainerBuildEnv.InstallDir = filepath.Join(sysCfg.ScratchDir)
		err := util.DirInit(e.ContainerBuildEnv.InstallDir)
		if err != nil {
			return fmt.Errorf("failed to initialize directory %s: %s", e.ContainerBuildEnv.ScratchDir, err)
		}
	} else {
		e.ContainerBuildEnv.InstallDir = filepath.Join(sysCfg.Persistent, containerDirName)
		if !util.PathExists(e.ContainerBuildEnv.InstallDir) {
			err := os.MkdirAll(e.ContainerBuildEnv.InstallDir, 0755)
			if err != nil {
				return fmt.Errorf("failed to create %s: %s", e.ContainerBuildEnv.InstallDir, err)
			}
		}

	}

	/* SET THE BUILD DIRECTORY */

	// At the moment, the build directory is also the install directory
	// todo: build the container in a build directory and copy only what we need to the install directory
	e.ContainerBuildEnv.BuildDir = e.ContainerBuildEnv.InstallDir

	/* SET THE SCRATCH DIRECTORY */

	e.ContainerBuildEnv.ScratchDir = filepath.Join(sysCfg.ScratchDir, "scratch_container_"+containerName)
	// We always initialize the scratch directory for MPI on the host
	err := util.DirInit(e.ContainerBuildEnv.ScratchDir)
	if err != nil {
		return fmt.Errorf("failed to initialize directory %s: %s", e.ContainerBuildEnv.ScratchDir, err)
	}

	return nil
}

func getAppData(sysCfg *sys.Config) app.Info {
	appInfo := app.GetHelloworld(sysCfg)
	if sysCfg.NetPipe {
		appInfo = app.GetNetpipe(sysCfg)
	}
	if sysCfg.IMB {
		appInfo = app.GetIMB(sysCfg)
	}

	return appInfo
}

func run(experiments []exp.Config, sysCfg *sys.Config, syConfig *sy.MPIToolConfig) []results.Result {
	var newResults []results.Result

	/* Sanity checks */
	if sysCfg == nil || sysCfg.OutputFile == "" {
		log.Fatalf("invalid parameter(s)")
	}

	f := util.OpenResultsFile(sysCfg.OutputFile)
	if f == nil {
		log.Fatalf("impossible to open result file %s", sysCfg.OutputFile)
	}
	defer f.Close()

	for _, e := range experiments {
		success := true
		failure := false
		var newRes results.Result
		var err error

		e.App = getAppData(sysCfg)
		e.Container.Distro = sysCfg.TargetDistro

		err = buildenv.CreateDefaultHostEnvCfg(&e.HostBuildEnv, &e.HostMPI, sysCfg)
		if err != nil {
			success = false
			failure = false
			log.Printf("[ERROR] failed to set host build environment: %s", err)
		}
		defer func() {
			os.RemoveAll(e.HostBuildEnv.ScratchDir)
			os.RemoveAll(e.HostBuildEnv.BuildDir)
		}()

		err = createContainerEnvCfg(&e, sysCfg)
		if err != nil {
			success = false
			failure = false
			log.Printf("[ERROR] failed to set container build environment: %s", err)
		}

		var i int
		for i = 0; i < sysCfg.Nrun; i++ {
			log.Printf("Running experiment %d/%d with host MPI %s and container MPI %s\n", i+1, sysCfg.Nrun, e.HostMPI.Version, e.ContainerMPI.Version)
			newRes, err = runExperiment(e, sysCfg, syConfig)
			if err != nil {
				log.Fatalf("failure during the execution of experiment: %s", err)
			}
			newResults = append(newResults, newRes)

			if err != nil {
				success = false
				failure = false
				log.Printf("WARNING! Cannot run experiment: %s", err)
			}

			if !newRes.Pass {
				success = false
			}
		}

		if failure {
			_, err := f.WriteString(e.HostMPI.Version + "\t" + e.ContainerMPI.Version + "\tERROR\t" + newRes.Note + "\n")
			if err != nil {
				log.Fatalf("failed to write result: %s", err)
			}
		} else if !success {
			log.Println("Experiment failed")
			_, err := f.WriteString(e.HostMPI.Version + "\t" + e.ContainerMPI.Version + "\tFAIL\t" + newRes.Note + "\n")
			if err != nil {
				log.Fatalf("failed to write result: %s", err)
			}
			err = f.Sync()
			if err != nil {
				log.Fatalf("failed to sync log file: %s", err)
			}
		} else {
			log.Println("Experiment succeeded")
			_, err := f.WriteString(e.HostMPI.Version + "\t" + e.ContainerMPI.Version + "\tPASS\t" + newRes.Note + "\n")
			if err != nil {
				log.Fatalf("failed to write result: %s", err)
			}
			err = f.Sync()
			if err != nil {
				log.Fatalf("failed to sync log file: %s", err)
			}
		}
	}

	return newResults
}

func testMPI(mpiImplem string, experiments []exp.Config, sysCfg sys.Config, syConfig sy.MPIToolConfig) error {
	// If the user did not specify an output file, we try to implicitly
	// set a relevant name
	if sysCfg.OutputFile == "" {
		err := exp.GetOutputFilename(mpiImplem, &sysCfg)
		if err != nil {
			log.Fatalf("failed to set default output filename: %s", err)
		}
	}

	if mpiImplem == "intel" {
		// Intel MPI is based on OFI so we read our OFI configuration file
		ofiCfg, err := cfg.LoadOFIConfig(sysCfg.OfiCfgFile)
		if err != nil {
			log.Fatalf("failed to read the OFI configuration file: %s", err)
		}
		sysCfg.Ifnet = ofiCfg.Ifnet
	}

	// Display configuration
	log.Println("Current directory:", sysCfg.CurPath)
	log.Println("Binary path:", sysCfg.BinPath)
	log.Println("Output file:", sysCfg.OutputFile)
	log.Println("Running NetPipe:", strconv.FormatBool(sysCfg.NetPipe))
	log.Println("Debug mode:", sysCfg.Debug)
	log.Println("Persistent installs:", sysCfg.Persistent)

	// Load the results we already have in result file
	existingResults, err := results.Load(sysCfg.OutputFile)
	if err != nil {
		log.Fatalf("failed to parse output file %s: %s", sysCfg.OutputFile, err)
	}

	// Remove the results we already have from list of experiments to run
	experimentsToRun := exp.Pruning(experiments, existingResults)

	// Run the experiments
	if len(experimentsToRun) > 0 {
		run(experimentsToRun, &sysCfg, &syConfig)
	}

	results.Analyse(mpiImplem)

	return nil
}

func main() {
	sysCfg, _, _, err := launcher.Load()
	if err != nil {
		log.Fatalf("unable to load configuration: %s", err)

	}

	/* Argument parsing */
	configFile := flag.String("configfile", sysCfg.EtcDir+"/openmpi.conf", "Path to the configuration file specifying which versions of a given implementation of MPI to test")
	outputFile := flag.String("outputFile", "", "Full path to the output file")
	verbose := flag.Bool("v", false, "Enable verbose mode")
	netpipe := flag.Bool("netpipe", false, "Run NetPipe as test")
	imb := flag.Bool("imb", false, "Run IMB as test")
	debug := flag.Bool("d", false, "Enable debug mode")
	nRun := flag.Int("n", 1, "Number of iterations")
	persistent := flag.Bool("persistent-installs", false, "Keep the MPI installations on the host and the container images in the specified directory (instead of deleting everything once an experiment terminates). Default is '~/.sympi', set SYMPI_INSTALL_DIR to overwrite")
	distro := flag.String("distro", "ubuntu:disco", "Identifier of the target Linux distribution for the containers (e.g., 'centos:6', 'ubuntu:disco')")

	flag.Parse()

	sysCfg.ConfigFile = *configFile
	sysCfg.OutputFile = *outputFile
	sysCfg.NetPipe = *netpipe
	sysCfg.IMB = *imb
	sysCfg.Nrun = *nRun
	sysCfg.TargetDistro = *distro
	sysCfg.Verbose = *verbose
	sysCfg.Debug = *debug
	if *persistent {
		sysCfg.Persistent = sys.GetSympiDir()
	}

	config, err := cfg.Parse(sysCfg.ConfigFile)
	if err != nil {
		log.Fatalf("cannot parse %s: %s", sysCfg.ConfigFile, err)
	}

	// Make sure the tool's configuration file is set and load its data
	toolConfigFile, err := sy.CreateMPIConfigFile()
	if err != nil {
		log.Fatalf("cannot setup configuration file: %s", err)
	}
	kvs, err := kv.LoadKeyValueConfig(toolConfigFile)
	if err != nil {
		log.Fatalf("cannot load the tool's configuration file (%s): %s", toolConfigFile, err)
	}
	var syConfig sy.MPIToolConfig
	syConfig.BuildPrivilege, err = strconv.ParseBool(kv.GetValue(kvs, sy.BuildPrivilegeKey))
	if err != nil {
		log.Fatalf("failed to load the tool's configuration: %s", err)
	}

	// Figure out all the experiments that need to be executed
	experiments := getListExperiments(config)
	mpiImplem, err := exp.GetMPIImplemFromExperiments(experiments)
	if err != nil {
		log.Fatalf("failed to figure out the type of experiment: %s", err)
	}
	sysCfg.ScratchDir = buildenv.GetDefaultScratchDir(mpiImplem)
	// If the scratch dir exists, we delete it to start fresh
	err = util.DirInit(sysCfg.ScratchDir)
	if err != nil {
		log.Fatalf("failed to initialize directory %s: %s", sysCfg.ScratchDir, err)
	}

	// Save the options passed in through the command flags
	if sysCfg.Debug {
		sysCfg.Verbose = true
		// If the scratch dir exists, we delete it to start fresh
		err := util.DirInit(sysCfg.ScratchDir)
		if err != nil {
			log.Fatalf("failed to initialize directory %s: %s", sysCfg.ScratchDir, err)
		}

		err = checker.CheckSystemConfig()
		if err != nil {
			log.Fatalf("the system is not correctly setup: %s", err)
		}
	}

	// Initialize the log file. Log messages will both appear on stdout and the log file if the verbose option is used
	logFile := util.OpenLogFile(mpiImplem.ID)
	defer logFile.Close()
	if sysCfg.Verbose {
		nultiWriters := io.MultiWriter(os.Stdout, logFile)
		log.SetOutput(nultiWriters)
	} else {
		log.SetOutput(ioutil.Discard)
	}

	// Sanity checks
	if sysCfg.IMB && sysCfg.NetPipe {
		log.Fatal("please netpipe or imb, not both")
	}

	// Try to detect the local distro. If we cannot, it is not a big deal but we know that for example having
	// different versions of Ubuntu in containers and host may lead to some libc problems
	hostDistro, err := checker.CheckDistro()
	if err != nil {
		log.Println("[INFO] Cannot detect the local distro")
	} else if hostDistro != "" {
		sysCfg.HostDistro = hostDistro
	}

	err = testMPI(mpiImplem.ID, experiments, sysCfg, syConfig)
	if err != nil {
		log.Fatalf("failed test MPI: %s", err)
	}
}
