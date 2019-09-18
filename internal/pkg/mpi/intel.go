// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package mpi

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/sylabs/singularity-mpi/internal/pkg/sys"
	util "github.com/sylabs/singularity-mpi/internal/pkg/util/file"
)

// Constants related to Intel MPI
const (
	IntelInstallPathPrefix         = "compilers_and_libraries/linux/mpi/intel64"
	intelInstallConfFile           = "silent_install.cfg"
	intelUninstallConfFile         = "silent_uninstall.cfg"
	intelInstallConfFileTemplate   = intelInstallConfFile + ".tmpl"
	intelUninstallConfFileTemplate = intelUninstallConfFile + ".tmpl"
)

func updateIntelMPIDefFile(mpiCfg *Config, sysCfg *sys.Config) error {
	// Intel MPI is very special so we have IMPI-specific code and it is okay

	// First we need to specialy prepare install & uninstall configuration file
	// Assumptions:
	// - code unpacked in /tmp/impi
	// - code installed in /opt/impi (but remember that the binaries and libraries are deep a sub-directory)
	const (
		containerIMPIInstallDir = "/opt/impi"
	)

	if mpiCfg.tarball == "" {
		mpiCfg.tarball = path.Base(mpiCfg.URL)
	}

	// Sanity checks
	if mpiCfg.MpiVersion == "" || mpiCfg.tarball == "" || mpiCfg.DefFile == "" {
		return fmt.Errorf("invalid parameter(s)")
	}

	// Copy the install & uninstall configuration file to the temporary directory used to build the container
	// These install &uninstall configuation file will be used wihtin the container to install IMPI
	srcInstallConfFile := filepath.Join(sysCfg.TemplateDir, "intel", intelInstallConfFileTemplate)
	destInstallConfFile := filepath.Join(mpiCfg.BuildDir, intelInstallConfFile)
	srcUninstallConfFile := filepath.Join(sysCfg.TemplateDir, "intel", intelUninstallConfFileTemplate)
	destUninstallConfFile := filepath.Join(mpiCfg.BuildDir, intelUninstallConfFile)
	err := util.CopyFile(srcInstallConfFile, destInstallConfFile)
	if err != nil {
		return fmt.Errorf("enable to copy %s to %s: %s", srcInstallConfFile, destInstallConfFile, err)
	}
	err = util.CopyFile(srcUninstallConfFile, destUninstallConfFile)
	if err != nil {
		return fmt.Errorf("enable to copy %s to %s: %s", srcUninstallConfFile, destUninstallConfFile, err)
	}

	err = updateIntelTemplate(destInstallConfFile, containerIMPIInstallDir)
	if err != nil {
		return fmt.Errorf("unable to update IMPI template %s: %s", destInstallConfFile, err)
	}

	err = updateIntelTemplate(destUninstallConfFile, containerIMPIInstallDir)
	if err != nil {
		return fmt.Errorf("unable to update IMPI template %s: %s", destUninstallConfFile, err)
	}

	// Then we have to put together a valid def file
	data, err := ioutil.ReadFile(mpiCfg.DefFile)
	if err != nil {
		return fmt.Errorf("failed to read %s: %s", mpiCfg.DefFile, err)
	}

	content := string(data)
	content = strings.Replace(content, "IMpiVersion", mpiCfg.MpiVersion, -1)
	content = strings.Replace(content, "IMPITARBALL", mpiCfg.URL[7:], -1)
	content = strings.Replace(content, "IMPIDIR", filepath.Join(containerIMPIInstallDir, mpiCfg.MpiVersion, IntelInstallPathPrefix), -1)
	content = strings.Replace(content, "IMPIINSTALLCONFFILE", intelInstallConfFile, -1)
	content = strings.Replace(content, "IMPIUNINSTALLCONFFILE", intelUninstallConfFile, -1)
	content = strings.Replace(content, "NETWORKINTERFACE", sysCfg.Ifnet, -1)

	err = ioutil.WriteFile(mpiCfg.DefFile, []byte(content), 0)
	if err != nil {
		return fmt.Errorf("failed to write file %s: %s", mpiCfg.DefFile, err)
	}

	return nil
}

