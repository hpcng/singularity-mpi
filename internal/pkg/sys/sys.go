// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package sys

// Config captures some system configuration aspects that are necessary
// to run experiments
type Config struct {
	ConfigFile         string // Path to the configuration file describing experiments
	TargetUbuntuDistro string // The version of Ubuntu that we will run in the containers
	BinPath            string // Path to the current binary
	CurPath            string // Current path
	EtcDir             string // Path to the directory with the configuration files
	TemplateDir        string // Where the template are
	ScratchDir         string // Where a copy generated files are saved for debugging
	SedBin             string // Path to the sed binary
	SingularityBin     string // Path to the singularity binary
	OutputFile         string // Path the output file
	NetPipe            bool   // Execute NetPipe as test
	IMB                bool   // Execute IMB as test
	OfiCfgFile         string // Absolute path to the OFI configuration file
	Ifnet              string // Network interface to use (e.g., required to setup OFI)
	Debug              bool   // Debug mode is active/inactive
	Nrun               int    // Number of iterations, i.e., number of times the test is executed
	AppContainizer     string // Path to the configuration for automatic containerization of app
}

const (
	// CmdTimetout is the maximum time we allow a command to run
	CmdTimeout = 10
)
