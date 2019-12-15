// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package sympi

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/gvallee/go_util/pkg/util"
	"github.com/gvallee/kv/pkg/kv"
	"github.com/sylabs/singularity-mpi/internal/pkg/builder"
	"github.com/sylabs/singularity-mpi/internal/pkg/implem"
	"github.com/sylabs/singularity-mpi/internal/pkg/jm"
	"github.com/sylabs/singularity-mpi/internal/pkg/manifest"
	"github.com/sylabs/singularity-mpi/internal/pkg/mpi"
	"github.com/sylabs/singularity-mpi/pkg/app"
	"github.com/sylabs/singularity-mpi/pkg/buildenv"
	"github.com/sylabs/singularity-mpi/pkg/container"
	"github.com/sylabs/singularity-mpi/pkg/launcher"
	"github.com/sylabs/singularity-mpi/pkg/sy"
	"github.com/sylabs/singularity-mpi/pkg/syexec"
	"github.com/sylabs/singularity-mpi/pkg/sys"
)

// UpdateEnvFile updates the file that is automatically sources while using
// SyMPI and setting the environment.
func UpdateEnvFile(file string, pathEnv string, ldlibEnv string) error {
	// sanity checks
	if len(pathEnv) == 0 {
		return fmt.Errorf("invalid parameter, empty PATH")
	}

	f, err := os.Create(file)
	if err != nil {
		return fmt.Errorf("failed to create %s: %s", file, err)
	}
	defer f.Close()
	_, err = f.WriteString("export PATH=" + pathEnv + "\n")
	if err != nil {
		return fmt.Errorf("failed to write to %s: %s", file, err)
	}
	_, err = f.WriteString("export LD_LIBRARY_PATH=" + ldlibEnv + "\n")
	if err != nil {
		return fmt.Errorf("failed to write to %s: %s", file, err)
	}
	return nil
}

func getPPPID() (int, error) {
	// We need to find the parent of our parent process
	ppid := os.Getppid()
	pppid := 0 // Only for now
	parentInfoFile := filepath.Join("/proc", strconv.Itoa(ppid), "status")
	procFile, err := os.Open(parentInfoFile)
	if err != nil {
		return -1, fmt.Errorf("failed to open %s: %s", parentInfoFile, err)
	}
	defer procFile.Close()
	for s := bufio.NewScanner(procFile); s.Scan(); {
		var temp int
		if n, _ := fmt.Sscanf(s.Text(), "PPid:\t%d", &temp); n == 1 {
			pppid = temp
		}
	}

	return pppid, nil
}

// GetEnvFile returns the absolute path to the file that is automatically sources while using
// SyMPI.
func GetEnvFile() (string, error) {
	pppid, err := getPPPID()
	if err != nil {
		return "", fmt.Errorf("failed to get PPPID: %s", err)
	}
	filename := "sympi_" + strconv.Itoa(pppid)
	return filepath.Join("/tmp", filename), nil
}

func cleanupEnvVar(prefix string) ([]string, []string) {
	var newPath []string
	var newLDLIB []string

	curPath := os.Getenv("PATH")
	curLDLIB := os.Getenv("LD_LIBRARY_PATH")

	pathTokens := strings.Split(curPath, ":")
	for _, t := range pathTokens {
		if !strings.Contains(t, prefix) {
			newPath = append(newPath, t)
		}
	}

	ldlibTokens := strings.Split(curLDLIB, ":")
	for _, t := range ldlibTokens {
		if !strings.Contains(t, prefix) {
			newLDLIB = append(newLDLIB, t)
		}
	}

	return newPath, newLDLIB
}

// GetCleanedUpSyEnvVars parses the current environment and cleans up to
// ensure that is not interference between the currently loaded installation
// of Singularity and what was previously used.
func GetCleanedUpSyEnvVars() ([]string, []string) {
	return cleanupEnvVar(sys.SingularityInstallDirPrefix)
}

// GetCleanedUpMPIEnvVars parses the current environment and cleans up to
// ensure that is not interference between the currently loaded installation
// of MPI and what was previously used.
func GetCleanedUpMPIEnvVars() ([]string, []string) {
	return cleanupEnvVar(sys.MPIInstallDirPrefix)
}

