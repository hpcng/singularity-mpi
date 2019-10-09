// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package mpi

import (
	"path/filepath"

	"github.com/sylabs/singularity-mpi/internal/pkg/app"
	"github.com/sylabs/singularity-mpi/internal/pkg/buildenv"
	"github.com/sylabs/singularity-mpi/internal/pkg/container"
	"github.com/sylabs/singularity-mpi/internal/pkg/impi"
	"github.com/sylabs/singularity-mpi/internal/pkg/implem"
	"github.com/sylabs/singularity-mpi/internal/pkg/openmpi"
	"github.com/sylabs/singularity-mpi/internal/pkg/sys"
)

//type GetExtraMpirunArgsFn func(*Config, *sys.Config, jobmgr *jm.JM) []string

/*
type GetConfigureExtraArgsFn func(net *network.Config) []string
*/

/*
type Implementation struct {

		GetExtraMpirunArgs GetExtraMpirunArgsFn

}
*/

// Config represents a configuration of MPI for a target platform
type Config struct {
	// Implem gathers information about the MPI implementation to use
	Implem implem.Info
	/*
		// tarball is the file name of the source code file for the target MPI implementation
		tarball string
	*/
	/*
		// srcDir is the path to the directory where the source has been untared
		srcDir string
		// BuildDir is the path to the directory where to compile
		BuildDir string
		// InstallDirectory is the path to the directory where to install the compiled software
		InstallDir string
	*/

	Buildenv buildenv.Info

	Container container.Config

	/*
		// Deffile is the path to the definition file used to create MPI container
		DefFile string
		// ContainerName is the name of the container's image file
		ContainerName string
		// ContainerPath is the Path to the container image
		ContainerPath string
		// Distro is the ID of the Linux distro to use in the container
		Distro string
		// ImageURL is the URL to use to pull an image
		ImageURL string
	*/

	//	app app.Info // was TestPath
}

/*
type compileConfig struct {
	mpiVersionTag string
	mpiURLTag     string
	mpiTarballTag string
}
*/

// GetPathToMpirun returns the path to mpirun based a configuration of MPI
func GetPathToMpirun(mpiCfg *implem.Info, env *buildenv.Info) string {
	// Intel MPI is installing the binaries and libraries in a quite complex setup
	if mpiCfg.ID == implem.IMPI {
		return impi.GetPathToMpirun(env)
	}

	return filepath.Join(env.InstallDir, "bin", "mpirun")
}

/*
// GetMpirunArgs returns the arguments required by a mpirun
func GetMpirunArgs(myHostMPICfg *implem.Info, app *app.Info, container *container.Config, sysCfg *sys.Config) ([]string, error) {
	args := []string{"singularity", "exec", container.Path, app.BinPath}

	// We really do not want to do this but MPICH is being picky about args so for now, it will do the job.
	switch myHostMPICfg.ID {
	case implem.IMPI:
		extraArgs := impi.GetExtraMpirunArgs(myHostMPICfg, sysCfg)
	case implem.OMPI:
		extraArgs := openmpi.GetExtraMpirunArgs(sysCfg)
	}

	if len(extraArgs) > 0 {
		args = append(extraArgs, args...)
	}

	return args, nil
}
*/

/*
 */

/*
func updateDeffile(myCfg *Config, sysCfg *sys.Config, compileCfg *compileConfig) error {
	// Sanity checks
	if myCfg.Implem.Version == "" || myCfg.BuildDir == "" || myCfg.Implem.URL == "" ||
		myCfg.DefFile == "" || compileCfg.mpiVersionTag == "" ||
		compileCfg.mpiURLTag == "" || compileCfg.mpiTarballTag == "" ||
		sysCfg.TargetUbuntuDistro == "" {
		return fmt.Errorf("invalid parameter(s)")
	}

	if myCfg.tarball == "" {
		myCfg.tarball = path.Base(myCfg.Implem.URL)
	}

	data, err := ioutil.ReadFile(myCfg.DefFile)
	if err != nil {
		return fmt.Errorf("failed to read %s: %s", myCfg.DefFile, err)
	}

	var tarArgs string
	format := util.DetectTarballFormat(myCfg.tarball)
	switch format {
	case util.FormatBZ2:
		tarArgs = "-xjf"
	case util.FormatGZ:
		tarArgs = "-xzf"
	case util.FormatTAR:
		tarArgs = "-xf"
	default:
		return fmt.Errorf("un-supported tarball format for %s", myCfg.tarball)
	}

	if sysCfg.Debug {
		log.Printf("--> Replacing %s with %s", compileCfg.mpiVersionTag, myCfg.Implem.Version)
		log.Printf("--> Replacing %s with %s", compileCfg.mpiURLTag, myCfg.Implem.URL)
		log.Printf("--> Replacing %s with %s", compileCfg.mpiTarballTag, myCfg.tarball)
		log.Printf("--> Replacing TARARGS with %s", tarArgs)
	}

	content := string(data)
	content = strings.Replace(content, compileCfg.mpiVersionTag, myCfg.Implem.Version, -1)
	content = strings.Replace(content, compileCfg.mpiURLTag, myCfg.Implem.URL, -1)
	content = strings.Replace(content, compileCfg.mpiTarballTag, myCfg.tarball, -1)
	content = strings.Replace(content, "TARARGS", tarArgs, -1)
	content = deffile.UpdateDistroCodename(content, sysCfg.TargetUbuntuDistro)

	err = ioutil.WriteFile(myCfg.DefFile, []byte(content), 0)
	if err != nil {
		return fmt.Errorf("failed to write file %s: %s", myCfg.DefFile, err)
	}

	return nil
}
*/

/*
func Detect(cfg *Config) *implem.Info {
	loaded, implem := LoadOpenMPI(cfg)
	if loaded {
		return &implem
	}

	loaded, implem = LoadMPICH(cfg)
	if loaded {
		return &implem
	}

	loaded, implem = LoadIMPI(cfg)
	if loaded {
		return &implem
	}

	return &implem
}
*/

// GetMpirunArgs returns the arguments required by a mpirun
func GetMpirunArgs(myHostMPICfg *implem.Info, app *app.Info, container *container.Config, sysCfg *sys.Config) ([]string, error) {
	args := []string{"singularity", "exec", container.Path, app.BinPath}
	var extraArgs []string

	// We really do not want to do this but MPICH is being picky about args so for now, it will do the job.
	switch myHostMPICfg.ID {
	/*
		case implem.IMPI:
			extraArgs := impi.GetExtraMpirunArgs(myHostMPICfg, sysCfg)
	*/
	case implem.OMPI:
		extraArgs = append(extraArgs, openmpi.GetExtraMpirunArgs(sysCfg)...)
	}

	if len(extraArgs) > 0 {
		args = append(extraArgs, args...)
	}

	return args, nil
}
