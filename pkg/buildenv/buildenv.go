// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

/*
 * buildenv is a package that provides all the capabilities to deal with a build environment,
 * from defining where the software should be compiled and install, to the actual configuration,
 * compilation and installation of software.
 */
package buildenv

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/gvallee/go_util/pkg/util"
	"github.com/gvallee/kv/pkg/kv"
	"github.com/sylabs/singularity-mpi/internal/pkg/persistent"
	"github.com/sylabs/singularity-mpi/pkg/implem"
	"github.com/sylabs/singularity-mpi/pkg/syexec"
	"github.com/sylabs/singularity-mpi/pkg/sys"
)

// SoftwarePackage gathers all the information related to the software package to prepare in the build environment
type SoftwarePackage struct {
	// Name is the name with which the software package is recognized
	Name string

	// URL is the source of the software
	URL string

	// InstallCmd is the command used to install the software
	InstallCmd string

	tarball string
}

// Info gathers the details of the build environment
type Info struct {
	// SrcPath is the path to the downloaded tarball
	SrcPath string

	// SrcDir is the directory where the source code is
	SrcDir string

	// ScratchDir is the directory where we can store temporary data
	ScratchDir string

	// InstallDir is the directory where the software needs to be installed
	InstallDir string

	// BuildDir is the directory where the software is built
	BuildDir string

	// Env is the environment to use with the build environment
	Env []string
}

// Unpack extracts the source code from a package/tarball/zip file.
func (env *Info) Unpack() error {
	log.Println("- Unpacking software...")

	// Sanity checks
	if env.SrcPath == "" || env.BuildDir == "" {
		return fmt.Errorf("invalid parameter(s)")
	}

	// Figure out the extension of the tarball
	if util.IsDir(env.SrcPath) {
		// If we point to a directory, it is something like a Git checkout so nothing to do
		log.Printf("%s does not seem to need to be unpacked, skipping...", env.SrcPath)
		return nil
	}

	format := util.DetectTarballFormat(env.SrcPath)
	if format == "" {
		// A typical use case here is a single file that just needs to be compiled
		log.Printf("%s does not seem to need to be unpacked, skipping...", env.SrcPath)
		env.SrcDir = env.BuildDir
		return nil
	}

	// At the moment we always assume we have to use the tar command
	// (and it is a fair assumption for our current context)
	tarPath, err := exec.LookPath("tar")
	if err != nil {
		return fmt.Errorf("tar is not available: %s", err)
	}

	tarArg := util.GetTarArgs(format)
	if tarArg == "" {
		return fmt.Errorf("unsupported format: %s", format)
	}

	// Untar the package
	log.Printf("-> Executing from %s: %s %s %s \n", env.BuildDir, tarPath, tarArg, env.SrcPath)
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(tarPath, tarArg, env.SrcPath)
	cmd.Dir = env.BuildDir
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("command failed: %s - stdout: %s - stderr: %s", err, stdout.String(), stderr.String())
	}

	// We do not need the package anymore, delete it
	err = os.Remove(env.SrcPath)
	if err != nil {
		return fmt.Errorf("failed to delete %s: %s", env.SrcPath, err)
	}

	// We save the directory created while untaring the tarball
	entries, err := ioutil.ReadDir(env.BuildDir)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %s", env.BuildDir, err)
	}
	if len(entries) != 1 {
		return fmt.Errorf("inconsistent temporary %s directory, %d files instead of 1", env.BuildDir, len(entries))
	}
	env.SrcDir = filepath.Join(env.BuildDir, entries[0].Name())

	return nil
}