// LoadMPI loads a specific implementation of MPI in the current environment.
func LoadMPI(id string) error {
	// We can change the env multiple times during the execution of a single command
	// and these modifications will NOT be reflected in the actual environment until
	// we exit the command and let bash do some magic to update it. Fortunately, we
	// know that we can have one and only one MPI in the environment of a single time
	// so when we load a MPI, we make sure that we remove a previous load changes.
	cleanedPath, cleanedLDLIB := GetCleanedUpMPIEnvVars()

	implem, ver := GetMPIDetails(id)
	if implem == "" || ver == "" {
		fmt.Println("invalid installation of MPI, execute 'sympi -list' to get the list of available installations")
		return nil
	}

	sympiDir := sys.GetSympiDir()
	mpiBaseDir := filepath.Join(sympiDir, sys.MPIInstallDirPrefix+implem+"-"+ver)
	mpiBinDir := filepath.Join(mpiBaseDir, "bin")
	mpiLibDir := filepath.Join(mpiBaseDir, "lib")

	path := strings.Join(cleanedPath, ":")
	ldlib := strings.Join(cleanedLDLIB, ":")
	path = mpiBinDir + ":" + path
	ldlib = mpiLibDir + ":" + ldlib

	file, err := GetEnvFile()
	if err != nil || !util.FileExists(file) {
		return fmt.Errorf("file %s does not exist", file)
	}

	err = UpdateEnvFile(file, path, ldlib)
	if err != nil {
		return fmt.Errorf("failed to update %s: %s", file, err)
	}

	return nil
}

func getImagePath(containerDesc string, sysCfg *sys.Config) (string, error) {
	containerInstallDir := filepath.Join(sys.GetSympiDir(), sys.ContainerInstallDirPrefix+containerDesc)
	imgPath := filepath.Join(containerInstallDir, containerDesc+".sif")
	if !util.FileExists(imgPath) {
		return "", fmt.Errorf("%s does not exist", imgPath)
	}

	return imgPath, nil
}

// GetDefaultSysConfig loads the default system configuration
func GetDefaultSysConfig() sys.Config {
	sysCfg, _, _, err := launcher.Load()
	if err != nil {
		log.Fatalf("unable to load configuration: %s", err)

	}

	return sysCfg
}

func runStandardContainer(args []string, containerInfo *container.Config, sysCfg *sys.Config) (syexec.Result, error) {
	var hostBuildEnv buildenv.Info
	var hostCfg mpi.Config
	var containerCfg mpi.Config
	var appInfo app.Info
	var execRes syexec.Result

	err := buildenv.CreateDefaultHostEnvCfg(&hostBuildEnv, nil, sysCfg)
	if err != nil {
		return execRes, fmt.Errorf("failed to create default host environment configuration: %s", err)
	}

	hostCfg.Buildenv = hostBuildEnv
	containerCfg.Container = *containerInfo
	appInfo.Name = containerInfo.Name
	appInfo.BinPath = containerInfo.AppExe

	// Launch the container
	jobmgr := jm.Detect()
	expRes, execRes := launcher.Run(&appInfo, nil, &hostBuildEnv, &containerCfg, &jobmgr, sysCfg, args)
	if !expRes.Pass {
		return execRes, fmt.Errorf("failed to run the container: %s (stdout: %s; stderr: %s)", execRes.Err, execRes.Stderr, execRes.Stdout)
	}

	return execRes, nil
}

