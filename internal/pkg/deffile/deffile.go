// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package deffile

import (
	"fmt"
	"os"
	"path"
	"strings"

	util "github.com/sylabs/singularity-mpi/internal/pkg/util/file"
)

const (
	distroCodenameTag = "DISTROCODENAME"
)

type DefFileData struct {
	// Distro is the linux distribution identifier to be used in the definition file
	Distro string
	// MpiImplm is the MPI implementation ID (e.g., OMPI, MPICH)
	MpiImplm string
	// MpiVersion is the Version of the MPI implementation to use
	MpiVersion string
	// MPIInstallDir is the path to the directory in the image where MPI is installed
	MPIInstallDir string
	// MpiURL is the URL to use to download MPI while building the container
	MpiURL string
	// AppDir is the path to the directory within the container where the application is installed
	//AppDir string
}

func setMPIInstallDir(mpiImplm string, mpiVersion string) string {
	return mpiImplm + "-" + mpiVersion
}

// AddLabels adds a set of labels to the definition file.
func AddLabels(f *os.File, deffile *DefFileData) error {
	linuxDistro := "ubuntu"    // todo: do not hardcode
	appName := "NetPIPE-5.1.4" // todo: do not hardcode

	_, err := f.WriteString("%labels\n")
	if err != nil {
		return err
	}

	_, err = f.WriteString("\tLinux_distribution " + linuxDistro + "\n")
	if err != nil {
		return err
	}

	_, err = f.WriteString("\tLinux_version " + deffile.Distro + "\n")
	if err != nil {
		return err
	}

	_, err = f.WriteString("\tMPI_Implementation " + deffile.MpiImplm + "\n")
	if err != nil {
		return err
	}

	_, err = f.WriteString("\tMPI_Version " + deffile.MpiVersion + "\n")
	if err != nil {
		return err
	}

	deffile.MPIInstallDir = setMPIInstallDir(deffile.MpiImplm, deffile.MpiVersion)
	_, err = f.WriteString("\tMPI_Directory /opt/" + deffile.MPIInstallDir + "\n")
	if err != nil {
		return err
	}

	_, err = f.WriteString("\tApplication " + appName + "\n")
	if err != nil {
		return err
	}

	/*
		_, err = f.WriteString("\tApp_Directory /opt/" + deffile.AppDir + "\n")
		if err != nil {
			return err
		}
	*/

	_, err = f.WriteString("\n")
	if err != nil {
		return err
	}

	return nil
}

// AddBoostrap adds all the data to the definition file related to bootstrapping
func AddBootstrap(f *os.File, deffile *DefFileData) error {
	_, err := f.WriteString("Bootstrap: docker\nFrom: " + deffile.Distro + "\n\n")
	if err != nil {
		return fmt.Errorf("failed to add bootstrap section to definition file: %s", err)
	}

	return nil
}

func AddMPIInstall(f *os.File, deffile *DefFileData) error {
	ompitarball := path.Base(deffile.MpiURL)
	tarballFormat := util.DetectTarballFormat(ompitarball)
	tarArgs := util.GetTarArgs(tarballFormat)
	_, err := f.WriteString("\tcd /tmp/mpi && wget $MPI_URL && tar " + tarArgs + " " + ompitarball + "\n")
	if err != nil {
		return err
	}

	_, err = f.WriteString("\tcd /tmp/mpi/" + deffile.MpiImplm + "-$MPI_VERSION && ./configure --prefix=$MPI_DIR && make -j8 install\n")
	if err != nil {
		return err
	}

	_, err = f.WriteString("\texport PATH=$MPI_DIR/bin:$PATH\n\texport LD_LIBRARY_PATH=$MPI_DIR/lib:$LD_LIBRARY_PATH\n\texport MANPATH=$MPI_DIR/share/man:$MANPATH\n\n")
	if err != nil {
		return err
	}

	return nil
}

func AddMPIEnv(f *os.File, deffile *DefFileData) error {
	deffile.MPIInstallDir = setMPIInstallDir(deffile.MpiImplm, deffile.MpiVersion)

	_, err := f.WriteString("%environment\n\tMPI_DIR=/opt/" + deffile.MPIInstallDir + "\n")
	if err != nil {
		return err
	}

	_, err = f.WriteString("\texport MPI_DIR\n\texport SINGULARITY_MPI_DIR=$MPI_DIR\n\texport SINGULARITYENV_APPEND_PATH=$MPI_DIR/bin\n\texport SINGULARITYENV_APPEND_LD_LIBRARY_PATH=$MPI_DIR/lib\n\n")
	if err != nil {
		return err
	}

	_, err = f.WriteString("%post\n\tapt-get update && apt-get install -y wget git bash gcc gfortran g++ make file\n\n")
	if err != nil {
		return err
	}

	_, err = f.WriteString("\texport MPI_VERSION=" + deffile.MpiVersion + "\n\texport MPI_URL=\"" + deffile.MpiURL + "\"\n")
	if err != nil {
		return err
	}

	_, err = f.WriteString("\texport MPI_DIR=/opt/" + deffile.MPIInstallDir + "\n")
	if err != nil {
		return err
	}

	_, err = f.WriteString("\tmkdir -p /tmp/mpi\n\tmkdir -p /opt\n\n")
	if err != nil {
		return err
	}

	return nil
}

// UpdateDefFileDistroCodename replace the tag for the distro codename in a definition file by the actual target distro codename
func UpdateDistroCodename(data, distro string) string {
	return strings.Replace(data, distroCodenameTag, distro, -1)
}
