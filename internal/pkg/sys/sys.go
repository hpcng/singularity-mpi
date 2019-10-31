// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package sys

import (
	"os"
	"path/filepath"
)

const (
	// SYMPI_INSTALL_DIR_ENV is the name of the environment variable to set the
	// directory used to install MPI and store container images
	SYMPI_INSTALL_DIR_ENV = "SYMPI_INSTALL_DIR"

	// DefaultSympiInstallDir is the name of the default directory in $HOME to store
	// image containers and install MPI
	DefaultSympiInstallDir = ".sympi"

	// CmdTimetout is the maximum time we allow a command to run
	CmdTimeout = 10

	// DefaultUbuntuDistro is the default Ubuntu distribution we use
	DefaultUbuntuDistro = "disco"
)

// SetConfigFn is a "function pointer" that lets us store the configuration of a given job manager
type SetConfigFn func() error

// GetConfigFn is a "function pointer" that lets us get the configuration of a given job manager
type GetConfigFn func() error

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

	// Verbose mode is active/inactive
	Verbose bool

	// Debug mode is active/inactive
	Debug bool

	// Nrun specifies the number of iterations, i.e., number of times the test is executed
	Nrun int

	// AppContainizer is the path to the configuration for automatic containerization of app
	AppContainizer string

	// Registry is the optinal user registery where images can be uploaded
	Registry string

	// Upload specifies whether images needs to be uploaded to the registery
	Upload bool

	// Persistent specifies whether we need to keep the installed software (MPI and containers)
	Persistent string

	// SlurmEnable specifies whether Slurm is currently enabled
	SlurmEnabled bool

	// IBEnables specifies whether Infiniband is currently enabled
	IBEnabled bool

	// SyConfigFile
	SyConfigFile string
}

// GetSympiDir returns the directory where MPI is installed and container images
// stored
func GetSympiDir() string {
	if os.Getenv(SYMPI_INSTALL_DIR_ENV) != "" {
		return os.Getenv(SYMPI_INSTALL_DIR_ENV)
	} else {
		return filepath.Join(os.Getenv("HOME"), DefaultSympiInstallDir)
	}
}