func updateIntelTemplate(filepath string, destMPIInstall string) error {
	data, err := ioutil.ReadFile(filepath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %s", filepath, err)
	}
	content := string(data)
	content = strings.Replace(content, "MPIINSTALLDIR", destMPIInstall, -1)
	err = ioutil.WriteFile(filepath, []byte(content), 0)
	if err != nil {
		return fmt.Errorf("failed to write file %s: %s", filepath, err)
	}
	return nil
}

// This function updated the Intel install/uninstall scripts for the installation on the host
func updateIntelTemplates(mpiCfg *Config, sysCfg *sys.Config) error {
	// Sanity checks
	if mpiCfg.srcDir == "" || mpiCfg.BuildDir == "" {
		return fmt.Errorf("Invalid parameter(s)")
	}

	intelSilentInstallConfig := filepath.Join(mpiCfg.srcDir, intelInstallConfFile)
	intelSilentUninstallConfig := filepath.Join(mpiCfg.srcDir, intelUninstallConfFile)

	err := updateIntelTemplate(intelSilentInstallConfig, mpiCfg.BuildDir)
	if err != nil {
		return fmt.Errorf("failed to update template %s: %s", intelSilentInstallConfig, err)
	}

	err = updateIntelTemplate(intelSilentUninstallConfig, mpiCfg.BuildDir)
	if err != nil {
		return fmt.Errorf("failed to update template %s: %s", intelSilentUninstallConfig, err)
	}

	return nil
}

// SetupIntelInstallScript creates the install script for Intel MPI
func SetupIntelInstallScript(mpiCfg *Config, sysCfg *sys.Config) error {
	// Copy silent script templates to install Intel MPI
	intelSilentInstallTemplate := filepath.Join(sysCfg.TemplateDir, "intel", intelInstallConfFileTemplate)
	intelSilentInstallConfig := filepath.Join(mpiCfg.srcDir, intelInstallConfFile)
	fmt.Printf("Copying %s to %s\n", intelSilentInstallTemplate, intelSilentInstallConfig)
	err := util.CopyFile(intelSilentInstallTemplate, intelSilentInstallConfig)
	if err != nil {
		return fmt.Errorf("failed to copy %s to %s: %s", intelSilentInstallTemplate, intelSilentInstallConfig, err)
	}

	intelSilentUninstallTemplate := filepath.Join(sysCfg.TemplateDir, "intel", intelUninstallConfFileTemplate)
	intelSilentUninstallConfig := filepath.Join(mpiCfg.srcDir, intelUninstallConfFile)
	err = util.CopyFile(intelSilentUninstallTemplate, intelSilentUninstallConfig)
	if err != nil {
		return fmt.Errorf("failed to copy %s to %s: %s", intelSilentUninstallTemplate, intelSilentUninstallConfig, err)
	}

	// Update the templates
	err = updateIntelTemplates(mpiCfg, sysCfg)
	if err != nil {
		return fmt.Errorf("unable to update Intel templates: %s", err)
	}

	return nil
}

func runIntelScript(mpiCfg *Config, sysCfg *sys.Config, phase string) ExecResult {
	var configFile string
	var res ExecResult

	fmt.Printf("Running %s script...\n", phase)

	switch phase {
	case "install":
		configFile = intelInstallConfFile
	case "uninstall":
		configFile = intelUninstallConfFile
	default:
		res.Err = fmt.Errorf("unknown phase: %s", phase)
		return res
	}

	// Run the install or uninstall script
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("./install.sh", "--silent", configFile)
	cmd.Dir = mpiCfg.srcDir
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout
	res.Err = cmd.Run()
	/*
		if err != nil {
			return fmt.Errorf("command failed: %s - stdout: %s - stderr: %s", err, stdout.String(), stderr.String())
		}
	*/
	res.Stderr = stderr.String()
	res.Stdout = stdout.String()

	return res
}
