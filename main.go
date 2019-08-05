// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package main

import (
	"flag"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"singularity-mpi/checker"
	cfg "singularity-mpi/configparser"
	exp "singularity-mpi/experiments"
	"singularity-mpi/results"
)

const (
	defaultUbuntuDistro = "disco"
)

func getListExperiments(config *cfg.Config) []exp.Experiment {
	var experiments []exp.Experiment
	for mpi1, mpi1url := range config.MpiMap {
		for mpi2, mpi2url := range config.MpiMap {
			newExperiment := exp.Experiment{
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

func getMPIImplemFromExperiments(experiments []exp.Experiment) string {
	// Fair assumption: all experiments are based on the same MPI
	// implementation (we actually check for that and the implementation
	// is only included in the experiment structure so that the structure
	// is self-contained).
	if len(experiments) == 0 {
		return ""
	}

	return experiments[0].MPIImplm
}

func run(experiments []exp.Experiment, sysCfg *exp.SysConfig) []results.Result {
	var results []results.Result

	/* Sanity checks */
	if sysCfg == nil || sysCfg.OutputFile == "" {
		log.Fatalf("invalid parameter(s)")
	}

	f, err := os.OpenFile(sysCfg.OutputFile, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		log.Fatalf("failed to open file %s: %s", sysCfg.OutputFile, err)
	}
	defer f.Close()

	for _, e := range experiments {
		log.Printf("Running experiment with host MPI %s and container MPI %s\n", e.VersionHostMPI, e.VersionContainerMPI)
		success, note, err := exp.Run(e, sysCfg)
		if err != nil {
			log.Printf("WARNING! Cannot run experiment: %s", err)
			_, err := f.WriteString(e.VersionHostMPI + "\t" + e.VersionContainerMPI + "\tERROR\t" + note + "\n")
			if err != nil {
				log.Fatalf("failed to write result: %s", err)
			}
		} else {
			if success {
				log.Println("Experiment succeeded")
				_, err := f.WriteString(e.VersionHostMPI + "\t" + e.VersionContainerMPI + "\tPASS\t" + note + "\n")
				if err != nil {
					log.Fatalf("failed to write result: %s", err)
				}
				err = f.Sync()
				if err != nil {
					log.Fatalf("failed to sync log file: %s", err)
				}
			} else {
				log.Println("Experiment failed")
				_, err := f.WriteString(e.VersionHostMPI + "\t" + e.VersionContainerMPI + "\tFAIL\t" + note + "\n")
				if err != nil {
					log.Fatalf("failed to write result: %s", err)
				}
				err = f.Sync()
				if err != nil {
					log.Fatalf("failed to sync log file: %s", err)
				}
			}
		}
	}
	return results
}

func setDefaultOutputFile(experiments []exp.Experiment, sysCfg *exp.SysConfig) error {
	// We get the MPI implementation from the list
	mpiImplem := getMPIImplemFromExperiments(experiments)
	if !sysCfg.NetPipe {
		sysCfg.OutputFile = mpiImplem + "-init-results.txt"
	} else {
		sysCfg.OutputFile = mpiImplem + "-netpipe-results.txt"
	}

	return nil
}

func main() {
	var sysCfg exp.SysConfig

	/* Figure out the directory of this binary */
	bin, err := os.Executable()
	if err != nil {
		log.Fatal("cannot detect the directory of the binary")
	}

	sysCfg.BinPath = filepath.Dir(bin)
	sysCfg.EtcDir = filepath.Join(sysCfg.BinPath, "etc")
	sysCfg.TemplateDir = filepath.Join(sysCfg.EtcDir, "templates")
	sysCfg.OfiCfgFile = filepath.Join(sysCfg.EtcDir, "ofi.conf")
	sysCfg.ScratchDir = filepath.Join(sysCfg.BinPath, "scratch")

	/* Figure out the current path */
	sysCfg.CurPath, err = os.Getwd()
	if err != nil {
		log.Fatal("cannot detect current directory")
	}

	/* Argument parsing */
	configFile := flag.String("configfile", sysCfg.BinPath+"/etc/openmpi.conf", "Path to the configuration file specifying which versions of a given implementation of MPI to test")
	outputFile := flag.String("outputFile", "", "Full path to the output file")
	verbose := flag.Bool("v", false, "Enable verbose mode")
	netpipe := flag.Bool("netpipe", false, "Perform NetPipe rather than a basic hello world test")
	debug := flag.Bool("d", false, "Enable debug mode")

	flag.Parse()

	// Save the options passed in through the command flags
	if *debug {
		*verbose = true
		sysCfg.Debug = *debug
		// If the scratch dir exists, we delete it to start fresh
		if _, err := os.Stat(sysCfg.ScratchDir); !os.IsNotExist(err) {
			os.RemoveAll(sysCfg.ScratchDir)
		}
		err := os.MkdirAll(sysCfg.ScratchDir, 0755)
		if err != nil {
			log.Fatalf("failed to create scratch directory: %s", err)
		}

		err = checker.CheckSystemConfig()
		if err != nil {
			log.Fatalf("the system is not correctly setup: %s", err)
		}
	}
	if !*verbose {
		log.SetOutput(ioutil.Discard)
	}
	sysCfg.ConfigFile = *configFile
	sysCfg.OutputFile = *outputFile
	sysCfg.NetPipe = *netpipe

	config, err := cfg.Parse(sysCfg.ConfigFile)
	if err != nil {
		log.Fatalf("cannot parse %s: %s", sysCfg.ConfigFile, err)
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

	// Initialize the log file. Log messages will both appear on stdout and the log file if the verbose option is used
	logFile, err := os.OpenFile("singularity-mpi.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("failed to create log file: %s", err)
	}
	defer logFile.Close()
	if *verbose {
		nultiWriters := io.MultiWriter(os.Stdout, logFile)
		log.SetOutput(nultiWriters)
	} else {
		log.SetOutput(logFile)
	}

	// Figure out all the experiments that need to be executed
	experiments := getListExperiments(config)

	// If the user did not specify an output file, we try to implicitly
	// set a relevant name
	if sysCfg.OutputFile == "" {
		err = setDefaultOutputFile(experiments, &sysCfg)
		if err != nil {
			log.Fatalf("failed to set default output filename: %s", err)
		}
	}

	if experiments[0].MPIImplm == "intel" {
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
		log.Fatalf("failed to parse output file %s: %s", *outputFile, err)
	}

	// Remove the results we already have from list of experiments to run
	experimentsToRun := results.Pruning(experiments, existingResults)

	// Run the experiments
	run(experimentsToRun, &sysCfg)
}
