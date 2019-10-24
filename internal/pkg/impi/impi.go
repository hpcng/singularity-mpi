// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package impi

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/sylabs/singularity-mpi/internal/pkg/buildenv"
	"github.com/sylabs/singularity-mpi/internal/pkg/deffile"
	"github.com/sylabs/singularity-mpi/internal/pkg/implem"
	"github.com/sylabs/singularity-mpi/internal/pkg/syexec"
	"github.com/sylabs/singularity-mpi/internal/pkg/sys"
	util "github.com/sylabs/singularity-mpi/internal/pkg/util/file"
)

// Constants related to Intel MPI
const (
	// IntelInstallPathPrefix is the prefix to use when referring to the installation directory for Intel MPI
	IntelInstallPathPrefix         = "compilers_and_libraries/linux/mpi/intel64"

	intelInstallConfFile           = "silent_install.cfg"
	intelUninstallConfFile         = "silent_uninstall.cfg"
	intelInstallConfFileTemplate   = intelInstallConfFile + ".tmpl"
	intelUninstallConfFileTemplate = intelUninstallConfFile + ".tmpl"

	// VersionTag is the tag used to refer to the MPI version in the IMPI template(s)
	VersionTag           = "IMpiVersion"
	// TarballTag is the tag used to refer to the tarball of IMPI in the IMPI template(s)
	TarballTag           = "IMPITARBALL"
	// DirTag is the tag used to refer to the directory where IMPI is installed
	DirTag               = "IMPIDIR" // todo: Should be removed
	// InstallConffileTag is the tag used to refer to the path for the script to use to install IMPI
	InstallConffileTag   = "IMPIINSTALLCONFFILE"
	// UninstallConffileTag is the tag used to refer to the path for the script to use to uninstall IMPI
	UninstallConffileTag = "IMPIUNINSTALLCONFFILE"
	// IfnetTag is the tag used to refer to the network interface in the IMPI template(s)
	IfnetTag             = "NETWORKINTERFACE"
)

// Config represents a configuration of Intel MPI
type Config struct {
	// DefFile is the path to the definition file for a IMPI based container
	DefFile string
	// Info gathers all the information about the version of IMPI to use
	Info    implem.Info
}

// GetDeffileTemplateTags returns all the tags used in IMPI template files
func GetDeffileTemplateTags() deffile.TemplateTags {
	var tags deffile.TemplateTags
	tags.Version = VersionTag
	tags.Tarball = TarballTag
	tags.Dir = DirTag
	tags.InstallConffile = InstallConffileTag
	tags.UninstallConffile = UninstallConffileTag
	tags.Ifnet = IfnetTag
	return tags
}

func updateIntelMPIDefFile(impiCfg *Config, env *buildenv.Info, sysCfg *sys.Config) error {
	// Intel MPI is very special so we have IMPI-specific code and it is okay

	// First we need to specialy prepare install & uninstall configuration file
	// Assumptions:
	// - code unpacked in /tmp/impi
	// - code installed in /opt/impi (but remember that the binaries and libraries are deep a sub-directory)
	const (
		containerIMPIInstallDir = "/opt/impi"
	)

	if impiCfg.Info.Tarball == "" {
		impiCfg.Info.Tarball = path.Base(impiCfg.Info.URL)
	}

	// Sanity checks
	if impiCfg.Info.Version == "" || impiCfg.Info.Tarball == "" || impiCfg.DefFile == "" {
		return fmt.Errorf("invalid parameter(s)")
	}

	// Copy the install & uninstall configuration file to the temporary directory used to build the container
	// These install &uninstall configuation file will be used wihtin the container to install IMPI
	srcInstallConfFile := filepath.Join(sysCfg.TemplateDir, "intel", intelInstallConfFileTemplate)
	destInstallConfFile := filepath.Join(env.BuildDir, intelInstallConfFile)
	srcUninstallConfFile := filepath.Join(sysCfg.TemplateDir, "intel", intelUninstallConfFileTemplate)
	destUninstallConfFile := filepath.Join(env.BuildDir, intelUninstallConfFile)
	err := util.CopyFile(srcInstallConfFile, destInstallConfFile)
	if err != nil {
		return fmt.Errorf("enable to copy %s to %s: %s", srcInstallConfFile, destInstallConfFile, err)
	}
	err = util.CopyFile(srcUninstallConfFile, destUninstallConfFile)
	if err != nil {
		return fmt.Errorf("enable to copy %s to %s: %s", srcUninstallConfFile, destUninstallConfFile, err)
	}

	err = updateTemplate(destInstallConfFile, containerIMPIInstallDir)
	if err != nil {
		return fmt.Errorf("unable to update IMPI template %s: %s", destInstallConfFile, err)
	}

	err = updateTemplate(destUninstallConfFile, containerIMPIInstallDir)
	if err != nil {
		return fmt.Errorf("unable to update IMPI template %s: %s", destUninstallConfFile, err)
	}

	// Then we have to put together a valid def file
	data, err := ioutil.ReadFile(impiCfg.DefFile)
	if err != nil {
		return fmt.Errorf("failed to read %s: %s", impiCfg.DefFile, err)
	}

	content := string(data)
	content = strings.Replace(content, VersionTag, impiCfg.Info.Version, -1)
	content = strings.Replace(content, TarballTag, impiCfg.Info.URL[7:], -1)
	content = strings.Replace(content, DirTag, filepath.Join(containerIMPIInstallDir, impiCfg.Info.Version, IntelInstallPathPrefix), -1)
	content = strings.Replace(content, InstallConffileTag, intelInstallConfFile, -1)
	content = strings.Replace(content, UninstallConffileTag, intelUninstallConfFile, -1)
	content = strings.Replace(content, IfnetTag, sysCfg.Ifnet, -1)

	err = ioutil.WriteFile(impiCfg.DefFile, []byte(content), 0)
	if err != nil {
		return fmt.Errorf("failed to write file %s: %s", impiCfg.DefFile, err)
	}

	return nil
}

