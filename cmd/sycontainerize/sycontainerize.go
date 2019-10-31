// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package main

import (
	"flag"
	"log"
	"strconv"

	"github.com/sylabs/singularity-mpi/internal/pkg/checker"
	"github.com/sylabs/singularity-mpi/internal/pkg/kv"
	"github.com/sylabs/singularity-mpi/internal/pkg/launcher"
	"github.com/sylabs/singularity-mpi/internal/pkg/sys"
	util "github.com/sylabs/singularity-mpi/internal/pkg/util/file"
	"github.com/sylabs/singularity-mpi/internal/pkg/util/sy"
	"github.com/sylabs/singularity-mpi/pkg/containizer"
)

func main() {
	sysCfg, _, _, err := launcher.Load()
	if err != nil {
		log.Fatalf("unable to load configuration: %s", err)

	}

	/* Argument parsing */
	verbose := flag.Bool("v", false, "Enable verbose mode")
	debug := flag.Bool("d", false, "Enable debug mode")
	appContainizer := flag.String("app-containizer", "", "Path to the configuration file for automatically containerization an application")
	upload := flag.Bool("upload", false, "Upload generated images (appropriate configuration files need to specify the registry's URL")
	persistent := flag.Bool("persistent-installs", false, "Keep the MPI installations on the host and the container images in the specified directory (instead of deleting everything once an experiment terminates). Default is '~/.sympi', set SYMPI_INSTALL_DIR to overwrite")

	flag.Parse()

	sysCfg.AppContainizer = *appContainizer
	sysCfg.Upload = *upload
	sysCfg.Verbose = *verbose
	sysCfg.Debug = *debug
	if *persistent {
		sysCfg.Persistent = sys.GetSympiDir()
	}

	// Make sure the tool's configuration file is set and load its data
	toolConfigFile, err := sy.CreateMPIConfigFile()
	if err != nil {
		log.Fatalf("cannot setup configuration file: %s", err)
	}
	kvs, err := kv.LoadKeyValueConfig(toolConfigFile)
	if err != nil {
		log.Fatalf("cannot load the tool's configuration file (%s): %s", toolConfigFile, err)
	}
	var syConfig sy.MPIToolConfig
	syConfig.BuildPrivilege, err = strconv.ParseBool(kv.GetValue(kvs, sy.BuildPrivilegeKey))
	if err != nil {
		log.Fatalf("failed to load the tool's configuration: %s", err)
	}

	// Save the options passed in through the command flags
	if sysCfg.Debug {
		sysCfg.Verbose = true
		// If the scratch dir exists, we delete it to start fresh
		err := util.DirInit(sysCfg.ScratchDir)
		if err != nil {
			log.Fatalf("failed to initialize directory %s: %s", sysCfg.ScratchDir, err)
		}

		err = checker.CheckSystemConfig()
		if err != nil {
			log.Fatalf("the system is not correctly setup: %s", err)
		}
	}

	_, err = containizer.ContainerizeApp(&sysCfg)
	if err != nil {
		log.Fatalf("failed to create container for app: %s", err)
	}
}