// RunMake executes the appropriate command to build the software
func (env *Info) RunMake(priv bool, args []string, stage string) error {
	// Some sanity checks
	if env.SrcDir == "" {
		return fmt.Errorf("invalid parameter(s)")
	}

	var makeCmd syexec.SyCmd
	makeCmd.ManifestName = "make"
	if stage != "" {
		args = append(args, stage)
		makeCmd.ManifestName = strings.Join(args, "_")
	}

	args = append([]string{"-j4"}, args...)
	logMsg := "make " + strings.Join(args, " ")
	if !priv {
		makeCmd.BinPath = "make"
	} else {
		sudoBin, err := exec.LookPath("sudo")
		logMsg = sudoBin + " " + logMsg
		if err != nil {
			return fmt.Errorf("failed to find the sudo binary: %s", err)
		}
		args = append([]string{"make"}, args...)
		makeCmd.BinPath = sudoBin
	}
	makeCmd.CmdArgs = args
	log.Printf("* Executing (from %s): %s", env.SrcDir, logMsg)
	if len(env.Env) > 0 {
		makeCmd.Env = env.Env
	}
	makeCmd.ExecDir = env.SrcDir
	res := makeCmd.Run()
	if res.Err != nil {
		return fmt.Errorf("command failed: %s - stdout: %s - stderr: %s", res.Err, res.Stdout, res.Stderr)
	}

	return nil
}

func (env *Info) copyTarball(p *SoftwarePackage) error {
	// Some sanity checks
	if p.URL == "" {
		return fmt.Errorf("invalid parameter(s)")
	}

	// Figure out the name of the file if we do not already have it
	if p.tarball == "" {
		p.tarball = path.Base(p.URL)
	}

	targetTarballPath := filepath.Join(env.BuildDir, p.tarball)
	// The begining of the URL starts with 'file://' which we do not want
	err := util.CopyFile(p.URL[7:], targetTarballPath)
	if err != nil {
		return fmt.Errorf("cannot copy file %s to %s: %s", p.URL, targetTarballPath, err)
	}

	env.SrcPath = filepath.Join(env.BuildDir, p.tarball)

	return nil
}

func (env *Info) gitCheckout(p *SoftwarePackage) error {
	// todo: should it be cached in sysCfg and passed in?
	gitBin, err := exec.LookPath("git")
	if err != nil {
		return fmt.Errorf("failed to find git: %s", err)
	}

	repoName := filepath.Base(p.URL)
	repoName = strings.Replace(repoName, ".git", "", 1)
	checkoutPath := filepath.Join(env.BuildDir, repoName)

	if util.PathExists(checkoutPath) {
		gitCmd := exec.Command(gitBin, "pull")
		log.Printf("Running from %s: %s pull\n", checkoutPath, gitBin)
		gitCmd.Dir = checkoutPath
		var stderr, stdout bytes.Buffer
		gitCmd.Stderr = &stderr
		gitCmd.Stdout = &stdout
		err = gitCmd.Run()
		if err != nil {
			return fmt.Errorf("command failed: %s - stdout: %s - stderr: %s", err, stdout.String(), stderr.String())
		}

	} else {
		gitCmd := exec.Command(gitBin, "clone", p.URL)
		log.Printf("Running from %s: %s clone %s\n", env.BuildDir, gitBin, p.URL)
		gitCmd.Dir = env.BuildDir
		var stderr, stdout bytes.Buffer
		gitCmd.Stderr = &stderr
		gitCmd.Stdout = &stdout
		err = gitCmd.Run()
		if err != nil {
			return fmt.Errorf("command failed: %s - stdout: %s - stderr: %s", err, stdout.String(), stderr.String())
		}
	}

	// Both env.SrcPath and env.SrcDir are set to the directory checkout because:
	// - the value of SrcPath will make the code figure out in a safe manner that it is not necessary to do unpack
	// - the value of SrcDir will point to where the code is from configuration/compilation/installation
	env.SrcPath = checkoutPath
	env.SrcDir = checkoutPath

	return nil
}

// Get is the function to get a given source code
func (env *Info) Get(p *SoftwarePackage) error {
	log.Printf("- Getting %s from %s...\n", p.Name, p.URL)

	// Sanity checks
	if p.URL == "" {
		return fmt.Errorf("invalid parameter(s)")
	}

	// Detect the type of URL, e.g., file vs. http*
	urlFormat := util.DetectURLType(p.URL)
	if urlFormat == "" {
		return fmt.Errorf("impossible to detect type from URL %s", p.URL)
	}

	switch urlFormat {
	case util.FileURL:
		err := env.copyTarball(p)
		if err != nil {
			return fmt.Errorf("impossible to copy the tarball: %s", err)
		}
	case util.HttpURL:
		err := env.download(p)
		if err != nil {
			return fmt.Errorf("impossible to download %s: %s", p.Name, err)
		}
	case util.GitURL:
		err := env.gitCheckout(p)
		if err != nil {
			return fmt.Errorf("impossible to get Git repository %s: %s", p.URL, err)
		}
	default:
		return fmt.Errorf("impossible to detect URL type: %s", p.URL)
	}

	return nil
}