func updateTemplate(filepath string, destMPIInstall string) error {
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

// updateTemplates updates the Intel install/uninstall scripts for the installation on the host
func updateTemplates(env *buildenv.Info, sysCfg *sys.Config) error {
	// Sanity checks
	if env.SrcDir == "" || env.BuildDir == "" {
		return fmt.Errorf("Invalid parameter(s)")
	}

	intelSilentInstallConfig := filepath.Join(env.SrcDir, intelInstallConfFile)
	intelSilentUninstallConfig := filepath.Join(env.SrcDir, intelUninstallConfFile)

	err := updateTemplate(intelSilentInstallConfig, env.BuildDir)
	if err != nil {
		return fmt.Errorf("failed to update template %s: %s", intelSilentInstallConfig, err)
	}

	err = updateTemplate(intelSilentUninstallConfig, env.BuildDir)
	if err != nil {
		return fmt.Errorf("failed to update template %s: %s", intelSilentUninstallConfig, err)
	}

	return nil
}

// SetupIntelInstallScript creates the install script for Intel MPI
func SetupInstallScript(env *buildenv.Info, sysCfg *sys.Config) error {
	// Copy silent script templates to install Intel MPI
	intelSilentInstallTemplate := filepath.Join(sysCfg.TemplateDir, "intel", intelInstallConfFileTemplate)
	intelSilentInstallConfig := filepath.Join(env.SrcDir, intelInstallConfFile)
	fmt.Printf("Copying %s to %s\n", intelSilentInstallTemplate, intelSilentInstallConfig)
	err := util.CopyFile(intelSilentInstallTemplate, intelSilentInstallConfig)
	if err != nil {
		return fmt.Errorf("failed to copy %s to %s: %s", intelSilentInstallTemplate, intelSilentInstallConfig, err)
	}

	intelSilentUninstallTemplate := filepath.Join(sysCfg.TemplateDir, "intel", intelUninstallConfFileTemplate)
	intelSilentUninstallConfig := filepath.Join(env.SrcDir, intelUninstallConfFile)
	err = util.CopyFile(intelSilentUninstallTemplate, intelSilentUninstallConfig)
	if err != nil {
		return fmt.Errorf("failed to copy %s to %s: %s", intelSilentUninstallTemplate, intelSilentUninstallConfig, err)
	}

	// Update the templates
	err = updateTemplates(env, sysCfg)
	if err != nil {
		return fmt.Errorf("unable to update Intel templates: %s", err)
	}

	return nil
}

// RunScript executes a install/uninstall script
func RunScript(env *buildenv.Info, sysCfg *sys.Config, phase string) syexec.Result {
	var configFile string
	var res syexec.Result

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
	cmd.Dir = env.SrcDir
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout
	res.Err = cmd.Run()
	res.Stderr = stderr.String()
	res.Stdout = stdout.String()

	return res
}

// GetExtraMpirunArgs returns all the required additional arguments required to use
// mpirun for a given configuration of MPI
func IntelGetExtraMpirunArgs(mpiCfg *Config, sys *sys.Config) []string {
	// Intel MPI is based on OFI so even for a simple TCP test, we need some extra arguments
	return []string{"-env", "FI_PROVIDER", "socket", "-env", "I_MPI_FABRICS", "ofi"}
}

// IntelGetConfigureExtraArgs returns the extra arguments required to configure IMPI
func IntelGetConfigureExtraArgs() []string {
	return nil
}

// GetPathToMpirun returns the path to mpirun when using IMPI
func GetPathToMpirun(env *buildenv.Info) string {
	return filepath.Join(env.BuildDir, IntelInstallPathPrefix, "bin/mpiexec")
}
