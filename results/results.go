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
	"strconv"
	"strings"
)

// Result represents the result of a given experiment
type Result struct {
	experiment exp.Experiment
	pass       bool
}

// Write stores in a file the content of an array of results
func Write(outputFile string, results []Result) error {
	f, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("cannot create file: %s", err)
	}
	defer f.Close()

	writer := bufio.NewWriter(f)

	for _, result := range results {
		fmt.Fprintf(writer, "%s %s %v\n", result.experiment.VersionHostMPI, result.experiment.VersionContainerMPI, result.pass)
	}

	return nil
}

// Load reads a output file and load the list of experiments that are in the file
func Load(outputFile string) ([]Result, error) {
	var existingResults []Result

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
		words := strings.Split(line, " ")
		var newResult Result
		newResult.experiment.VersionHostMPI = words[0]
		newResult.experiment.VersionContainerMPI = words[1]
		newResult.pass, err = strconv.ParseBool(words[2])
		if err != nil {
			return existingResults, fmt.Errorf("error while parsing result file %s :%s", outputFile, err)
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
