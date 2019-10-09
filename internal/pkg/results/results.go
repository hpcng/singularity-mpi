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

	"github.com/sylabs/singularity-mpi/internal/pkg/implem"
	util "github.com/sylabs/singularity-mpi/internal/pkg/util/file"
)

// Result represents the result of a given experiment
type Result struct {
	HostMPI      implem.Info
	ContainerMPI implem.Info
	Pass         bool
	Note         string
}

func lookupResult(r []Result, hostVersion string, containerVersion string) bool {
	var i int
	for i = 0; i < len(r); i++ {
		if r[i].HostMPI.Version == hostVersion && r[i].ContainerMPI.Version == containerVersion {
			return r[i].Pass
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

		if initResults[i].Pass {
			passNetpipe := lookupResult(
				netpipeResults,
				initResults[i].HostMPI.Version,
				initResults[i].ContainerMPI.Version,
			)
			if passNetpipe {
				passIMB := lookupResult(
					imbResults,
					initResults[i].HostMPI.Version,
					initResults[i].ContainerMPI.Version,
				)
				if passIMB {
					testPassed = true
				}
			}
		}

		compatibilityResults += initResults[i].HostMPI.Version +
			"\t" +
			initResults[i].ContainerMPI.Version +
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
		log.Println("All expected result files found, creating compatibility matrix...")
		err := createCompatibilityMatrix(mpiImplem, initOutputFile, netpipeOutputFile, imbOutputFile)
		if err != nil {
			log.Fatalf("Cannot create the compatibility matrix")
		}
	}
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
		newResult.HostMPI.Version = words[0]
		newResult.ContainerMPI.Version = words[1]
		result := words[2]
		switch result {
		case "PASS":
			newResult.Pass = true
		case "FAIL":
			newResult.Pass = false
		default:
			return existingResults, fmt.Errorf("invalid experiment result: %s", result)
		}
		existingResults = append(existingResults, newResult)
	}

	return existingResults, nil
}
