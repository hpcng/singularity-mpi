// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package sys

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	// SYMPI_INSTALL_DIR_ENV is the name of the environment variable to set the
	// directory used to install MPI and store container images
	SYMPI_INSTALL_DIR_ENV = "SYMPI_INSTALL_DIR"

	// DefaultSympiInstallDir is the name of the default directory in $HOME to store
	// image containers and install MPI
	DefaultSympiInstallDir = ".sympi"

	// CmdTimeout is the maximum time we allow a command to run
	CmdTimeout = 30

	// DefaultUbuntuDistro is the default Ubuntu distribution we use
	DefaultUbuntuDistro = "disco"

	// SingularityInstallDirPrefix is the default prefix for the directory name to use for an installation of Singularity
	SingularityInstallDirPrefix = "install_singularity-"

	// SingularityBuildDirPrefix is the default prefix for the directory name where Singularity is built
	SingularityBuildDirPrefix = "build_singularity-"

	// SingularityScratchDirPrefix is the default prefix for the directory name to use as scratch for preparing Singularity
	SingularityScratchDirPrefix = "scratch_singularity-"

	// MPIInstallDirPrefix is the default prefix for the directory name where a version of MPI is installed
	MPIInstallDirPrefix = "mpi_install_"

	// MPIBuildDirPrefix is the default prefix for the directory name where a version of MPI is built
	MPIBuildDirPrefix = "mpi_build_"

	// ContainerInstallDirPrefix is the default prefix for the directory name where an MPI-based container is stored
	ContainerInstallDirPrefix = "mpi_container_"

	confFilePrefix = "sympi_"
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

	// TargetDistro is the Linux distribution identifier that we will be used in the containers
	TargetDistro string

	// HostDistro is the Linux distribution on the host
	HostDistro string

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

	// Registry is the optinal user registry where images can be uploaded
	Registry string

	// Upload specifies whether images needs to be uploaded to the registry
	Upload bool

	// Persistent specifies whether we need to keep the installed software (MPI and containers)
	Persistent string

	// SlurmEnable specifies whether Slurm is currently enabled
	SlurmEnabled bool

	// IBEnables specifies whether Infiniband is currently enabled
	IBEnabled bool

	// SyConfigFile
	SyConfigFile string

	// Nopriv specifies whether we need to use the '-u' option when running singularity
	Nopriv bool

	// SudoSyCmds is the list of Singularity commands that need to be executed with sudo
	SudoSyCmds []string

	// SudoBin is the path to sudo on the host
	SudoBin string
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

// ParseDistroID parses the string we use to identify a specific distro into a distribution name and its version
func ParseDistroID(distro string) (string, string) {
	if !strings.Contains(distro, ":") {
		log.Printf("[WARN] %s an invalid distro ID\n", distro)
		return "", ""
	}

	tokens := strings.Split(distro, ":")
	if len(tokens) != 2 {
		log.Printf("[WARN] %s an invalid distro ID\n", distro)
		return "", ""
	}

	return tokens[0], tokens[1]
}

// GetDistroID returns a formatted version of the value of TargetDistro.
//
// This is mainly used to have a standard way to set directory and file names
func GetDistroID(distro string) string {
	return strings.Replace(distro, ":", "_", 1)
}

// CompatibleArch checks whether the local architecture is compatible with a list of architectures.
//
// The list of architectures is for example the output of sy.GetSIFArchs()
func CompatibleArch(list []string) bool {
	for _, arch := range list {
		if arch == runtime.GOARCH {
			return true
		}
	}
	return false
}

// IsPersistent checks whether the system is setup for persistent installs or not
func IsPersistent(sysCfg *Config) bool {
	if sysCfg != nil && sysCfg.Persistent != "" {
		return true
	}

	return false
}

// GetMPIConfigFileName return the name of the configuration file for a specific implementation of MPI
func GetMPIConfigFileName(mpi string) string {
	switch mpi {
	case "openmpi":
		return confFilePrefix + "openmpi.conf"
	case "mpich":
		return confFilePrefix + "mpich.conf"
	case "intel":
		return confFilePrefix + "intel.conf"
	default:
		return ""
	}
}
