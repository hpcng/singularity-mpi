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
	"path/filepath"
	"strings"

	"github.com/sylabs/singularity-mpi/internal/pkg/app"
	"github.com/sylabs/singularity-mpi/internal/pkg/buildenv"
	"github.com/sylabs/singularity-mpi/internal/pkg/container"
	"github.com/sylabs/singularity-mpi/internal/pkg/implem"
	"github.com/sylabs/singularity-mpi/internal/pkg/ldd"
	"github.com/sylabs/singularity-mpi/internal/pkg/sys"
	util "github.com/sylabs/singularity-mpi/internal/pkg/util/file"
)

const (
	distroCodenameTag = "DISTROCODENAME"
)

// TemplateTags gathers all the data related to a given template
type TemplateTags struct {
	// Verion is the version of the MPI implementation tag
	Version string
	// Tarball is the tag used to refer to the MPI implementation tarball
	Tarball string
	// URL is the tag used to refer to the URL to be used to download MPI
	URL string
	// Dir is the tag to be used to refer to the directory where MPI is installed
	Dir string // todo: Should be removed
	// InstallConfFile is the tag used to specify where the installation configuration file is assumed to be in the image
	InstallConffile string
	// UninstallConfFile is the tag used to specify where the uninstallation configuration file is assumed to be in the image
	UninstallConffile string
	// Ifnet is the tag referring to the network interface to be used
	Ifnet string
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

	// Model specifies the model to follow for MPI inside the container
	Model string
}

func setMPIInstallDir(mpiImplm string, mpiVersion string) string {
	return mpiImplm + "-" + mpiVersion
}

// AddLabels adds a set of labels to the definition file.
func AddLabels(f *os.File, app *app.Info, deffile *DefFileData) error {
	// Some sanity checks
	if deffile.InternalEnv == nil {
		return fmt.Errorf("invalid parameter(s)")
	}

	linuxDistro := "ubuntu" // todo: do not hardcode

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

	_, err = f.WriteString("\tModel " + deffile.Model + "\n")
	if err != nil {
		return err
	}

	_, err = f.WriteString("\tApplication " + app.Name + "\n")
	if err != nil {
		return err
	}

	if deffile.Model == container.BindModel {
		// When dealing with the bind model, we explicitly copy the binary in /opt
		_, err = f.WriteString("\tApp_exe /opt/" + app.BinName + "\n")
		if err != nil {
			return err
		}
	} else {
		// When dealing with the hybrid model, we do not really know the path to the executable
		// so we rely on the data in the app.Config structure (from user input)
		_, err = f.WriteString("\tApp_exe " + app.BinPath + "\n")
		if err != nil {
			return err
		}
	}

	_, err = f.WriteString("\n")
	if err != nil {
		return err
	}

	return nil
}

func addDockerBootstrap(f *os.File, deffile *DefFileData) error {
	_, err := f.WriteString("Bootstrap: docker\nFrom: " + deffile.Distro + "\n\n")
	if err != nil {
		return fmt.Errorf("failed to add bootstrap section to definition file: %s", err)
	}

	return nil
}

func addDebootstrapBootstrap(f *os.File, deffile *DefFileData) error {
	distro := deffile.Distro
	// if the distro ID is of the form <distro>:<version>, e.g., ubuntu:disco, we extract the version
	if strings.Contains(distro, ":") {
		tokens := strings.Split(distro, ":")
		distro = tokens[1]
	}
	// todo: do not hardcode the mirror URL
	_, err := f.WriteString("Bootstrap: debootstrap\nOSVersion: " + distro + "\nMirrorURL: http://us.archive.ubuntu.com/ubuntu/\n\n")
	if err != nil {
		return fmt.Errorf("failed to add bootstrap section to definition file: %s", err)
	}

	return nil
}

