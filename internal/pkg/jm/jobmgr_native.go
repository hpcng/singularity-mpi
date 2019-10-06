// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package jm

import (
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/sylabs/singularity-mpi/internal/pkg/sys"

	"github.com/sylabs/singularity-mpi/internal/pkg/mpi"
)

type Native struct {
}

func NativeSetConfig() error {
	return nil
}

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

func NativeSubmit(j *Job, sysCfg *sys.Config) (Launcher, error) {
	var l Launcher

	l.Cmd = "mpirun"
	if j.NP > 0 {
		l.CmdArgs = append(l.CmdArgs, "-np")
		l.CmdArgs = append(l.CmdArgs, strconv.FormatInt(j.NP, 10))
	}

	newPath := getEnvPath(j.HostCfg)
	newLDPath := getEnvLDPath(j.HostCfg)
	log.Printf("-> PATH=%s", newPath)
	log.Printf("-> LD_LIBRARY_PATH=%s\n", newLDPath)
	l.Env = append([]string{"LD_LIBRARY_PATH=" + newLDPath}, os.Environ()...)
	l.Env = append([]string{"PATH=" + newPath}, os.Environ()...)

	return l, nil
}

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
