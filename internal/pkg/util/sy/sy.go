// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package sy

import (
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"
	"strings"

	"github.com/sylabs/singularity-mpi/internal/pkg/checker"
	"github.com/sylabs/singularity-mpi/internal/pkg/implem"
	"github.com/sylabs/singularity-mpi/internal/pkg/kv"
	"github.com/sylabs/singularity-mpi/internal/pkg/sys"
	util "github.com/sylabs/singularity-mpi/internal/pkg/util/file"
	"github.com/sylabs/singularity/pkg/syfs"
)

// MPIToolConfig is the structure hosting the data from the tool's configuration file (~/.singularity/singularity-mpi.conf)
type MPIToolConfig struct {
	// BuildPrivilege specifies whether or not we can build images on the platform
	BuildPrivilege bool

	// NoPriv specifies whether or not we need to use the '-u' flag with singularity commands
	NoPriv bool
}

const (
	// BuildPrivilegeKey is the key used in the tool's configuration file to specify if the tool can create images on the platform
	BuildPrivilegeKey = "build_privilege"

	// NoPrivKey is the key used to specify whether Singularity should be executed without any privilege
	NoPrivKey = "force_unprivileged"

	// SudoCmdsKey is the key used to specify which Singularity commands need to be executed with sudo
	SudoCmdsKey = "singularity_sudo_cmds"
)

// GetPathToSyMPIConfigFile returns the path to the tool's configuration file
func GetPathToSyMPIConfigFile() string {
	return filepath.Join(syfs.ConfigDir(), "singularity-mpi.conf")
}

func saveMPIConfigFile(path string, data []string) error {
	text := strings.Join(data, "\n")
	err := ioutil.WriteFile(path, []byte(text), 0644)
	if err != nil {
		return fmt.Errorf("Impossible to create configuration file %s :%s", path, err)
	}

	return nil
}

func initMPIConfigFile() ([]string, error) {
	buildPrivilegeEntry := BuildPrivilegeKey + " = true"
	err := checker.CheckBuildPrivilege()
	if err != nil {
		log.Printf("* [INFO] Cannot build singularity images: %s", err)
		buildPrivilegeEntry = BuildPrivilegeKey + " = false"
	}

	sudoCmdsEntry := SudoCmdsKey + " = build" // By default we assume build will require sudo

	data := []string{buildPrivilegeEntry, sudoCmdsEntry}

	return data, nil
}

// LoadMPIConfigFile loads the tool's configuration file into a slice of key/value pairs
func LoadMPIConfigFile() ([]kv.KV, error) {
	syMPIConfigFile := GetPathToSyMPIConfigFile()
	kvs, err := kv.LoadKeyValueConfig(syMPIConfigFile)
	if err != nil {
		return nil, fmt.Errorf("unable to parse %s: %s", syMPIConfigFile, err)
	}

	return kvs, nil
}

// CreateMPIConfigFile ensures that the configuration file of the tool is correctly created
func CreateMPIConfigFile() (string, error) {
	syDir := syfs.ConfigDir()
	if !util.PathExists(syDir) {
		return "", fmt.Errorf("%s does not exist. Is Singularity installed?", syDir)
	}

	syMPIConfigFile := GetPathToSyMPIConfigFile()
	log.Printf("-> Creating/update SyMPI configuration file: %s", syMPIConfigFile)
	if !util.PathExists(syMPIConfigFile) {
		data, err := initMPIConfigFile()
		if err != nil {
			return "", fmt.Errorf("failed to initialize MPI configuration file: %s", err)
		}

		err = saveMPIConfigFile(syMPIConfigFile, data)
		if err != nil {
			return "", fmt.Errorf("unable to save MPI configuration file: %s", err)
		}
	}

	return syMPIConfigFile, nil
}

// ConfigFileUpdateEntry updates the value of a key in the tool's configuration file
func ConfigFileUpdateEntry(configFile string, key string, value string) error {
	kvs, err := LoadMPIConfigFile()
	if err != nil {
		return fmt.Errorf("unable to laod MPI configuration file: %s", err)
	}

	// If the key is already correctly set, we just exit
	currentVal := kv.GetValue(kvs, key)
	if currentVal == value {
		log.Printf("Key %s from %s already set to %s", key, configFile, value)
		return nil
	}

	// If the key does not exist, we create one
	if !kv.KeyExists(kvs, key) {
		var newKV kv.KV
		newKV.Key = key
		newKV.Value = value
		kvs = append(kvs, newKV)
	} else {
		err = kv.SetValue(kvs, key, value)
		if err != nil {
			return fmt.Errorf("failed to update value of the key %s: %s", key, err)
		}
	}

	data := kv.ToStringSlice(kvs)
	err = saveMPIConfigFile(configFile, data)
	if err != nil {
		return fmt.Errorf("unable to save configuration in %s: %s", configFile, err)
	}

	return nil
}

func getRegistryConfigFilePath(mpiCfg *implem.Info, sysCfg *sys.Config) string {
	confFileName := mpiCfg.ID + "-images.conf"
	return filepath.Join(sysCfg.EtcDir, confFileName)
}

// GetImageURL returns the URL to pull an image for a given distro/MPI/test
func GetImageURL(mpiCfg *implem.Info, sysCfg *sys.Config) string {
	registryConfigFile := getRegistryConfigFilePath(mpiCfg, sysCfg)
	log.Printf("* Getting image URL for %s from %s...", mpiCfg.ID+"-"+mpiCfg.Version, registryConfigFile)
	kvs, err := kv.LoadKeyValueConfig(registryConfigFile)
	if err != nil {
		return ""
	}
	return kv.GetValue(kvs, mpiCfg.Version)
}

// IsSudoCnd checks whether a command needs to be executed with sudo based on data from
// the tool's configuration file
func IsSudoCmd(cmd string, sysCfg *sys.Config) bool {
	for _, c := range sysCfg.SudoSyCmds {
		if c == cmd {
			return true
		}
	}
	return false
}
