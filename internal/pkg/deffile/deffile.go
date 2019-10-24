// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package deffile

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strings"

	"github.com/sylabs/singularity-mpi/internal/pkg/buildenv"
	"github.com/sylabs/singularity-mpi/internal/pkg/implem"
	"github.com/sylabs/singularity-mpi/internal/pkg/sys"
	util "github.com/sylabs/singularity-mpi/internal/pkg/util/file"
)

const (
	distroCodenameTag = "DISTROCODENAME"
)

// TemplateTags gathers all the data related to a given template
type TemplateTags struct {
	// Verion is the version of the MPI implementation tag
	Version           string
	// Tarball is the tag used to refer to the MPI implementation tarball
	Tarball           string
	// URL is the tag used to refer to the URL to be used to download MPI
	URL               string
	// Dir is the tag to be used to refer to the directory where MPI is installed
	Dir               string // todo: Should be removed
	// InstallConfFile is the tag used to specify where the installation configuration file is assumed to be in the image
	InstallConffile   string
	// UninstallConfFile is the tag used to specify where the uninstallation configuration file is assumed to be in the image
	UninstallConffile string
	// Ifnet is the tag referring to the network interface to be used
	Ifnet             string
}

// DefFileData is all the data associated to a definition file
type DefFileData struct {
	// Path is the path to the definition file
	Path string
	// Distro is the linux distribution identifier to be used in the definition file
	Distro string
	// MpiImplm is the MPI implementation ID (e.g., OMPI, MPICH)
	MpiImplm *implem.Info
	// Tags are the keys used in the template file for the MPI to use
	Tags TemplateTags
	// InternalEnv represents the build environment to use in the definition file
	InternalEnv *buildenv.Info

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

	_, err = f.WriteString("\tMPI_Implementation " + deffile.MpiImplm.ID + "\n")
	if err != nil {
		return err
	}

	_, err = f.WriteString("\tMPI_Version " + deffile.MpiImplm.Version + "\n")
	if err != nil {
		return err
	}

	deffile.InternalEnv.InstallDir = setMPIInstallDir(deffile.MpiImplm.ID, deffile.MpiImplm.Version)
	_, err = f.WriteString("\tMPI_Directory /opt/" + deffile.InternalEnv.InstallDir + "\n")
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

// AddMPIInstall adds all the data to the definition file related to the installation of MPI
func AddMPIInstall(f *os.File, deffile *DefFileData) error {
	mpitarball := path.Base(deffile.MpiImplm.URL)
	tarballFormat := util.DetectTarballFormat(mpitarball)
	tarArgs := util.GetTarArgs(tarballFormat)
	_, err := f.WriteString("\tcd /tmp/mpi && wget $MPI_URL && tar " + tarArgs + " " + mpitarball + "\n")
	if err != nil {
		return err
	}

	_, err = f.WriteString("\tcd /tmp/mpi/" + deffile.MpiImplm.ID + "-$MPI_VERSION && ./configure --prefix=$MPI_DIR && make -j8 install\n")
	if err != nil {
		return err
	}

	_, err = f.WriteString("\texport PATH=$MPI_DIR/bin:$PATH\n\texport LD_LIBRARY_PATH=$MPI_DIR/lib:$LD_LIBRARY_PATH\n\texport MANPATH=$MPI_DIR/share/man:$MANPATH\n\n")
	if err != nil {
		return err
	}

	return nil
}

// AddMPIEnv adds all the data to the definition file to specify the environment of the MPI installation in the container
func AddMPIEnv(f *os.File, deffile *DefFileData) error {
	deffile.InternalEnv.InstallDir = setMPIInstallDir(deffile.MpiImplm.ID, deffile.MpiImplm.Version)

	_, err := f.WriteString("%environment\n\tMPI_DIR=/opt/" + deffile.InternalEnv.InstallDir + "\n")
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

	_, err = f.WriteString("\texport MPI_VERSION=" + deffile.MpiImplm.Version + "\n\texport MPI_URL=\"" + deffile.MpiImplm.URL + "\"\n")
	if err != nil {
		return err
	}

	_, err = f.WriteString("\texport MPI_DIR=/opt/" + deffile.InternalEnv.InstallDir + "\n")
	if err != nil {
		return err
	}

	_, err = f.WriteString("\tmkdir -p /tmp/mpi\n\tmkdir -p /opt\n\n")
	if err != nil {
		return err
	}

	return nil
}

// UpdateDefFileDistroCodename replaces the tag for the distro codename in a definition file by the actual target distro codename
func UpdateDistroCodename(data, distro string) string {
	return strings.Replace(data, distroCodenameTag, distro, -1)
}

// UpdateDeffileTemplate update a template file and create a usable definition file
func UpdateDeffileTemplate(data DefFileData, sysCfg *sys.Config) error {
	// Sanity checks
	if data.MpiImplm.Version == "" || data.MpiImplm.URL == "" ||
		data.Path == "" || data.Tags.Version == "" ||
		data.Tags.URL == "" || data.Tags.Tarball == "" ||
		data.Distro == "" {
		return fmt.Errorf("invalid parameter(s)")
	}

	tarball := path.Base(data.MpiImplm.URL)
	d, err := ioutil.ReadFile(data.Path)
	if err != nil {
		return fmt.Errorf("failed to read %s: %s", data.Path, err)
	}

	var tarArgs string
	format := util.DetectTarballFormat(tarball)
	switch format {
	case util.FormatBZ2:
		tarArgs = "-xjf"
	case util.FormatGZ:
		tarArgs = "-xzf"
	case util.FormatTAR:
		tarArgs = "-xf"
	default:
		return fmt.Errorf("un-supported tarball format for %s", tarball)
	}

	if sysCfg.Debug {
		log.Printf("--> Replacing %s with %s", data.Tags.Version, data.MpiImplm.Version)
		log.Printf("--> Replacing %s with %s", data.Tags.URL, data.MpiImplm.URL)
		log.Printf("--> Replacing %s with %s", data.Tags.Tarball, tarball)
		log.Printf("--> Replacing TARARGS with %s", tarArgs)
	}

	content := string(d)
	content = strings.Replace(content, data.Tags.Version, data.MpiImplm.Version, -1)
	content = strings.Replace(content, data.Tags.URL, data.MpiImplm.URL, -1)
	content = strings.Replace(content, data.Tags.Tarball, tarball, -1)
	content = strings.Replace(content, "TARARGS", tarArgs, -1)
	content = UpdateDistroCodename(content, data.Distro)

	err = ioutil.WriteFile(data.Path, []byte(content), 0)
	if err != nil {
		return fmt.Errorf("failed to write file %s: %s", data.Path, err)
	}

	return nil
}