func (env *Info) download(p *SoftwarePackage) error {
	// Sanity checks
	if p.URL == "" || env.BuildDir == "" {
		return fmt.Errorf("invalid parameter(s)")
	}

	log.Printf("- Downloading %s from %s...", p.Name, p.URL)

	// todo: do not assume wget
	binPath, err := exec.LookPath("wget")
	if err != nil {
		return fmt.Errorf("cannot find wget: %s", err)
	}

	log.Printf("* Executing from %s: %s %s", env.BuildDir, binPath, p.URL)
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(binPath, p.URL)
	cmd.Dir = env.BuildDir
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("command failed: %s - stdout: %s - stderr: %s", err, stdout.String(), stderr.String())
	}

	// todo: we currently assume that we have one and only one file in the
	// directory This is not a fair assumption, especially while debugging
	// when we do not wipe out the temporary directories
	files, err := ioutil.ReadDir(env.BuildDir)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %s", env.BuildDir, err)
	}
	if len(files) != 1 {
		return fmt.Errorf("inconsistent temporary %s directory, %d files instead of 1", env.BuildDir, len(files))
	}
	p.tarball = files[0].Name()
	env.SrcPath = filepath.Join(env.BuildDir, files[0].Name())

	return nil
}

// IsInstalled checks whether a specific software package is already installed in a specific build environment
func (env *Info) IsInstalled(p *SoftwarePackage) bool {
	switch util.DetectURLType(p.URL) {
	case util.FileURL:
		filename := path.Base(p.URL)
		filePathInBuildDir := filepath.Join(env.BuildDir, filename)
		filePathInInstallDir := filepath.Join(env.InstallDir, filename)
		return util.FileExists(filePathInBuildDir) || util.FileExists(filePathInInstallDir)
	case util.HttpURL:
		// todo: do not assume that a package downloaded from the web is always a tarball
		filename := path.Base(p.URL)
		filePath := filepath.Join(env.BuildDir, filename)
		log.Printf("* Checking whether %s exists...\n", filePath)
		return util.FileExists(filePath)
	case util.GitURL:
		dirname := path.Base(p.URL)
		dirname = strings.Replace(dirname, ".git", "", -1)
		path := filepath.Join(env.BuildDir, dirname)
		return util.PathExists(path)
	}

	return false
}

// GetEnvPath returns the string representing the value for the PATH environment
// variable to use
func (env *Info) GetEnvPath() string {
	return filepath.Join(env.InstallDir, "bin") + ":" + os.Getenv("PATH")
}

// GetEnvLDPath returns the string representing the value for the LD_LIBRARY_PATH
// environment variable to use
func (env *Info) GetEnvLDPath() string {
	return filepath.Join(env.InstallDir, "lib") + ":" + os.Getenv("LD_LIBRARY_PATH")
}

func (env *Info) lookPath(bin string) string {
	for _, e := range env.Env {
		envEntry := strings.Split(e, "=")
		if envEntry[0] == "PATH" {
			tokens := strings.Split(envEntry[1], ":")
			for _, t := range tokens {
				fullPath := filepath.Join(t, bin)
				if util.FileExists(fullPath) {
					return fullPath
				}
			}
		}
	}

	return bin
}

