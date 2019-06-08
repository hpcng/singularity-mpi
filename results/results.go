// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package results

import (
	"bufio"
	"fmt"
	"os"
	exp "singularity-mpi/experiments"
	"strings"
)

// Result represents the result of a given experiment
type Result struct {
	experiment exp.Experiment
	pass       bool
}

// Load reads a output file and load the list of experiments that are in the file
func Load(outputFile string) ([]Result, error) {
	var existingResults []Result

	fmt.Println ("Reading results from", outputFile)

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
		switch (result) {
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
				fmt.Println ("We already have results for %s on the host and %s in a container, skipping...\n", experiment.VersionHostMPI, experiment.VersionContainerMPI)
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
