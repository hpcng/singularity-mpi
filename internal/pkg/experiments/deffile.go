// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package experiments

import (
	"fmt"
	"io/ioutil"
	"log"
	"path"
	"path/filepath"
	util "singularity-mpi/internal/pkg/util/file"
	"strings"
)

const (
	distroCodenameTag = "DISTROCODENAME"
)

// UpdateDefFileDistroCodename replace the tag for the distro codename in a definition file by the actual target distro codename
func UpdateDefFileDistroCodename(data, distro string) string {
	return strings.Replace(data, distroCodenameTag, distro, -1)
}

func doUpdateDefFile(myCfg *mpiConfig, sysCfg *SysConfig, compileCfg *compileConfig) error {
	var err error

	// Sanity checks
	if myCfg.mpiVersion == "" || myCfg.buildDir == "" || myCfg.url == "" ||
		myCfg.defFile == "" || compileCfg.mpiVersionTag == "" ||
		compileCfg.mpiURLTag == "" || compileCfg.mpiTarballTag == "" ||
		sysCfg.TargetUbuntuDistro == "" {
		return fmt.Errorf("invalid parameter(s)")
	}

	if myCfg.tarball == "" {
		myCfg.tarball = path.Base(myCfg.url)
	}

	data, err := ioutil.ReadFile(myCfg.defFile)
	if err != nil {
		return fmt.Errorf("failed to read %s: %s", myCfg.defFile, err)
	}

	var tarArgs string
	format := util.DetectTarballFormat(myCfg.tarball)
	switch format {
	case util.FormatBZ2:
		tarArgs = "-xjf"
	case util.FormatGZ:
		tarArgs = "-xzf"
	case util.FormatTAR:
		tarArgs = "-xf"
	default:
		return fmt.Errorf("un-supported tarball format for %s", myCfg.tarball)
	}

	if sysCfg.Debug {
		log.Printf("--> Replacing %s with %s", compileCfg.mpiVersionTag, myCfg.mpiVersion)
		log.Printf("--> Replacing %s with %s", compileCfg.mpiURLTag, myCfg.url)
		log.Printf("--> Replacing %s with %s", compileCfg.mpiTarballTag, myCfg.tarball)
		log.Printf("--> Replacing TARARGS with %s", tarArgs)
	}

	content := string(data)
	content = strings.Replace(content, compileCfg.mpiVersionTag, myCfg.mpiVersion, -1)
	content = strings.Replace(content, compileCfg.mpiURLTag, myCfg.url, -1)
	content = strings.Replace(content, compileCfg.mpiTarballTag, myCfg.tarball, -1)
	content = strings.Replace(content, "TARARGS", tarArgs, -1)
	content = UpdateDefFileDistroCodename(content, sysCfg.TargetUbuntuDistro)

	err = ioutil.WriteFile(myCfg.defFile, []byte(content), 0)
	if err != nil {
		return fmt.Errorf("failed to write file %s: %s", myCfg.defFile, err)
	}

	return nil
}

func generateDefFile(mpiCfg *mpiConfig, sysCfg *SysConfig) error {
	log.Println("- Generating Singularity defintion file...")
	// Sanity checks
	if mpiCfg.buildDir == "" {
		return fmt.Errorf("invalid parameter(s)")
	}

	var defFileName string
	var templateFileName string
	switch mpiCfg.mpiImplm {
	case "openmpi":
		defFileName = "ubuntu_ompi.def"
		if sysCfg.NetPipe {
			defFileName = "ubuntu_ompi_netpipe.def"
		}
		if sysCfg.IMB {
			defFileName = "ubuntu_ompi_imb.def"
		}
	case "mpich":
		defFileName = "ubuntu_mpich.def"
		if sysCfg.NetPipe {
			defFileName = "ubuntu_mpich_netpipe.def"
		}
		if sysCfg.IMB {
			defFileName = "ubuntu_mpich_imb.def"
		}
	case "intel":
		defFileName = "ubuntu_intel.def"
		if sysCfg.NetPipe {
			defFileName = "ubuntu_intel_netpipe.def"
		}
		if sysCfg.IMB {
			defFileName = "ubuntu_intel_imb.def"
		}
	default:
		return fmt.Errorf("unsupported MPI implementation: %s", mpiCfg.mpiImplm)
	}

	templateFileName = defFileName + ".tmpl"

	templateDefFile := filepath.Join(sysCfg.TemplateDir, templateFileName)
	mpiCfg.defFile = filepath.Join(mpiCfg.buildDir, defFileName)

	// Copy the definition file template to the temporary directory
	err := util.CopyFile(templateDefFile, mpiCfg.defFile)
	if err != nil {
		return fmt.Errorf("failed to copy %s to %s: %s", templateDefFile, mpiCfg.defFile, err)
	}

	// Copy the test file
	testFile := filepath.Join(sysCfg.TemplateDir, "mpitest.c")
	destTestFile := filepath.Join(mpiCfg.buildDir, "mpitest.c")
	err = util.CopyFile(testFile, destTestFile)
	if err != nil {
		return fmt.Errorf("failed to copy %s to %s: %s", testFile, destTestFile, err)
	}

	// Update the definition file for the specific version of MPI we are testing
	switch mpiCfg.mpiImplm {
	case "openmpi":
		err := updateOMPIDefFile(mpiCfg, sysCfg)
		if err != nil {
			return fmt.Errorf("failed to update OMPI template: %s", err)
		}
	case "mpich":
		err := updateMPICHDefFile(mpiCfg, sysCfg)
		if err != nil {
			return fmt.Errorf("failed to update MPICH template: %s", err)
		}
	case "intel":
		err := updateIntelMPIDefFile(mpiCfg, sysCfg)
		if err != nil {
			return fmt.Errorf("failed to update IMPI template: %s", err)
		}
	default:
		return fmt.Errorf("unsupported MPI implementation: %s", mpiCfg.mpiImplm)
	}

	// In debug mode, we save the def file that was generated to the scratch directory
	if sysCfg.Debug {
		backupFile := filepath.Join(sysCfg.ScratchDir, defFileName)
		log.Printf("-> Backing up %s to %s", mpiCfg.defFile, backupFile)
		err = util.CopyFile(mpiCfg.defFile, backupFile)
		if err != nil {
			log.Printf("-> error while backing up %s to %s", mpiCfg.defFile, backupFile)
		}
	}

	return nil
}
