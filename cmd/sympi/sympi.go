// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/sylabs/singularity-mpi/internal/pkg/app"
	"github.com/sylabs/singularity-mpi/internal/pkg/buildenv"
	"github.com/sylabs/singularity-mpi/internal/pkg/builder"
	"github.com/sylabs/singularity-mpi/internal/pkg/checker"
	"github.com/sylabs/singularity-mpi/internal/pkg/container"
	"github.com/sylabs/singularity-mpi/internal/pkg/implem"
	"github.com/sylabs/singularity-mpi/internal/pkg/jm"
	"github.com/sylabs/singularity-mpi/internal/pkg/kv"
	"github.com/sylabs/singularity-mpi/internal/pkg/launcher"
	"github.com/sylabs/singularity-mpi/internal/pkg/mpi"
	"github.com/sylabs/singularity-mpi/internal/pkg/sys"
	util "github.com/sylabs/singularity-mpi/internal/pkg/util/file"
	"github.com/sylabs/singularity-mpi/internal/pkg/util/sy"
)

func getHostMPIInstalls(entries []os.FileInfo) ([]string, error) {
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

func getContainerInstalls(entries []os.FileInfo) ([]string, error) {
	var containers []string
	for _, entry := range entries {
		matched, err := regexp.MatchString(sys.ContainerInstallDirPrefix+`.*`, entry.Name())
		if err != nil {
			return containers, fmt.Errorf("failed to parse %s: %s", entry, err)
		}
		if matched {
			containers = append(containers, strings.Replace(entry.Name(), sys.ContainerInstallDirPrefix, "", -1))
		}
	}
	return containers, nil
}

func getSingularityInstalls(entries []os.FileInfo) ([]string, error) {
	var singularities []string
	for _, entry := range entries {
		matched, err := regexp.MatchString(sys.SingularityInstallDirPrefix+`.*`, entry.Name())
		if err != nil {
			return singularities, fmt.Errorf("failed to parse %s: %s", entry, err)
		}
		if matched {
			singularities = append(singularities, strings.Replace(entry.Name(), sys.SingularityInstallDirPrefix, "", -1))
		}
	}
	return singularities, nil
}

func displayInstalled(dir string) error {

	entries, err := ioutil.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read %s: %s", dir, err)
	}

	hostInstalls, err := getHostMPIInstalls(entries)
	if err != nil {
		return fmt.Errorf("unable to get the install of MPIs installed on the host: %s", err)
	}
	containers, err := getContainerInstalls(entries)
	if err != nil {
		return fmt.Errorf("unable to get the list of containers stored on the host: %s", err)
	}
	singularities, err := getSingularityInstalls(entries)
	if err != nil {
		return fmt.Errorf("unable to get the list of singularity installs on the host: %s", err)
	}

	if len(singularities) > 0 {
		fmt.Printf("Available Singularity installation(s) on the host:\n")
		for _, sy := range singularities {
			fmt.Printf("\tsingularity: %s\n", sy)
		}
		fmt.Printf("\n")
	} else {
		fmt.Printf("No Singularity available on the host\n\n")
	}

	if len(hostInstalls) > 0 {
		fmt.Printf("Available MPI installation(s) on the host:\n\t")
		fmt.Println(strings.Join(hostInstalls, "\n\t"))
		fmt.Printf("\n")
	} else {
		fmt.Printf("No MPI available on the host\n\n")
	}

	if len(containers) > 0 {
		fmt.Printf("Available container(s):\n\t")
		fmt.Println(strings.Join(containers, "\n\t"))
	} else {
		fmt.Printf("No container available\n\n")
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

func getEnvFile() (string, error) {
	pppid, err := getPPPID()
	if err != nil {
		return "", fmt.Errorf("failed to get PPPID: %s", err)
	}
	filename := "sympi_" + strconv.Itoa(pppid)
	return filepath.Join("/tmp", filename), nil
}

func updateEnvFile(file string, pathEnv string, ldlibEnv string) error {
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

func getMPIDetails(desc string) (string, string) {
	tokens := strings.Split(desc, ":")
	if len(tokens) != 2 {
		fmt.Println("invalid MPI, execute 'sympi -list' to get the list of available installations")
		return "", ""
	}
	return tokens[0], tokens[1]
}

func getCleanedUpEnvVars() ([]string, []string) {
	var newPath []string
	var newLDLIB []string

	sympiDir := sys.GetSympiDir()

	curPath := os.Getenv("PATH")
	curLDLIB := os.Getenv("LD_LIBRARY_PATH")

	pathTokens := strings.Split(curPath, ":")
	for _, t := range pathTokens {
		if !strings.Contains(t, sympiDir) {
			newPath = append(newPath, t)
		}
	}

	ldlibTokens := strings.Split(curLDLIB, ":")
	for _, t := range ldlibTokens {
		if !strings.Contains(t, sympiDir) {
			newLDLIB = append(newLDLIB, t)
		}
	}

	return newPath, newLDLIB
}

func loadMPI(id string) error {
	// We can change the env multiple times during the execution of a single command
	// and these modifications will NOT be reflected in the actual environment until
	// we exit the command and let bash do some magic to update it. Fortunately, we
	// know that we can have one and only one MPI in the environment of a single time
	// so when we load a MPI, we make sure that we remove a previous load changes.
	cleanedPath, cleanedLDLIB := getCleanedUpEnvVars()

	implem, ver := getMPIDetails(id)
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

	file, err := getEnvFile()
	if err != nil || !util.FileExists(file) {
		return fmt.Errorf("file %s does not exist", file)
	}

	err = updateEnvFile(file, path, ldlib)
	if err != nil {
		return fmt.Errorf("failed to update %s: %s", file, err)
	}

	return nil
}

func unloadMPI() error {
	newPath, newLDLIB := getCleanedUpEnvVars()

	// Sanity checks
	if len(newPath) == 0 {
		return fmt.Errorf("new PATH is empty")
	}

	file, err := getEnvFile()
	if err != nil || !util.FileExists(file) {
		return fmt.Errorf("file %s does not exist", file)
	}
	err = updateEnvFile(file, strings.Join(newPath, ":"), strings.Join(newLDLIB, ":"))
	if err != nil {
		return fmt.Errorf("failed to update %s: %s", file, err)
	}

	return nil
}

func getDefaultSysConfig() sys.Config {
	sysCfg, _, _, err := launcher.Load()
	if err != nil {
		log.Fatalf("unable to load configuration: %s", err)

	}

	sympiKVs, err := sy.LoadMPIConfigFile()
	if err != nil {
		log.Printf("failed to run configuration from singularity-mpi configuration file: %s", err)
	}
	val := kv.GetValue(sympiKVs, sy.NoPrivKey)
	if val == "" {
		sysCfg.Nopriv = false
	} else {
		sysCfg.Nopriv = true
	}
	val = kv.GetValue(sympiKVs, sy.SudoCmdsKey)
	if val != "" {
		sysCfg.SudoSyCmds = strings.Split(val, " ")
	}

	return sysCfg
}

func uninstallMPIfromHost(mpiDesc string, sysCfg *sys.Config) error {
	var mpiCfg implem.Info
	mpiCfg.ID, mpiCfg.Version = getMPIDetails(mpiDesc)

	var buildEnv buildenv.Info
	err := buildenv.CreateDefaultHostEnvCfg(&buildEnv, &mpiCfg, sysCfg)
	if err != nil {
		return fmt.Errorf("failed to set host build environment: %s", err)
	}

	b, err := builder.Load(&mpiCfg)
	if err != nil {
		return fmt.Errorf("failed to load a builder: %s", err)
	}

	execRes := b.UninstallHost(&mpiCfg, &buildEnv, sysCfg)
	if execRes.Err != nil {
		return fmt.Errorf("failed to install MPI on the host: %s", execRes.Err)
	}

	return nil
}

func installMPIonHost(mpiDesc string, sysCfg *sys.Config) error {
	var mpiCfg implem.Info
	mpiCfg.ID, mpiCfg.Version = getMPIDetails(mpiDesc)

	sysCfg.ScratchDir = buildenv.GetDefaultScratchDir(&mpiCfg)
	// When installing a MPI with sympi, we are always in persistent mode
	sysCfg.Persistent = sys.GetSympiDir()

	err := util.DirInit(sysCfg.ScratchDir)
	if err != nil {
		return fmt.Errorf("unable to initialize scratch directory %s: %s", sysCfg.ScratchDir, err)
	}

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

	execRes := b.InstallOnHost(&mpiCfg, &buildEnv, sysCfg)
	if execRes.Err != nil {
		return fmt.Errorf("failed to install MPI on the host: %s", execRes.Err)
	}

	return nil
}

func findCompatibleMPI(targetMPI implem.Info) (implem.Info, error) {
	var mpi implem.Info
	mpi.ID = targetMPI.ID

	entries, err := ioutil.ReadDir(sys.GetSympiDir())
	if err != nil {
		return mpi, fmt.Errorf("failed to read %s: %s", sys.GetSympiDir(), err)
	}

	hostInstalls, err := getHostMPIInstalls(entries)
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

func runContainer(containerDesc string, sysCfg *sys.Config) error {
	// When running containers with sympi, we are always in the context of persistent installs
	sysCfg.Persistent = sys.GetSympiDir()

	// Get the full path to the image
	containerInstallDir := filepath.Join(sys.GetSympiDir(), sys.ContainerInstallDirPrefix+containerDesc)
	imgPath := filepath.Join(containerInstallDir, containerDesc+".sif")
	if !util.FileExists(imgPath) {
		return fmt.Errorf("%s does not exist", imgPath)
	}

	// Inspect the image and extract the metadata
	if sysCfg.SingularityBin == "" {
		log.Fatalf("singularity bin not defined")
	}

	fmt.Printf("Analyzing %s to figure out the correct configuration for execution...\n", imgPath)
	containerInfo, containerMPI, err := container.GetMetadata(imgPath, sysCfg)
	if err != nil {
		return fmt.Errorf("failed to extract container's metadata: %s", err)
	}
	fmt.Printf("Container based on %s %s\n", containerMPI.ID, containerMPI.Version)
	fmt.Println("Looking for available compatible version...")
	hostMPI, err := findCompatibleMPI(containerMPI)
	if err != nil {
		fmt.Printf("No compatible MPI found, installing the appropriate version...")
		err := installMPIonHost(containerMPI.ID+"-"+containerMPI.Version, sysCfg)
		if err != nil {
			return fmt.Errorf("failed to install %s %s", containerMPI.ID, containerMPI.Version)
		}
		hostMPI.ID = containerMPI.ID
		hostMPI.Version = containerMPI.Version
	} else {
		fmt.Printf("%s %s was found on the host as a compatible version\n", hostMPI.ID, hostMPI.Version)
	}

	err = loadMPI(hostMPI.ID + ":" + hostMPI.Version)
	if err != nil {
		return fmt.Errorf("failed to load MPI %s %s on host: %s", hostMPI.ID, hostMPI.Version, err)
	}

	var hostBuildEnv buildenv.Info
	err = buildenv.CreateDefaultHostEnvCfg(&hostBuildEnv, &hostMPI, sysCfg)
	var hostMPICfg mpi.Config
	var containerMPICfg mpi.Config
	var appInfo app.Info

	hostMPICfg.Implem = hostMPI
	hostMPICfg.Buildenv = hostBuildEnv

	containerMPICfg.Implem = containerMPI
	containerMPICfg.Container = containerInfo
	appInfo.Name = containerDesc
	appInfo.BinPath = containerInfo.AppExe

	// Launch the container
	jobmgr := jm.Detect()
	expRes, execRes := launcher.Run(&appInfo, &hostMPICfg, &hostBuildEnv, &containerMPICfg, &jobmgr, sysCfg)
	if !expRes.Pass {
		return fmt.Errorf("failed to run the container: %s (stdout: %s; stderr: %s)", execRes.Err, execRes.Stderr, execRes.Stdout)
	}

	fmt.Printf("Execution successful!\n\tStdout: %s\n\tStderr: %s\n", execRes.Stderr, execRes.Stdout)

	return nil
}

func installSingularity(id string, sysCfg *sys.Config) error {
	kvs, err := sy.LoadSingularityReleaseConf(sysCfg)
	if err != nil {
		return fmt.Errorf("failed to load data about Singularity releases: %s", err)
	}

	var sy implem.Info
	sy.ID = implem.SY
	tokens := strings.Split(id, ":")
	if len(tokens) != 2 {
		return fmt.Errorf("%s had an invalid format, it should of the form 'singularity:<version>'", id)
	}

	sy.Version = tokens[1]
	sy.URL = kv.GetValue(kvs, sy.Version)

	b, err := builder.Load(&sy)
	if err != nil {
		return fmt.Errorf("failed to load a builder: %s", err)
	}

	var buildEnv buildenv.Info
	buildEnv.InstallDir = filepath.Join(sys.GetSympiDir(), sys.SingularityInstallDirPrefix+sy.Version)
	buildEnv.ScratchDir = filepath.Join(sys.GetSympiDir(), sys.SingularityScratchDirPrefix+sy.Version)
	buildEnv.BuildDir = filepath.Join(sys.GetSympiDir(), sys.SingularityBuildDirPrefix+sy.Version)
	err = util.DirInit(buildEnv.ScratchDir)
	if err != nil {
		return fmt.Errorf("failed to initialize %s: %s", buildEnv.ScratchDir, err)
	}
	defer os.RemoveAll(buildEnv.ScratchDir)
	err = util.DirInit(buildEnv.BuildDir)
	if err != nil {
		return fmt.Errorf("failed to initializat %s: %s", buildEnv.BuildDir, err)
	}
	defer os.RemoveAll(buildEnv.BuildDir)

	execRes := b.InstallOnHost(&sy, &buildEnv, sysCfg)
	if execRes.Err != nil {
		return fmt.Errorf("failed to install %s: %s", id, execRes.Err)
	}

	return nil
}

func main() {
	verbose := flag.Bool("v", false, "Enable verbose mode")
	debug := flag.Bool("d", false, "Enable debug mode")
	list := flag.Bool("list", false, "List all MPI on the host and all MPI containers")
	load := flag.String("load", "", "The version of MPI installed on the host to load")
	unload := flag.Bool("unload", false, "Unload the current version of the MPI that is used")
	install := flag.String("install", "", "MPI implementation to install, e.g., openmpi:4.0.2")
	uninstall := flag.String("uninstall", "", "MPI implementation to uninstall, e.g., openmpi:4.0.2")
	run := flag.String("run", "", "Run a container")

	flag.Parse()

	// Initialize the log file. Log messages will both appear on stdout and the log file if the verbose option is used
	logFile := util.OpenLogFile("sympi")
	defer logFile.Close()
	if *verbose || *debug {
		nultiWriters := io.MultiWriter(os.Stdout, logFile)
		log.SetOutput(nultiWriters)
	} else {
		log.SetOutput(ioutil.Discard)
	}

	sysCfg := getDefaultSysConfig()
	sysCfg.Verbose = *verbose
	sysCfg.Debug = *debug
	// Save the options passed in through the command flags
	if sysCfg.Debug {
		sysCfg.Verbose = true
		err := checker.CheckSystemConfig()
		if err != nil {
			log.Fatalf("the system is not correctly setup: %s", err)
		}
	}

	sympiDir := sys.GetSympiDir()

	if *list {
		displayInstalled(sympiDir)
	}

	if *load != "" {
		err := loadMPI(*load)
		if err != nil {
			log.Fatalf("impossible to load MPI: %s", err)
		}
	}

	if *unload {
		err := unloadMPI()
		if err != nil {
			log.Fatalf("impossible to unload MPI: %s", err)
		}
	}

	if *install != "" {
		re := regexp.MustCompile("^singularity")

		if re.Match([]byte(*install)) {
			err := installSingularity(*install, &sysCfg)
			if err != nil {
				log.Fatalf("failed to install Singularity %s: %s", *install, err)
			}
		} else {
			err := installMPIonHost(*install, &sysCfg)
			if err != nil {
				log.Fatalf("failed to install MPI %s: %s", *install, err)
			}
		}
	}

	if *uninstall != "" {
		err := uninstallMPIfromHost(*uninstall, &sysCfg)
		if err != nil {
			log.Fatalf("impossible to uninstall %s: %s", *install, err)
		}
	}

	if *run != "" {
		err := runContainer(*run, &sysCfg)
		if err != nil {
			log.Fatalf("impossible to run container %s: %s", *run, err)
		}

	}
}
