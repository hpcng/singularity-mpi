// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package jm

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/sylabs/singularity-mpi/internal/pkg/sys"

	"github.com/sylabs/singularity-mpi/internal/pkg/mpi"
)

// Native is the structure representing the native job manager (i.e., directly use mpirun)
type Native struct {
}

// NativeSetConfig sets the configuration of the native job manager
func NativeSetConfig() error {
	return nil
}

// NativeGetConfig gets the configuration of the native job manager
func NativeGetConfig() error {
	return nil
}

func getEnvPath(mpiCfg *mpi.Config) string {
	// Intel MPI is installing the binaries and libraries in a quite complex setup
	if mpiCfg.MpiImplm == "intel" {
		return filepath.Join(mpiCfg.InstallDir, mpi.IntelInstallPathPrefix, "bin") + ":" + os.Getenv("PATH")
	}

	return filepath.Join(mpiCfg.InstallDir, "bin") + ":" + os.Getenv("PATH")
}

func getEnvLDPath(mpiCfg *mpi.Config) string {
	// Intel MPI is installing the binaries and libraries in a quite complex setup
	if mpiCfg.MpiImplm == "intel" {
		return filepath.Join(mpiCfg.InstallDir, mpi.IntelInstallPathPrefix, "lib") + ":" + os.Getenv("LD_LIBRARY_PATH")
	}

	return filepath.Join(mpiCfg.InstallDir, "lib") + ":" + os.Getenv("LD_LIBRARY_PATH")
}

// NativeGetOutput retrieves the application's output after the completion of a job
func NativeGetOutput(j *Job, sysCfg *sys.Config) string {
	return j.OutBuffer.String()
}

// NativeGetError retrieves the error messages from an application after the completion of a job
func NativeGetError(j *Job, sysCfg *sys.Config) string {
	return j.ErrBuffer.String()
}

// NativeSubmit is the function to call to submit a job through the native job manager
func NativeSubmit(j *Job, sysCfg *sys.Config) (Launcher, error) {
	var l Launcher

	if j.AppBin == "" {
		return l, fmt.Errorf("application binary is undefined")
	}

	l.Cmd = mpi.GetPathToMpirun(j.HostCfg)
	if j.NP > 0 {
		l.CmdArgs = append(l.CmdArgs, "-np")
		l.CmdArgs = append(l.CmdArgs, strconv.FormatInt(j.NP, 10))
	}

	mpirunArgs, err := mpi.GetMpirunArgs(j.HostCfg, j.ContainerCfg)
	if err != nil {
		return l, fmt.Errorf("unable to get mpirun arguments: %s", err)
	}
	l.CmdArgs = append(l.CmdArgs, mpirunArgs...)

	newPath := getEnvPath(j.HostCfg)
	newLDPath := getEnvLDPath(j.HostCfg)
	log.Printf("-> PATH=%s", newPath)
	log.Printf("-> LD_LIBRARY_PATH=%s\n", newLDPath)
	log.Printf("Using %s as PATH\n", newPath)
	log.Printf("Using %s as LD_LIBRARY_PATH\n", newLDPath)
	l.Env = append([]string{"LD_LIBRARY_PATH=" + newLDPath}, os.Environ()...)
	l.Env = append([]string{"PATH=" + newPath}, os.Environ()...)

	j.GetOutput = NativeGetOutput
	j.GetError = NativeGetError

	return l, nil
}

// LoadNative is the function used by our job management framework to figure out if mpirun should be used directly.
// The native component is the default job manager. If application, the function returns a structure with all the
// "function pointers" to correctly use the native job manager.
func LoadNative() (bool, JM) {
	var jm JM
	jm.ID = NativeID
	jm.Get = NativeGetConfig
	jm.Set = NativeSetConfig
	jm.Submit = NativeSubmit

	// This is the default job manager, i.e., mpirun so we do not check anything, just return this component.
	// If the component is selected and mpirun not correctly installed, the framework will pick it up later.
	return true, jm
}
