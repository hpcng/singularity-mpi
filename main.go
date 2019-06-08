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
	"strconv"

	cfg "singularity-mpi/configparser"
	exp "singularity-mpi/experiments"
	"singularity-mpi/results"
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
		fmt.Printf("Running experiment with host MPI %s and container MPI %s\n", e.VersionHostMPI, e.VersionContainerMPI)
		success, note, err := exp.Run(e, sysCfg)
		if err != nil {
			fmt.Printf("WARNING! Cannot run experiment: %s", err)
			_, err := f.WriteString(e.VersionHostMPI + "\t" + e.VersionContainerMPI + "\tERROR\t" + note + "\n")
			if err != nil {
				log.Fatalf("failed to write result: %s", err)
			}
		} else {
			if success {
				fmt.Println("Experiment succeeded")
				_, err := f.WriteString(e.VersionHostMPI + "\t" + e.VersionContainerMPI + "\tPASS\t" + note + "\n")
				if err != nil {
					log.Fatalf("failed to write result: %s", err)
				}
				f.Sync()
			} else {
				fmt.Println("Experiment failed")
				_, err := f.WriteString(e.VersionHostMPI + "\t" + e.VersionContainerMPI + "\tFAIL\t" + note + "\n")
				if err != nil {
					log.Fatalf("failed to write result: %s", err)
				}
				f.Sync()
			}
		}
	}
	return results
}

func main() {
	var sysCfg exp.SysConfig

	/* Figure out the directory of this binary */
	bin, err := os.Executable()
	if err != nil {
		log.Fatal("cannot detect the directory of the binary")
	}

	sysCfg.BinPath = filepath.Dir(bin)
	sysCfg.TemplateDir = filepath.Join(sysCfg.BinPath, "etc", "templates")

	/* Figure out the current path */
	sysCfg.CurPath, err = os.Getwd()
	if err != nil {
		log.Fatal("cannot detect current directory")
	}

	/* Argument parsing */
	configFile := flag.String("configfile", sysCfg.BinPath+"/etc/openmpi.conf", "Path to the configuration file specifying which versions of a given implementation of MPI to test")
	outputFile := flag.String("outputFile", "", "Full path to the output file")
	verbose := flag.Bool("v", false, "Enable/disable verbosity")
	netpipe := flag.Bool("netpipe", false, "Perform NetPipe rather than a basic hello world test")

	flag.Parse()

	// Save the options passed in through the command flags
	if *verbose == false {
		log.SetOutput(ioutil.Discard)
	}
	sysCfg.ConfigFile = *configFile
	sysCfg.OutputFile = *outputFile
	sysCfg.NetPipe = *netpipe

	config, err := cfg.Parse(sysCfg.ConfigFile)
	if err != nil {
		log.Fatal("cannot parse", sysCfg.ConfigFile, " - ", err)
	}

	// Figure out all the experiments that need to be executed
	experiments := getListExperiments(config)

	// If the user did not specify an output file, we try to implicitly
	// set a relevant name
	if sysCfg.OutputFile == "" {
		// We get the MPI implementation from the list
		mpiImplem := getMPIImplemFromExperiments(experiments)
		sysCfg.OutputFile = mpiImplem + "-results.txt"
	}

	// Display configuration
	fmt.Println("Current directory:", sysCfg.CurPath)
	fmt.Println("Binary path:", sysCfg.BinPath)
	fmt.Println("Output file:", sysCfg.OutputFile)
	fmt.Println("Running NetPipe:", strconv.FormatBool(sysCfg.NetPipe))

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