func addDistroInit(f *os.File, distro string) error {
	codename := distro
	// if the distro ID is of the form <distro>:<version>, e.g., ubuntu:disco, we extract the version
	if strings.Contains(codename, ":") {
		tokens := strings.Split(codename, ":")
		codename = tokens[1]
	}

	_, err := f.WriteString("%post\n\tapt-get update && apt-get install -y wget git bash gcc gfortran g++ make file software-properties-common\n\n")
	if err != nil {
		return err
	}

	_, err = f.WriteString("\tadd-apt-repository universe\n")
	if err != nil {
		return fmt.Errorf("failed to add ubuntu initialization code to definition file: %s", err)
	}
	_, err = f.WriteString("\tadd-apt-repository multiverse\n")
	if err != nil {
		return fmt.Errorf("failed to add ubuntu initialization code to definition file: %s", err)
	}
	_, err = f.WriteString("\tapt-get update\n\n")
	if err != nil {
		return fmt.Errorf("failed to add ubuntu initialization code to definition file: %s", err)
	}

	return nil
}

// AddBoostrap adds all the data to the definition file related to bootstrapping
func AddBootstrap(f *os.File, deffile *DefFileData) error {
	return addDebootstrapBootstrap(f, deffile)
}

// AddMPIInstall adds all the data to the definition file related to the installation of MPI
func AddMPIInstall(f *os.File, deffile *DefFileData) error {
	_, err := f.WriteString("\texport MPI_VERSION=" + deffile.MpiImplm.Version + "\n\texport MPI_URL=\"" + deffile.MpiImplm.URL + "\"\n")
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

	mpitarball := path.Base(deffile.MpiImplm.URL)
	tarballFormat := util.DetectTarballFormat(mpitarball)
	tarArgs := util.GetTarArgs(tarballFormat)
	_, err = f.WriteString("\tcd /tmp/mpi && wget $MPI_URL && tar " + tarArgs + " " + mpitarball + "\n")
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

func createFilesSection(f *os.File, app *app.Info, data *DefFileData, sysCfg *sys.Config) error {
	// In the context of the bind model, we compile the application on the host and copy it over
	if data.Model == container.BindModel {
		// This means this is most certainly a file
		_, err := f.WriteString("%files\n")
		if err != nil {
			return fmt.Errorf("failed to write to definition file: %s", err)
		}
		_, err = f.WriteString("\t" + app.BinPath + " /opt\n\n")
		if err != nil {
			return fmt.Errorf("failed to write to definition file: %s", err)
		}
	}

	// If the application is a file that we compiled, we copy it into the container
	if util.DetectTarballFormat(app.Source) == util.UnknownFormat {
		// This means this is most certainly a file
		_, err := f.WriteString("%files\n")
		if err != nil {
			return fmt.Errorf("failed to write to definition file: %s", err)
		}

		src := strings.Replace(app.Source, "file://", "", 1)
		_, err = f.WriteString("\t" + src + " /opt\n\n")
		if err != nil {
			return fmt.Errorf("failed to write to definition file: %s", err)
		}
	}

	return nil
}

func createBootstrapSection(f *os.File, data *DefFileData, sysCfg *sys.Config) error {
	_, err := f.WriteString("Bootstrap: docker\n")
	if err != nil {
		return fmt.Errorf("failed to write to definition file: %s", err)
	}

	_, err = f.WriteString("From: ubuntu:DISTROCODENAME\n\n")
	if err != nil {
		return fmt.Errorf("failed to write to definition file: %s", err)
	}

	return nil
}

func addAppInstall(f *os.File, app *app.Info, data *DefFileData) error {
	installCmd := "make install"
	if app.InstallCmd != "" {
		installCmd = app.InstallCmd
	}

	urlType := util.DetectURLType(app.Source)
	switch urlType {
	case util.GitURL:
		srcDir := path.Base(app.Source)
		srcDir = strings.Replace(srcDir, ".git", "", -1)
		_, err := f.WriteString("\tcd /opt/$APPDIR" + " && " + installCmd + "\n")
		if err != nil {
			return fmt.Errorf("failed to write to definition file: %s", err)
		}
	case util.FileURL:
		containerSrcPath := filepath.Join(data.InternalEnv.SrcDir, filepath.Base(app.Source))
		_, err := f.WriteString("\tcd /opt/$APPDIR && mpicc -o " + app.BinPath + " " + containerSrcPath + "\n")
		if err != nil {
			return fmt.Errorf("failed to write to definition file: %s", err)
		}
	case util.HttpURL:
		_, err := f.WriteString("\tcd /opt/$APPDIR && " + installCmd)
		if err != nil {
			return fmt.Errorf("failed to write to definition file: %s", err)
		}
	}

	// todo: Clean up
	/*
		_, err := f.WriteString("\trm -rf /opt/" + app.tarball + "\n")
		if err != nil {
			return fmt.Errorf("failed to add cleanup section: %s", err)
		}
	*/

	return nil
}

func addMPICleanup(f *os.File, app *app.Info, data *DefFileData) error {
	// todo
	return nil
}

func addDetectAppDir(f *os.File, app *app.Info, data *DefFileData) error {
	_, err := f.WriteString("\tAPPDIR=`ls -l /opt | egrep '^d' | head -1 | awk '{print $9}'`\n\n")
	if err != nil {
		return fmt.Errorf("failed to add app env info: %s", err)
	}

	return nil
}

// addAppDownload adds the code to the definition file to download an application
//
// Note that the function assumes that /opt is empty when called so it needs to be
// called before downloading/installing anything else.
func addAppDownload(f *os.File, app *app.Info, data *DefFileData) error {
	urlType := util.DetectURLType(app.Source)
	switch urlType {
	case util.GitURL:
		srcDir := path.Base(app.Source)
		srcDir = strings.Replace(srcDir, ".git", "", -1)
		_, err := f.WriteString("\tcd /opt && git clone " + app.Source + "\n")
		if err != nil {
			return fmt.Errorf("failed to write to definition file: %s", err)
		}

		err = addDetectAppDir(f, app, data)
		if err != nil {
			return fmt.Errorf("failed to add code to get the directory of the app to the definition file: %s", err)
		}
	case util.HttpURL:
		format := util.DetectTarballFormat(app.Source)
		tarArgs := util.GetTarArgs(format)
		_, err := f.WriteString("\tcd /opt && wget " + app.Source + " && tar " + tarArgs + " " + path.Base(app.Source) + "\n")
		if err != nil {
			return fmt.Errorf("failed to write to definition file: %s", err)
		}

		err = addDetectAppDir(f, app, data)
		if err != nil {
			return fmt.Errorf("failed to add code to get the directory of the app to the definition file: %s", err)
		}
	}

	return nil
}

func addDependencies(f *os.File, list []string) error {
	// todo: do not assume debian
	_, err := f.WriteString("\tapt install -y " + strings.Join(list, " ") + "\n")
	if err != nil {
		return fmt.Errorf("failed to add cleanup section: %s", err)
	}

	// todo: find a better way to deal with symlinks that are necessary for cross-distro compatility
	_, err = f.WriteString("\tln -s /usr/lib/x86_64-linux-gnu/libosmcomp.so /usr/lib/x86_64-linux-gnu/libosmcomp.so.3\n")
	if err != nil {
		return fmt.Errorf("failed to add cleanup section: %s", err)
	}

	_, err = f.WriteString("\tldconfig\n")
	if err != nil {
		return fmt.Errorf("failed to add cleanup section: %s", err)
	}

	return nil
}

func addCleanUp(f *os.File) error {
	// todo: do not assume debian
	_, err := f.WriteString("\tapt-get clean\n")
	if err != nil {
		return fmt.Errorf("failed to add cleanup section: %s", err)
	}

	return nil
}

// CreateBindDefFile creates a definition file for a given bybrid-based configuration.
func CreateHybridDefFile(app *app.Info, data *DefFileData, sysCfg *sys.Config) error {
	// Some sanity checks
	if data.Path == "" {
		return fmt.Errorf("invalid parameter(s)")
	}

	log.Printf("- Defintion file is %s\n", data.Path)
	f, err := os.Create(data.Path)
	if err != nil {
		return fmt.Errorf("failed to create %s: %s", data.Path, err)
	}

	err = AddBootstrap(f, data)
	if err != nil {
		return fmt.Errorf("failed to create the bootstrap section of the definition file: %s", err)
	}

	err = AddLabels(f, app, data)
	if err != nil {
		return fmt.Errorf("failed to create the files section of the definition file: %s", err)
	}

	if util.DetectURLType(app.Source) == util.FileURL {
		err = createFilesSection(f, app, data, sysCfg)
		if err != nil {
			return fmt.Errorf("failed to create the files section of the definition file: %s", err)
		}
	}

	err = AddMPIEnv(f, data)
	if err != nil {
		return fmt.Errorf("failed to create the environment section of the definition file: %s", err)
	}

	err = addDistroInit(f, data.Distro)
	if err != nil {
		return fmt.Errorf("failed to add the code initializing the distro: %s", err)
	}

	err = addAppDownload(f, app, data)
	if err != nil {
		return fmt.Errorf("failed to add the section to download the app: %s", err)
	}

	err = AddMPIInstall(f, data)
	if err != nil {
		return fmt.Errorf("failed to create the post section of the definition file: %s", err)
	}

	err = addAppInstall(f, app, data)
	if err != nil {
		return fmt.Errorf("failed to create the post section of the definition file: %s", err)
	}

	err = addMPICleanup(f, app, data)
	if err != nil {
		return fmt.Errorf("failed to add code to cleanup MPI files: %s", err)
	}

	f.Close()

	return nil
}

// CreateBindDefFile creates a definition file for a given bind-based configuration.
//
// Note that the application must have been compiled on the host prior to calling this function.
// All data to handle the application once compiled is available in app.
func CreateBindDefFile(app *app.Info, data *DefFileData, sysCfg *sys.Config) error {
	// Some sanity checks
	if data.Path == "" {
		return fmt.Errorf("invalid parameter(s)")
	}

	f, err := os.Create(data.Path)
	if err != nil {
		return fmt.Errorf("failed to create %s: %s", data.Path, err)
	}

	// At this point the application already has been installed on the host.
	// Detect the list of dependencies required for the binary that we are about to copy in
	// the container.
	lddMod, err := ldd.Detect()
	if err != nil {
		return fmt.Errorf("failed to load a workable ldd module")
	}
	log.Printf("* Getting dependencies for %s\n", app.BinPath)
	pkgs := lddMod.GetPackageDependenciesForFile(app.BinPath)

	// Add some packages we always want in the image
	// todo: find a way to do this in a clean and maintainable way
	pkgs = append(pkgs, "libc-bin")
	pkgs = append(pkgs, "libopensm-dev")
	pkgs = append(pkgs, "librdmacm-dev")
	pkgs = append(pkgs, "librdmacm1")
	pkgs = append(pkgs, "kmod")
	pkgs = append(pkgs, "libmlx4-1")
	pkgs = append(pkgs, "libibverbs-dev")
	pkgs = append(pkgs, "libibverbs1")
	pkgs = append(pkgs, "libnl-3-dev")
	pkgs = append(pkgs, "infiniband-diags")
	pkgs = append(pkgs, "ibverbs-utils")

	err = AddBootstrap(f, data)
	if err != nil {
		return fmt.Errorf("failed to create the bootstrap section of the definition file: %s", err)
	}

	err = AddLabels(f, app, data)
	if err != nil {
		return fmt.Errorf("failed to create the files section of the definition file: %s", err)
	}

	// This will copy the application that we compiled in the container
	err = createFilesSection(f, app, data, sysCfg)
	if err != nil {
		return fmt.Errorf("failed to create the files section of the definition file: %s", err)
	}

	err = AddMPIEnv(f, data)
	if err != nil {
		return fmt.Errorf("failed to create the environment section of the definition file: %s", err)
	}

	err = addDistroInit(f, data.Distro)
	if err != nil {
		return fmt.Errorf("failed to add the code initializing the distro: %s", err)
	}

	err = addDependencies(f, pkgs)
	if err != nil {
		return fmt.Errorf("failed to add package dependencies to the definition file: %s", err)
	}

	err = addCleanUp(f)
	if err != nil {
		return fmt.Errorf("failed to add code to clean up: %s", err)
	}

	f.Close()

	return nil
}

// Backup a definition file based on a build environment (copy the file from the build directory
// to the install directory)
func (d *DefFileData) Backup(env *buildenv.Info) error {
	defFileName := filepath.Base(d.Path)
	backupFile := filepath.Join(env.InstallDir, defFileName)
	if d.Path != backupFile {
		log.Printf("-> Backing up %s to %s", d.Path, backupFile)
		err := util.CopyFile(d.Path, backupFile)
		if err != nil {
			return fmt.Errorf("error while backing up %s to %s", d.Path, backupFile)
		}
	}

	return nil
}
