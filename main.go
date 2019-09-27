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

	"github.com/sylabs/singularity-mpi/pkg/containizer"

	cfg "github.com/sylabs/singularity-mpi/internal/pkg/configparser"
	"github.com/sylabs/singularity-mpi/internal/pkg/mpi"
	"github.com/sylabs/singularity-mpi/internal/pkg/results"
	"github.com/sylabs/singularity-mpi/internal/pkg/sys"
	exp "github.com/sylabs/singularity-mpi/pkg/experiments"

	"github.com/sylabs/singularity-mpi/internal/pkg/checker"
	util "github.com/sylabs/singularity-mpi/internal/pkg/util/file"
	"github.com/sylabs/singularity-mpi/internal/pkg/util/sy"
)

const (
	defaultUbuntuDistro = "disco"
)

func getListExperiments(config *cfg.Config) []mpi.Experiment {
	var experiments []mpi.Experiment
	for mpi1, mpi1url := range config.MpiMap {
		for mpi2, mpi2url := range config.MpiMap {
			newExperiment := mpi.Experiment{
				MPIImplm:            config.MPIImplem,
				VersionHostMPI:      mpi1,
				VersionContainerMPI: mpi2,
				URLHostMPI:          mpi1url,
				URLContainerMPI:     mpi2url,
			}
			experiments = append(experiments, newExperiment)
		}
	}

	return experiments
}

func runExperiment(e mpi.Experiment, sysCfg *sys.Config) (results.Result, error) {
	var res results.Result
	var err error

	res.Experiment = e
	res.Pass, res.Note, err = exp.Run(e, sysCfg)
	if err != nil {
		return res, fmt.Errorf("failure during the execution of the experiment: %s", err)
	}

	return res, nil
}

func run(experiments []mpi.Experiment, sysCfg *sys.Config) []results.Result {
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

		var i int
		for i = 0; i < sysCfg.Nrun; i++ {
			log.Printf("Running experiment %d/%d with host MPI %s and container MPI %s\n", i+1, sysCfg.Nrun, e.VersionHostMPI, e.VersionContainerMPI)
			newRes, err := runExperiment(e, sysCfg)
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
			_, err := f.WriteString(e.VersionHostMPI + "\t" + e.VersionContainerMPI + "\tERROR\t" + newRes.Note + "\n")
			if err != nil {
				log.Fatalf("failed to write result: %s", err)
			}
		} else if !success {
			log.Println("Experiment failed")
			_, err := f.WriteString(e.VersionHostMPI + "\t" + e.VersionContainerMPI + "\tFAIL\t" + newRes.Note + "\n")
			if err != nil {
				log.Fatalf("failed to write result: %s", err)
			}
			err = f.Sync()
			if err != nil {
				log.Fatalf("failed to sync log file: %s", err)
			}
		} else {
			log.Println("Experiment succeeded")
			_, err := f.WriteString(e.VersionHostMPI + "\t" + e.VersionContainerMPI + "\tPASS\t" + newRes.Note + "\n")
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

func testMPI(mpiImplem string, experiments []mpi.Experiment, sysCfg sys.Config) error {
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

	// Load the results we already have in result file
	existingResults, err := results.Load(sysCfg.OutputFile)
	if err != nil {
		log.Fatalf("failed to parse output file %s: %s", sysCfg.OutputFile, err)
	}

	// Remove the results we already have from list of experiments to run
	experimentsToRun := results.Pruning(experiments, existingResults)

	// Run the experiments
	if len(experimentsToRun) > 0 {
		run(experimentsToRun, &sysCfg)
	}

	results.Analyse(mpiImplem)

	return nil
}

func main() {
	var sysCfg sys.Config

	/* Figure out the directory of this binary */
	bin, err := os.Executable()
	if err != nil {
		log.Fatal("cannot detect the directory of the binary")
	}

	sysCfg.BinPath = filepath.Dir(bin)
	sysCfg.EtcDir = filepath.Join(sysCfg.BinPath, "etc")
	sysCfg.TemplateDir = filepath.Join(sysCfg.EtcDir, "templates")
	sysCfg.OfiCfgFile = filepath.Join(sysCfg.EtcDir, "ofi.conf")

	/* Figure out the current path */
	sysCfg.CurPath, err = os.Getwd()
	if err != nil {
		log.Fatal("cannot detect current directory")
	}

	/* Argument parsing */
	configFile := flag.String("configfile", sysCfg.BinPath+"/etc/openmpi.conf", "Path to the configuration file specifying which versions of a given implementation of MPI to test")
	outputFile := flag.String("outputFile", "", "Full path to the output file")
	verbose := flag.Bool("v", false, "Enable verbose mode")
	netpipe := flag.Bool("netpipe", false, "Run NetPipe as test")
	imb := flag.Bool("imb", false, "Run IMB as test")
	debug := flag.Bool("d", false, "Enable debug mode")
	nRun := flag.Int("n", 1, "Number of iterations")
	appContainizer := flag.String("app-containizer", "", "Path to the configuration file for automatically containerization an application")
	upload := flag.Bool("upload", false, "Upload generated images (appropriate configuration files need to specify the registry's URL")

	flag.Parse()

	sysCfg.ConfigFile = *configFile
	sysCfg.OutputFile = *outputFile
	sysCfg.NetPipe = *netpipe
	sysCfg.IMB = *imb
	sysCfg.Nrun = *nRun
	sysCfg.AppContainizer = *appContainizer
	sysCfg.Upload = *upload
	sysCfg.Verbose = *verbose
	sysCfg.Debug = *debug

	config, err := cfg.Parse(sysCfg.ConfigFile)
	if err != nil {
		log.Fatalf("cannot parse %s: %s", sysCfg.ConfigFile, err)
	}
	// Figure out all the experiments that need to be executed
	experiments := getListExperiments(config)
	mpiImplem := exp.GetMPIImplemFromExperiments(experiments)

	scratchPath := "scratch-" + mpiImplem
	sysCfg.ScratchDir = filepath.Join(sysCfg.BinPath, scratchPath)

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
	logFile := util.OpenLogFile(mpiImplem)
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

	_, err = sy.CreateMPIConfigFile()
	if err != nil {
		log.Fatalf("cannot setup configuration file: %s", err)
	}

	// Try to detect the local distro. If we cannot, it is not a big deal but we know that for example having
	// different versions of Ubuntu in containers and host may lead to some libc problems
	sysCfg.TargetUbuntuDistro = defaultUbuntuDistro // By default, containers will use a specific Ubuntu distro
	distro, err := checker.CheckDistro()
	if err != nil {
		log.Println("[INFO] Cannot detect the local distro")
	} else if distro != "" {
		sysCfg.TargetUbuntuDistro = distro
	}

	// Run the requested tool capability
	if sysCfg.AppContainizer != "" {
		err := containizer.ContainerizeApp(&sysCfg)
		if err != nil {
			log.Fatalf("failed to create container for app: %s", err)
		}
	} else {
		err := testMPI(mpiImplem, experiments, sysCfg)
		if err != nil {
			log.Fatalf("failed test MPI: %s", err)
		}
	}
}