// Install is a generic function to install a software
func (env *Info) Install(p *SoftwarePackage) error {
	if p.InstallCmd == "" {
		log.Println("* Application does not need installation, skipping...")
		return nil
	}

	var cmd syexec.SyCmd
	cmdElts := strings.Split(p.InstallCmd, " ")
	cmd.BinPath = env.lookPath(cmdElts[0])
	cmd.CmdArgs = cmdElts[1:]
	cmd.ExecDir = env.SrcDir
	cmd.ManifestName = "install"
	cmd.ManifestDir = env.InstallDir
	cmd.Env = env.Env

	log.Printf("Executing from %s: %s %s.", env.SrcDir, cmd.BinPath, strings.Join(cmdElts[1:], " "))
	log.Printf("Environment: %s\n", strings.Join(env.Env, "\n"))
	res := cmd.Run()
	if res.Err != nil {
		return fmt.Errorf("failed to install %s: %s; stdout: %s; stderr: %s", p.Name, res.Err, res.Stdout, res.Stderr)
	}

	return nil
}

func getDefaultHostMPIInstallDir(mpi *implem.Info, sysCfg *sys.Config) (string, error) {
	installDir := persistent.GetPersistentHostMPIInstallDir(mpi, sysCfg)

	if sysCfg.Persistent == "" {
		// Create a temporary directory where to install MPI
		installDir = filepath.Join(sysCfg.ScratchDir, sys.MPIInstallDirPrefix+mpi.ID+"-"+mpi.Version)
		err := util.DirInit(installDir)
		if err != nil {
			return "", fmt.Errorf("failed to initialize directory %s: %s", installDir, err)
		}
	}

	return installDir, nil
}

func createMPIHostEnvCfg(env *Info, mpi *implem.Info, sysCfg *sys.Config) error {
	/* SET THE BUILD DIRECTORY */

	// The build directory is always in the scratch
	env.BuildDir = filepath.Join(sysCfg.ScratchDir, sys.MPIBuildDirPrefix+mpi.ID+"_"+mpi.Version)
	// We always initialize the build directory for MPI on the host
	err := util.DirInit(env.BuildDir)
	if err != nil {
		return fmt.Errorf("failed to initialize directory %s: %s", env.BuildDir, err)
	}

	/* SET THE INSTALL DIRECTORY */

	env.InstallDir, err = getDefaultHostMPIInstallDir(mpi, sysCfg)
	if err != nil {
		return fmt.Errorf("failed to get installation directory: %s", err)
	}

	/* SET THE SCRATCH DIRECTORY */

	env.ScratchDir = filepath.Join(sysCfg.ScratchDir, "scratch_"+mpi.ID+"_"+mpi.Version)
	// We always initialize the scratch directory for MPI on the host
	err = util.DirInit(env.ScratchDir)
	if err != nil {
		return fmt.Errorf("failed to initialize directory %s: %s", env.ScratchDir, err)
	}

	return nil
}

func createNoMPIHostEnvCfg(env *Info, sysCfg *sys.Config) error {
	var err error

	/* SET THE BUILD DIRECTORY */

	// The build directory is always in the scratch
	env.BuildDir, err = ioutil.TempDir(sysCfg.ScratchDir, "build")
	if err != nil {
		return fmt.Errorf("failed to create scratch directory: %s", err)
	}

	/* SET THE INSTALL DIRECTORY */

	env.InstallDir, err = ioutil.TempDir(sysCfg.ScratchDir, "install")
	if err != nil {
		return fmt.Errorf("failed to get installation directory: %s", err)
	}

	/* SET THE SCRATCH DIRECTORY */

	env.ScratchDir, err = ioutil.TempDir(sysCfg.ScratchDir, "scratch")
	if err != nil {
		return fmt.Errorf("failed to initialize directory %s: %s", env.ScratchDir, err)
	}

	return nil
}

// CreateDefaultHostEnvCfg returns the default configuration to install/manage MPI on the host
func CreateDefaultHostEnvCfg(env *Info, mpi *implem.Info, sysCfg *sys.Config) error {
	if mpi != nil {
		return createMPIHostEnvCfg(env, mpi, sysCfg)
	}

	return createNoMPIHostEnvCfg(env, sysCfg)
}

