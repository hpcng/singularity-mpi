// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package deffile

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/sylabs/singularity-mpi/internal/pkg/buildenv"

	"github.com/sylabs/singularity-mpi/internal/pkg/implem"

	"github.com/sylabs/singularity-mpi/internal/pkg/app"
	"github.com/sylabs/singularity-mpi/internal/pkg/sys"
)

func TestCreateDefFile(t *testing.T) {
	var sysCfg sys.Config

	curDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get the current work directory: %s", err)
	}
	sysCfg.BinPath = filepath.Join(curDir, "../../..")
	sysCfg.EtcDir = filepath.Join(sysCfg.BinPath, "etc")
	sysCfg.TemplateDir = filepath.Join(sysCfg.EtcDir, "templates")

	netpipe := app.GetNetpipe(&sysCfg)
	imb := app.GetIMB(&sysCfg)
	helloworld := app.GetHelloworld(&sysCfg)

	tempDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("failed to create temporary directory: %s", err)
	}
	defer os.RemoveAll(tempDir)

	var openmpi implem.Info
	openmpi.ID = implem.OMPI
	openmpi.URL = "https://download.open-mpi.org/release/open-mpi/v3.1/openmpi-3.1.4.tar.bz2"
	openmpi.Tarball = "openmpi-3.1.4.tar.bz2"
	openmpi.Version = "3.1.4"

	var helloworldEnv buildenv.Info
	helloworldEnv.InstallDir = ""
	helloworldEnv.SrcDir = "/opt"

	var netpipeEnv buildenv.Info
	netpipeEnv.InstallDir = ""
	netpipeEnv.SrcDir = "/opt"

	var imbEnv buildenv.Info
	imbEnv.InstallDir = ""
	imbEnv.SrcDir = "/opt"

	var helloworldData DefFileData
	helloworldData.Path = filepath.Join(tempDir, "helloworld.def")
	helloworldData.Distro = "ubuntu:disco"
	helloworldData.MpiImplm = &openmpi
	helloworldData.InternalEnv = &helloworldEnv

	var netpipeData DefFileData
	netpipeData.Path = filepath.Join(tempDir, "netpipe.def")
	netpipeData.Distro = "ubuntu:disco"
	netpipeData.MpiImplm = &openmpi
	netpipeData.InternalEnv = &netpipeEnv

	var imbData DefFileData
	imbData.Path = filepath.Join(tempDir, "imb.def")
	imbData.Distro = "ubuntu:disco"
	imbData.MpiImplm = &openmpi
	imbData.InternalEnv = &imbEnv

	err = CreateHybridDefFile(&helloworld, &helloworldData, &sysCfg)
	if err != nil {
		t.Fatalf("failed to create definition file for helloworld: %s", err)
	}

	err = CreateHybridDefFile(&netpipe, &netpipeData, &sysCfg)
	if err != nil {
		t.Fatalf("failed to create definition file for netpipe: %s", err)
	}

	err = CreateHybridDefFile(&imb, &imbData, &sysCfg)
	if err != nil {
		t.Fatalf("failed to create definition file for IMB: %s", err)
	}

	fmt.Printf("Definition files are in %s", tempDir)
}