func runMPIContainer(args []string, containerMPI *implem.Info, containerInfo *container.Config, sysCfg *sys.Config) (syexec.Result, error) {
	var execRes syexec.Result
	fmt.Printf("Container based on %s %s\n", containerMPI.ID, containerMPI.Version)
	fmt.Println("Looking for available compatible version...")
	hostMPI, err := findCompatibleMPI(containerMPI)
	if err != nil {
		fmt.Printf("No compatible MPI found, installing the appropriate version...")
		err := InstallMPIonHost(containerMPI.ID+"-"+containerMPI.Version, sysCfg)
		if err != nil {
			return execRes, fmt.Errorf("failed to install %s %s", containerMPI.ID, containerMPI.Version)
		}
		hostMPI.ID = containerMPI.ID
		hostMPI.Version = containerMPI.Version
	} else {
		fmt.Printf("%s %s was found on the host as a compatible version\n", hostMPI.ID, hostMPI.Version)
	}
	fmt.Printf("Container is in %s mode\n", containerInfo.Model)
	if containerInfo.Model == container.BindModel {
		fmt.Printf("Binding/mounting %s %s on host -> %s\n", hostMPI.ID, hostMPI.Version, containerInfo.MPIDir)
	}

	err = LoadMPI(hostMPI.ID + ":" + hostMPI.Version)
	if err != nil {
		return execRes, fmt.Errorf("failed to load MPI %s %s on host: %s", hostMPI.ID, hostMPI.Version, err)
	}

	var hostBuildEnv buildenv.Info
	err = buildenv.CreateDefaultHostEnvCfg(&hostBuildEnv, &hostMPI, sysCfg)
	if err != nil {
		return execRes, fmt.Errorf("failed to create default host environment configuration: %s", err)
	}
	var hostMPICfg mpi.Config
	var containerMPICfg mpi.Config
	var appInfo app.Info

	hostMPICfg.Implem = hostMPI
	hostMPICfg.Buildenv = hostBuildEnv

	containerMPICfg.Implem = *containerMPI
	containerMPICfg.Container = *containerInfo
	appInfo.Name = containerInfo.Name
	appInfo.BinPath = containerInfo.AppExe

	// Launch the container
	jobmgr := jm.Detect()
	expRes, execRes := launcher.Run(&appInfo, &hostMPICfg, &hostBuildEnv, &containerMPICfg, &jobmgr, sysCfg, args)
	if !expRes.Pass {
		return execRes, fmt.Errorf("failed to run the container: %s (stdout: %s; stderr: %s)", execRes.Err, execRes.Stderr, execRes.Stdout)
	}

	return execRes, nil
}

// RunContainer is a high-level function to execute a container that was created with the
// SyMPI framework (it relies on metadata)
func RunContainer(containerDesc string, args []string, sysCfg *sys.Config) error {
	// When running containers with sympi, we are always in the context of persistent installs
	sysCfg.Persistent = sys.GetSympiDir()

	// Get the full path to the image
	imgPath, err := getImagePath(containerDesc, sysCfg)
	if err != nil {
		return fmt.Errorf("failed to get path to image for container %s: %s", containerDesc, err)
	}

	// Inspect the image and extract the metadata
	err = sy.CheckIntegrity(sysCfg)
	if err != nil {
		fmt.Printf("[WARNING] Your Singularity installation seems to be corrupted: %s\n", err)
		return fmt.Errorf("Compromised Singularity installation")
	}

	fmt.Printf("Analyzing %s to figure out the correct configuration for execution...\n", imgPath)
	containerInfo, containerMPI, err := container.GetMetadata(imgPath, sysCfg)
	if err != nil {
		return fmt.Errorf("failed to extract container's metadata: %s", err)
	}
	containerInfo.Name = containerDesc
	var execRes syexec.Result
	if containerMPI.ID != "" && containerMPI.Version != "" {
		execRes, err = runMPIContainer(args, &containerMPI, &containerInfo, sysCfg)
		if err != nil {
			return fmt.Errorf("failed to run MPI container: %s", err)
		}
	} else {
		log.Println("Container is not using MPI")
		execRes, err = runStandardContainer(args, &containerInfo, sysCfg)
		if err != nil {
			return fmt.Errorf("failed to run standard container: %s", err)
		}
	}

	fmt.Printf("Execution successful!\n\tStdout: %s\n\tStderr: %s\n", execRes.Stderr, execRes.Stdout)

	return nil
}

// GetHostMPIInstalls returns all the MPI implementations installed in the current
// workspace
func GetHostMPIInstalls(entries []os.FileInfo) ([]string, error) {
	var hostInstalls []string

	for _, entry := range entries {
		matched, err := regexp.MatchString(sys.MPIInstallDirPrefix+`.*`, entry.Name())
		if err != nil {
			return hostInstalls, fmt.Errorf("failed to parse %s: %s", entry, err)
		}
		if matched {
			s := strings.Replace(entry.Name(), sys.MPIInstallDirPrefix, "", -1)
			hostInstalls = append(hostInstalls, strings.Replace(s, "-", ":", -1))
		}
	}

	return hostInstalls, nil
}

