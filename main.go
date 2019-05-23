// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	cfg "singularity-mpi/configparser"
	exp "singularity-mpi/experiments"
	"singularity-mpi/results"
)

func getListExperiments(config *cfg.Config) []exp.Experiment {
	var experiments []exp.Experiment
	for mpi1, mpi1url := range config.MpiMap {
		for mpi2, mpi2url := range config.MpiMap {
			newExperiment := exp.Experiment{
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

func run(experiments []exp.Experiment) []results.Result {
	var results []results.Result
	for _, e := range experiments {
		fmt.Printf("Running experiment with host MPI %s and container MPI %s\n", e.VersionHostMPI, e.VersionContainerMPI)
		success, err := exp.Run(e)
		if err != nil {
			fmt.Printf("WARNING! Cannot run experiment: %s", err)
			log.Fatal("error detected, stopping")
		}
		if success {
			fmt.Println("Experiment succeeded")
		} else {
			fmt.Println("Experiment failed")
		}
		os.Exit(0)
	}
	return results
}

func main() {
	/* Figure out the directory of this binary */
	bin, err := os.Executable()
	if err != nil {
		log.Fatal("cannot detect the directory of the binary")
	}

	binPath := filepath.Dir(bin)

	/* Figure out the current path */
	curPath, err := os.Getwd()
	if err != nil {
		log.Fatal("cannot detect current directory")
	}

	/* Argument parsing */
	configFile := flag.String("configfile", binPath+"/etc/openmpi.conf", "Path to the configuration file specifying which versions of a given implementation of MPI to test")
	outputFile := flag.String("outputFile", "./mpi-results.txt", "Full path to the output file")
	verbose := flag.Bool("v", false, "Enable/disable verbosity")

	flag.Parse()

	if *verbose == false {
		log.SetOutput(ioutil.Discard)
	}

	config, err := cfg.Parse(*configFile)
	if err != nil {
		log.Fatal("cannot parse", *configFile, " - ", err)
	}

	// Display configuration
	fmt.Println("Current directory:", curPath)
	fmt.Println("Binary path:", binPath)

	// Figure out all the experiments that need to be executed
	experiments := getListExperiments(config)

	// Load the results we already have in result file
	existingResults, err := results.Load(*outputFile)
	if err != nil {
		log.Fatalf("failed to parse output file %s: %s", *outputFile, err)
	}

	// Remove the results we already have from list of experiments to run
	experimentsToRun := results.Pruning(experiments, existingResults)

	// Run the experiments
	run(experimentsToRun)
}
