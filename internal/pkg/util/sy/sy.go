// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package sy

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"

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
}

const (
	// BuildPrivilegeKey is the key used in the tool's configuration file to specify if the tool can create images on the platform
	BuildPrivilegeKey = "build_privilege"
)

// GetPathToSyMPIConfigFile returns the path to the tool's configuration file
func GetPathToSyMPIConfigFile() string {
	return filepath.Join(syfs.ConfigDir(), "singularity-mpi.conf")
}

func saveMPIConfigFile(path string, data []string) error {
	buffer := &bytes.Buffer{}
	err := gob.NewEncoder(buffer).Encode(data)
	if err != nil {
		return fmt.Errorf("unable to convert configuration data: %s", err)
	}
	d := buffer.Bytes()
	err = ioutil.WriteFile(path, d, 0644)
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
		buildPrivilegeEntry = BuildPrivilegeKey + " = false\n"
	}

	data := []string{buildPrivilegeEntry}

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
	log.Printf("-> Creating MPI configuration file: %s", syMPIConfigFile)
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

	if kv.GetValue(kvs, key) == value {
		log.Printf("Key %s from %s already set to %s", key, configFile, value)
		return nil
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
