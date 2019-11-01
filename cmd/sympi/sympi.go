// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/sylabs/singularity-mpi/internal/pkg/buildenv"
	"github.com/sylabs/singularity-mpi/internal/pkg/jm"

	"github.com/sylabs/singularity-mpi/internal/pkg/kv"
	"github.com/sylabs/singularity-mpi/internal/pkg/launcher"
	"github.com/sylabs/singularity-mpi/internal/pkg/mpi"

	"github.com/sylabs/singularity-mpi/internal/pkg/builder"
	"github.com/sylabs/singularity-mpi/internal/pkg/implem"

	"github.com/sylabs/singularity-mpi/internal/pkg/sys"
	util "github.com/sylabs/singularity-mpi/internal/pkg/util/file"
)

func displayInstalled(dir string) error {
	var hostInstalls []string
	var containers []string

	entries, err := ioutil.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read %s: %s", dir, err)
	}

	for _, entry := range entries {
		matched, err := regexp.MatchString(`mpi_install_.*`, entry.Name())
		if err != nil {
			return fmt.Errorf("failed to parse %s: %s", entry, err)
		}
		if matched {
			s := strings.Replace(entry.Name(), "mpi_install_", "", -1)
			hostInstalls = append(hostInstalls, strings.Replace(s, "-", ":", -1))
		}

		matched, err = regexp.MatchString(`mpi_container_.*`, entry.Name())
		if err != nil {
			return fmt.Errorf("failed to parse %s: %s", entry, err)
		}
		if matched {
			containers = append(containers, strings.Replace(entry.Name(), "mpi_container_", "", -1))
		}
	}

	if len(hostInstalls) > 0 {
		fmt.Printf("Available MPI installation(s) on the host:\n\t")
		fmt.Println(strings.Join(hostInstalls, "\n\t"))
		fmt.Printf("\n")
	} else {
		fmt.Println("No MPI available on the host")
	}

	if len(containers) > 0 {
		fmt.Printf("Available container(s):\n\t")
		fmt.Println(strings.Join(containers, "\n\t"))
	} else {
		fmt.Println("No container available")
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

func loadMPI(id string) error {
	// We do not check the return code because it really does not matter if it fails
	// since we do not know the prior state
	unloadMPI()

	tokens := strings.Split(id, ":")
	if len(tokens) != 2 {
		fmt.Println("invalid installation of MPI, execute 'sympi -list' to get the list of available installations")
		return nil
	}
	implem := tokens[0]
	ver := tokens[1]

	if implem == "" || ver == "" {
		fmt.Println("invalid installation of MPI, execute 'sympi -list' to get the list of available installations")
		return nil
	}

	sympiDir := sys.GetSympiDir()
	mpiBaseDir := filepath.Join(sympiDir, "mpi_install_"+implem+"-"+ver)
	mpiBinDir := filepath.Join(mpiBaseDir, "bin")
	mpiLibDir := filepath.Join(mpiBaseDir, "lib")

	path := os.Getenv("PATH")
	path = mpiBinDir + ":" + path
	ldlib := os.Getenv("LD_LIBRARY_PATH")
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

func installMPIonHost(mpiDesc string) error {
	var mpiCfg implem.Info

	tokens := strings.Split(mpiDesc, ":")
	if len(tokens) != 2 {
		return fmt.Errorf("%s is not the correct format to describe an MPI implementation (e.g., 'openmpi:4.0.2'", mpiDesc)
	}

	sysCfg, _, _, err := launcher.Load()
	if err != nil {
		log.Fatalf("unable to load configuration: %s", err)

	}

	mpiCfg.ID = tokens[0]
	mpiCfg.Version = tokens[1]
	// With sympi, we are always in persistent mode
	sysCfg.Persistent = sys.GetSympiDir()
	sysCfg.ScratchDir = buildenv.GetDefaultScratchDir(&mpiCfg)
	err := util.DirInit(sysCfg.ScratchDir)
	if err != nil {
		return fmt.Errorf("unable to initialize scratch directory %s: %s", sysCfg.ScratchDir, err)
	}

	mpiConfigFile := mpi.GetMPIConfigFile(mpiCfg.ID, &sysCfg)
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
	err = buildenv.CreateDefaultHostEnvCfg(&buildEnv, &mpiCfg, &sysCfg)
	if err != nil {
		return fmt.Errorf("failed to set host build environment: %s", err)
	}

	jobmgr := jm.Detect()
	execRes := b.InstallHost(&mpiCfg, &jobmgr, &buildEnv, &sysCfg)
	if execRes.Err != nil {
		return fmt.Errorf("failed to install MPI on the host: %s", execRes.Err)
	}

	return nil
}

func main() {
	list := flag.Bool("list", false, "List all MPI on the host and all MPI containers")
	load := flag.String("load", "", "The version of MPI installed on the host to load")
	unload := flag.Bool("unload", false, "Unload the current version of the MPI that is used")
	install := flag.String("install", "", "MPI implementation to install, e.g., openmpi:4.0.2")

	flag.Parse()

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
		err := installMPIonHost(*install)
		if err != nil {
			log.Fatalf("impossible to install %s: %s", *install, err)
		}
	}
}