func createContainerNonpersistentMPIBuildEnv(containerBuildEnv *Info, sysCfg *sys.Config) (func(), error) {
	var err error
	var cleanup func()

	// If we do not integrate with the sympi (i.e., no persistent mode), all subdirectories
	// in the system wide scratch directory or, if that directory is not defined, in a new
	// temporary directory. In any case, the temporary directory will NOT
	// be deleted since it will have both the container image and the definition file. The
	// subdirectories in the temporary directory should be deleted automatically.
	if sysCfg.ScratchDir != "" {
		containerBuildEnv.ScratchDir = sysCfg.ScratchDir
	} else {
		containerBuildEnv.ScratchDir, err = ioutil.TempDir("", "")
		if err != nil {
			return cleanup, fmt.Errorf("failed to create temporary directory: %s", err)
		}
	}
	containerBuildEnv.BuildDir = filepath.Join(containerBuildEnv.ScratchDir, "container", "build")
	containerBuildEnv.InstallDir = filepath.Join(containerBuildEnv.ScratchDir, "install")

	cleanup = func() {
		err := os.RemoveAll(containerBuildEnv.ScratchDir)
		if err != nil {
			log.Printf("failed to cleanup %s: %s", containerBuildEnv.ScratchDir, err)
		}
		err = os.RemoveAll(containerBuildEnv.BuildDir)
		if err != nil {
			log.Printf("failed to cleanup %s: %s", containerBuildEnv.BuildDir, err)
		}
		err = os.RemoveAll(containerBuildEnv.InstallDir)
		if err != nil {
			log.Printf("failed to cleanup %s: %s", containerBuildEnv.InstallDir, err)
		}
	}

	return cleanup, err
}

func createContainerPersistentMPIBuildEnv(containerBuildEnv *Info, kvs []kv.KV, sysCfg *sys.Config) (func(), error) {
	var err error
	var cleanup func()

	containerBuildEnv.ScratchDir = filepath.Join(sysCfg.Persistent, "scratch_"+kv.GetValue(kvs, "app_name"))
	containerBuildEnv.BuildDir = filepath.Join(sysCfg.Persistent, "build_"+kv.GetValue(kvs, "app_name"))
	containerBuildEnv.InstallDir = filepath.Join(sysCfg.Persistent, sys.ContainerInstallDirPrefix+kv.GetValue(kvs, "app_name"))

	cleanup = func() {
		err := os.RemoveAll(containerBuildEnv.ScratchDir)
		if err != nil {
			log.Printf("failed to cleanup %s: %s", containerBuildEnv.ScratchDir, err)
		}
		err = os.RemoveAll(containerBuildEnv.BuildDir)
		if err != nil {
			log.Printf("failed to cleanup %s: %s", containerBuildEnv.BuildDir, err)
		}
	}

	return cleanup, err
}

// CreateDefaultContainerEnvCfg sets all the details for a default build environment for any
// type of application (it does not have to be a MPI application)
func CreateDefaultContainerEnvCfg(containerBuildEnv *Info, kvs []kv.KV, sysCfg *sys.Config) (func(), error) {
	var err error
	var cleanup func()

	if sys.IsPersistent(sysCfg) {
		cleanup, err = createContainerPersistentMPIBuildEnv(containerBuildEnv, kvs, sysCfg)
	} else {
		cleanup, err = createContainerNonpersistentMPIBuildEnv(containerBuildEnv, sysCfg)
	}

	return cleanup, err
}

// GetDefaultScratchDir returns the default directory to use as scratch directory
func GetDefaultScratchDir(mpi *implem.Info) string {
	return filepath.Join(sys.GetSympiDir(), "scratch-"+mpi.ID)
}

// Init ensures that the buildenv is correctly initialized
func (e *Info) Init(sysCfg *sys.Config) error {
	if !util.PathExists(e.ScratchDir) {
		err := os.MkdirAll(e.ScratchDir, 0755)
		if err != nil {
			return fmt.Errorf("failed to create scratch directory %s: %s", e.ScratchDir, err)
		}
	}
	if !util.PathExists(e.BuildDir) {
		err := os.MkdirAll(e.BuildDir, 0755)
		if err != nil {
			return fmt.Errorf("failed to create build directory %s: %s", e.BuildDir, err)
		}
	}
	if !util.PathExists(e.InstallDir) {
		err := os.MkdirAll(e.InstallDir, 0755)
		if err != nil {
			return fmt.Errorf("failed to create build directory %s: %s", e.InstallDir, err)
		}
	}
	return nil
}
