// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package sys

// Config captures some system configuration aspects that are necessary
// to run experiments
type Config struct {
	// ConfigFile is the path to the configuration file describing experiments
	ConfigFile string
	// TargetUbuntuDistro is the version of Ubuntu that we will run in the containers
	TargetUbuntuDistro string
	// BinPath is the path to the current binary
	BinPath string
	// CurPath is the current path
	CurPath string
	// EtcDir is the path to the directory with the configuration files
	EtcDir string
	// TemplateDir is the path where the template are
	TemplateDir string
	// ScratchDir is the path where a copy generated files are saved for debugging
	ScratchDir string
	// SedBin is the path to the sed binary
	SedBin string
	// SingularityBin is the path to the singularity binary
	SingularityBin string
	// OutputFile is the path the output file
	OutputFile string
	// Netpipe specifies whether we need to execute NetPipe as test
	NetPipe bool
	// IMB specifies whether we need to execute IMB as test
	IMB bool
	// OfiCfgFile is the absolute path to the OFI configuration file
	OfiCfgFile string
	// Ifnet is the network interface to use (e.g., required to setup OFI)
	Ifnet string
	// Debug mode is active/inactive
	Debug bool
	// Nrun specifies the number of iterations, i.e., number of times the test is executed
	Nrun int
	// AppContainizer is the path to the configuration for automatic containerization of app
	AppContainizer string
	// Register is the optinal user registery where images can be uploaded
	Registery string
	// Upload specifies whether images needs to be uploaded to the registery
	Upload bool
}

const (
	// CmdTimetout is the maximum time we allow a command to run
	CmdTimeout = 10
)
