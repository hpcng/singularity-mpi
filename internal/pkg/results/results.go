// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package results

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"

	util "github.com/sylabs/singularity-mpi/internal/pkg/util/file"
	exp "github.com/sylabs/singularity-mpi/pkg/experiments"
)

// Result represents the result of a given experiment
type Result struct {
	experiment exp.Experiment
	pass       bool
}

// Load reads a output file and load the list of experiments that are in the file
func Load(outputFile string) ([]Result, error) {
	var existingResults []Result

	log.Println("Reading results from", outputFile)

	f, err := os.Open(outputFile)
	if err != nil {
		// No result file, it is okay
		return existingResults, nil
	}
	defer f.Close()

	lineReader := bufio.NewScanner(f)
	if lineReader == nil {
		return existingResults, fmt.Errorf("failed to create file reader")
	}

	for lineReader.Scan() {
		line := lineReader.Text()
		words := strings.Split(line, "\t")
		var newResult Result
		if len(words) < 3 {
			return existingResults, fmt.Errorf("invalid format: %s", line)
		}
		newResult.experiment.VersionHostMPI = words[0]
		newResult.experiment.VersionContainerMPI = words[1]
		result := words[2]
		switch result {
		case "PASS":
			newResult.pass = true
		case "FAIL":
			newResult.pass = false
		default:
			return existingResults, fmt.Errorf("invalid experiment result: %s", result)
		}
		existingResults = append(existingResults, newResult)
	}

	return existingResults, nil
}

// Pruning removes the experiments for which we already have results
func Pruning(experiments []exp.Experiment, existingResults []Result) []exp.Experiment {
	// No optimization at the moment, double loop and creation of a new array
	var experimentsToRun []exp.Experiment
	//	for j := 0; j < len(experiments); j++ {
	for _, experiment := range experiments {
		found := false
		for _, result := range existingResults {
			if experiment.VersionHostMPI == result.experiment.VersionHostMPI && experiment.VersionContainerMPI == result.experiment.VersionContainerMPI {
				log.Printf("We already have results for %s on the host and %s in a container, skipping...\n", experiment.VersionHostMPI, experiment.VersionContainerMPI)
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

func lookupResult(r []Result, hostVersion string, containerVersion string) bool {
	var i int
	for i = 0; i < len(r); i++ {
		if r[i].experiment.VersionHostMPI == hostVersion && r[i].experiment.VersionContainerMPI == containerVersion {
			return r[i].pass
		}
	}

	return false
}

func createCompatibilityMatrix(mpiImplem string, initFile string, netpipeFile string, imbFile string) error {
	outputFile := mpiImplem + "_compatibility_matrix.txt"

	initResults, err := Load(initFile)
	if err != nil {
		return err
	}

	netpipeResults, err := Load(netpipeFile)
	if err != nil {
		return err
	}

	imbResults, err := Load(imbFile)
	if err != nil {
		return err
	}

	compatibilityResults := ""

	var i int
	for i = 0; i < len(initResults); i++ {
		testPassed := false

		if initResults[i].pass {
			passNetpipe := lookupResult(
				netpipeResults,
				initResults[i].experiment.VersionHostMPI,
				initResults[i].experiment.VersionContainerMPI,
			)
			if passNetpipe {
				passIMB := lookupResult(
					imbResults,
					initResults[i].experiment.VersionHostMPI,
					initResults[i].experiment.VersionContainerMPI,
				)
				if passIMB {
					testPassed = true
				}
			}
		}

		compatibilityResults += initResults[i].experiment.VersionHostMPI +
			"\t" +
			initResults[i].experiment.VersionContainerMPI +
			"\t" +
			strconv.FormatBool(testPassed) +
			"\n"
	}

	err = ioutil.WriteFile(outputFile, []byte(compatibilityResults), 0777)
	if err != nil {
		return err
	}

	return nil
}

// Analyse checks whether all the result files are present and if so, create
// the compatibility matrix.
func Analyse(mpiImplem string) {
	// todo we need to make that better, it should not be hardcoded here
	initOutputFile := mpiImplem + "-init-results.txt"
	netpipeOutputFile := mpiImplem + "-netpipe-results.txt"
	imbOutputFile := mpiImplem + "-imb-results.txt"

	if util.FileExists(initOutputFile) && util.FileExists(netpipeOutputFile) && util.FileExists(imbOutputFile) {
		err := createCompatibilityMatrix(mpiImplem, initOutputFile, netpipeOutputFile, imbOutputFile)
		if err != nil {
			log.Fatalf("Cannot create the compatibility matrix")
		}
	}
}
