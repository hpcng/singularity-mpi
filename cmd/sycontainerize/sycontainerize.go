// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package main

import (
	"flag"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strconv"

	"github.com/sylabs/singularity-mpi/internal/pkg/checker"
	"github.com/sylabs/singularity-mpi/internal/pkg/kv"
	"github.com/sylabs/singularity-mpi/internal/pkg/launcher"
	"github.com/sylabs/singularity-mpi/internal/pkg/sy"
	"github.com/sylabs/singularity-mpi/internal/pkg/sys"
	util "github.com/sylabs/singularity-mpi/internal/pkg/util/file"
	"github.com/sylabs/singularity-mpi/pkg/containerizer"
)

func main() {

	/* Argument parsing */
	verbose := flag.Bool("v", false, "Enable verbose mode")
	debug := flag.Bool("d", false, "Enable debug mode")
	appContainizer := flag.String("conf", "", "Path to the configuration file for automatically containerization an application")
	upload := flag.Bool("upload", false, "Upload generated images (appropriate configuration files need to specify the registry's URL")
	noinstall := flag.Bool("noinstall", false, "Keep the MPI installations on the host and the container images in the specified directory (instead of deleting everything once an experiment terminates). Default is '~/.sympi', set SYMPI_INSTALL_DIR to overwrite")

	flag.Parse()

	// Save the options passed in through the command flags
	// Initialize the log file. Log messages will both appear on stdout and the log file if the verbose option is used
	logFile := util.OpenLogFile("sycontainerize")
	defer logFile.Close()
	if *verbose || *debug {
		nultiWriters := io.MultiWriter(os.Stdout, logFile)
		log.SetOutput(nultiWriters)
	} else {
		log.SetOutput(ioutil.Discard)
	}

	sysCfg, _, _, err := launcher.Load()
	if err != nil {
		log.Fatalf("unable to load configuration: %s", err)

	}

	if *debug {
		sysCfg.Debug = true
		sysCfg.Verbose = true
		err = checker.CheckSystemConfig()
		if err != nil {
			log.Fatalf("the system is not correctly setup: %s", err)
		}
	}

	sysCfg.AppContainizer = *appContainizer
	sysCfg.Upload = *upload
	sysCfg.Verbose = *verbose
	sysCfg.Debug = *debug
	if !*noinstall {
		sysCfg.Persistent = sys.GetSympiDir()
	}

	// Check if we can figure out any detail about the installation of Singularity
	// that may change the way we use Singularity. For instance, do we need to use
	// sudo or fakeroot to create an image?
	sysCfg, err = sy.LookupConfig(&sysCfg)
	if err != nil {
		log.Fatalf("failed to get the Singularity configuration: %s", err)
	}

	// Make sure the tool's configuration file is set and load its data
	log.Println("* Loading the tool's configuration...")
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

	log.Println("* Creating container for your application...")
	_, err = containerizer.ContainerizeApp(&sysCfg)
	if err != nil {
		log.Fatalf("failed to create container for app: %s", err)
	}
}
