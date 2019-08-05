// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package experiments

import (
	"fmt"
	"io/ioutil"
	"path"
	"path/filepath"
	"strings"
)

func updateIntelMPIDefFile(mpiCfg *mpiConfig, sysCfg *SysConfig) error {
	// Intel MPI is very special so we have IMPI-specific code and it is okay

	// First we need to specialy prepare install & uninstall configuration file
	// Assumptions:
	// - code unpacked in /tmp/impi
	// - code installed in /opt/impi (but remember that the binaries and libraries are deep a sub-directory)
	const (
		containerIMPIInstallDir = "/opt/impi"
	)

	if mpiCfg.tarball == "" {
		mpiCfg.tarball = path.Base(mpiCfg.url)
	}

	// Sanity checks
	if mpiCfg.mpiVersion == "" || mpiCfg.tarball == "" || mpiCfg.defFile == "" {
		return fmt.Errorf("invalid parameter(s)")
	}

	// Copy the install & uninstall configuration file to the temporary directory used to build the container
	// These install &uninstall configuation file will be used wihtin the container to install IMPI
	srcInstallConfFile := filepath.Join(sysCfg.TemplateDir, "intel", intelInstallConfFileTemplate)
	destInstallConfFile := filepath.Join(mpiCfg.buildDir, intelInstallConfFile)
	srcUninstallConfFile := filepath.Join(sysCfg.TemplateDir, "intel", intelUninstallConfFileTemplate)
	destUninstallConfFile := filepath.Join(mpiCfg.buildDir, intelUninstallConfFile)
	err := copyFile(srcInstallConfFile, destInstallConfFile)
	if err != nil {
		return fmt.Errorf("enable to copy %s to %s: %s", srcInstallConfFile, destInstallConfFile, err)
	}
	err = copyFile(srcUninstallConfFile, destUninstallConfFile)
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
	data, err := ioutil.ReadFile(mpiCfg.defFile)
	if err != nil {
		return fmt.Errorf("failed to read %s: %s", mpiCfg.defFile, err)
	}

	content := string(data)
	content = strings.Replace(content, "IMPIVERSION", mpiCfg.mpiVersion, -1)
	content = strings.Replace(content, "IMPITARBALL", mpiCfg.url[7:], -1)
	content = strings.Replace(content, "IMPIDIR", filepath.Join(containerIMPIInstallDir, mpiCfg.mpiVersion, intelInstallPathPrefix), -1)
	content = strings.Replace(content, "IMPIINSTALLCONFFILE", intelInstallConfFile, -1)
	content = strings.Replace(content, "IMPIUNINSTALLCONFFILE", intelUninstallConfFile, -1)
	content = strings.Replace(content, "NETWORKINTERFACE", sysCfg.Ifnet, -1)

	err = ioutil.WriteFile(mpiCfg.defFile, []byte(content), 0)
	if err != nil {
		return fmt.Errorf("failed to write file %s: %s", mpiCfg.defFile, err)
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
func updateIntelTemplates(mpiCfg *mpiConfig, sysCfg *SysConfig) error {
	// Sanity checks
	if mpiCfg.srcDir == "" || mpiCfg.buildDir == "" {
		return fmt.Errorf("Invalid parameter(s)")
	}

	intelSilentInstallConfig := filepath.Join(mpiCfg.srcDir, intelInstallConfFile)
	intelSilentUninstallConfig := filepath.Join(mpiCfg.srcDir, intelUninstallConfFile)

	err := updateIntelTemplate(intelSilentInstallConfig, mpiCfg.buildDir)
	if err != nil {
		return fmt.Errorf("failed to update template %s: %s", intelSilentInstallConfig, err)
	}

	err = updateIntelTemplate(intelSilentUninstallConfig, mpiCfg.buildDir)
	if err != nil {
		return fmt.Errorf("failed to update template %s: %s", intelSilentUninstallConfig, err)
	}

	return nil
}

func setupIntelInstallScript(mpiCfg *mpiConfig, sysCfg *SysConfig) error {
	// Copy silent script templates to install Intel MPI
	intelSilentInstallTemplate := filepath.Join(sysCfg.TemplateDir, "intel", intelInstallConfFileTemplate)
	intelSilentInstallConfig := filepath.Join(mpiCfg.srcDir, intelInstallConfFile)
	fmt.Printf("Copying %s to %s\n", intelSilentInstallTemplate, intelSilentInstallConfig)
	err := copyFile(intelSilentInstallTemplate, intelSilentInstallConfig)
	if err != nil {
		return fmt.Errorf("failed to copy %s to %s: %s", intelSilentInstallTemplate, intelSilentInstallConfig, err)
	}

	intelSilentUninstallTemplate := filepath.Join(sysCfg.TemplateDir, "intel", intelUninstallConfFileTemplate)
	intelSilentUninstallConfig := filepath.Join(mpiCfg.srcDir, intelUninstallConfFile)
	err = copyFile(intelSilentUninstallTemplate, intelSilentUninstallConfig)
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