func findCompatibleMPI(targetMPI *implem.Info) (implem.Info, error) {
	var mpi implem.Info
	mpi.ID = targetMPI.ID

	entries, err := ioutil.ReadDir(sys.GetSympiDir())
	if err != nil {
		return mpi, fmt.Errorf("failed to read %s: %s", sys.GetSympiDir(), err)
	}

	hostInstalls, err := GetHostMPIInstalls(entries)
	if err != nil {
		return mpi, fmt.Errorf("unable to get the install of MPIs installed on the host: %s", err)
	}

	versionDetails := strings.Split(targetMPI.Version, ".")
	major := versionDetails[0]
	ver := ""
	for _, entry := range hostInstalls {
		tokens := strings.Split(entry, ":")
		if tokens[0] == targetMPI.ID {
			if tokens[1] == targetMPI.Version {
				// We have the exact version available
				mpi.Version = tokens[1]
				return mpi, nil
			}
			if ver == "" {
				t := strings.Split(tokens[1], ".")
				if t[0] >= major && ver == "" {
					// At first we accept any version from the same major release
					ver = tokens[1]
				}
			} else {
				if ver < tokens[1] {
					ver = tokens[1]
				}
			}
		}
	}

	if ver != "" {
		mpi.Version = ver
		return mpi, nil
	}

	return mpi, fmt.Errorf("no compatible version available")
}

// GetMPIDetails extract the details of a specific MPI implementation from its description
func GetMPIDetails(desc string) (string, string) {
	tokens := strings.Split(desc, ":")
	if len(tokens) != 2 {
		fmt.Println("invalid MPI, execute 'sympi -list' to get the list of available installations")
		return "", ""
	}
	return tokens[0], tokens[1]
}

// InstallMPIonHost installs a specific implementation of MPI on the host
func InstallMPIonHost(mpiDesc string, sysCfg *sys.Config) error {
	var mpiCfg implem.Info
	mpiCfg.ID, mpiCfg.Version = GetMPIDetails(mpiDesc)

	sysCfg.ScratchDir = buildenv.GetDefaultScratchDir(&mpiCfg)
	// When installing a MPI with sympi, we are always in persistent mode
	sysCfg.Persistent = sys.GetSympiDir()

	err := util.DirInit(sysCfg.ScratchDir)
	if err != nil {
		return fmt.Errorf("unable to initialize scratch directory %s: %s", sysCfg.ScratchDir, err)
	}
	defer os.RemoveAll(sysCfg.ScratchDir)

	mpiConfigFile := mpi.GetMPIConfigFile(mpiCfg.ID, sysCfg)
	kvs, err := kv.LoadKeyValueConfig(mpiConfigFile)
	if err != nil {
		return fmt.Errorf("unable to load configuration file %s: %s", mpiConfigFile, err)
	}
	mpiCfg.URL = kv.GetValue(kvs, mpiCfg.Version)

	b, err := builder.Load(&mpiCfg)
	if err != nil {
		return fmt.Errorf("failed to load a builder: %s", err)
	}

	var buildEnv buildenv.Info
	err = buildenv.CreateDefaultHostEnvCfg(&buildEnv, &mpiCfg, sysCfg)
	if err != nil {
		return fmt.Errorf("failed to set host build environment: %s", err)
	}
	defer os.RemoveAll(buildEnv.BuildDir)

	execRes := b.InstallOnHost(&mpiCfg, &buildEnv, sysCfg)
	if execRes.Err != nil {
		return fmt.Errorf("failed to install MPI on the host: %s", execRes.Err)
	}

	// Create the manifest for the MPI installation we just completed
	mpiManifest := filepath.Join(buildEnv.InstallDir, "mpi.MANIFEST")
	if !util.PathExists(mpiManifest) {
		mpiBin := filepath.Join(buildEnv.InstallDir, "bin", "mpiexec")
		fileHashes := manifest.HashFiles([]string{mpiBin})

		err = manifest.Create(mpiManifest, fileHashes)
		if err != nil {
			// This is not a fatal error, we just log the fact we cannot create the manifest
			log.Printf("failed to create the manifest for the MPI installation: %s", err)
		}
	} else {
		// This is not a fatal error, we just log that the manifest already exists
		log.Println("Manifest for MPI installation already exists, skipping...")
	}

	return nil
}
