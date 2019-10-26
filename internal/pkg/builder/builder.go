// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package builder

import (
	"fmt"
	"log"
	"path/filepath"

	"github.com/sylabs/singularity-mpi/internal/pkg/app"

	"github.com/sylabs/singularity-mpi/internal/pkg/autotools"
	"github.com/sylabs/singularity-mpi/internal/pkg/buildenv"
	"github.com/sylabs/singularity-mpi/internal/pkg/container"
	"github.com/sylabs/singularity-mpi/internal/pkg/deffile"
	"github.com/sylabs/singularity-mpi/internal/pkg/impi"
	"github.com/sylabs/singularity-mpi/internal/pkg/implem"
	"github.com/sylabs/singularity-mpi/internal/pkg/jm"
	"github.com/sylabs/singularity-mpi/internal/pkg/mpich"
	"github.com/sylabs/singularity-mpi/internal/pkg/openmpi"
	"github.com/sylabs/singularity-mpi/internal/pkg/syexec"
	"github.com/sylabs/singularity-mpi/internal/pkg/sys"
	util "github.com/sylabs/singularity-mpi/internal/pkg/util/file"
)

const (
	DefaultUbuntuDistro = "ubuntu:disco"
)

// GetConfigureExtraArgsFn is the function prootype for getting extra arguments to configure a software
type GetConfigureExtraArgsFn func(*sys.Config) []string

// ConfigureFn is the function prototype to configuration a specific software
type ConfigureFn func(*buildenv.Info, *sys.Config, []string) error

// GetDeffileTemplateTagsFn is a "function pointer" to get the tags used in the definition file template for a given implementation of MPI
type GetDeffileTemplateTagsFn func() deffile.TemplateTags

// Builder gathers all the data specific to a software builder
type Builder struct {
	// Configure is the function to call to configure the software
	Configure ConfigureFn
	// GetConfigureExtraArgs is the function to call to get extra arguments for the configuration command
	GetConfigureExtraArgs GetConfigureExtraArgsFn
	// GetDeffileTemplateTags is the function to call to get all template tags
	GetDeffileTemplateTags GetDeffileTemplateTagsFn
}

// GenericConfigure is a generic function to configure a software, basically a wrapper around autotool's configure
func GenericConfigure(env *buildenv.Info, sysCfg *sys.Config, extraArgs []string) error {
	var ac autotools.Config
	ac.Install = env.InstallDir
	ac.Source = env.SrcDir
	err := autotools.Configure(&ac)
	if err != nil {
		return fmt.Errorf("failed to configure MPI: %s", err)
	}

	return nil
}

func (b *Builder) compile(mpiCfg *implem.Info, env *buildenv.Info, sysCfg *sys.Config) syexec.Result {
	var res syexec.Result

	log.Println("- Compiling MPI...")
	if env.SrcDir == "" {
		res.Err = fmt.Errorf("invalid parameter(s)")
		return res
	}

	makefilePath := filepath.Join(env.SrcDir, "Makefile")
	if util.FileExists(makefilePath) {
		res.Err = env.RunMake("")
		return res
	}

	fmt.Println("-> No Makefile, trying to figure out how to compile/install MPI...")
	if mpiCfg.ID == implem.IMPI {
		res.Err = impi.SetupInstallScript(env, sysCfg)
		if res.Err != nil {
			return res
		}
		return impi.RunScript(env, sysCfg, "install")
	}

	return res
}

func (b *Builder) install(mpiCfg *implem.Info, env *buildenv.Info, sysCfg *sys.Config) syexec.Result {
	var res syexec.Result

	if mpiCfg.ID == implem.IMPI {
		fmt.Println("-> Intel MPI detected, no install step, compile step installed the software...")
	}

	log.Printf("- Installing MPI in %s...", env.InstallDir)
	if env.InstallDir == "" || env.BuildDir == "" {
		res.Err = fmt.Errorf("invalid parameter(s)")
		return res
	}

	makefilePath := filepath.Join(env.SrcDir, "Makefile")
	if util.FileExists(makefilePath) {
		res.Err = env.RunMake("install")
		return res
	}

	return res
}

// InstallHost installs a specific version of MPI on the host
func (b *Builder) InstallHost(mpiCfg *implem.Info, jobmgr *jm.JM, env *buildenv.Info, sysCfg *sys.Config) syexec.Result {
	var res syexec.Result

	// Sanity checks
	if env.InstallDir == "" || mpiCfg.URL == "" {
		res.Err = fmt.Errorf("invalid parameter(s)")
		return res
	}

	log.Println("Installing MPI on host...")
	if sysCfg.Persistent != "" && util.PathExists(env.InstallDir) {
		log.Printf("* %s already exists, skipping installation...\n", env.InstallDir)
		return res
	}

	log.Printf("* %s does not exists, installing from scratch\n", env.InstallDir)
	res.Err = env.Get(mpiCfg)
	if res.Err != nil {
		res.Err = fmt.Errorf("failed to download MPI from %s: %s", mpiCfg.URL, res.Err)
		return res
	}

	res.Err = env.Unpack()
	if res.Err != nil {
		res.Err = fmt.Errorf("failed to unpack MPI: %s", res.Err)
		return res
	}

	// Right now, we assume we do not have to install autotools, which is a bad assumption
	extraArgs := b.GetConfigureExtraArgs(sysCfg)
	res.Err = b.Configure(env, sysCfg, extraArgs)
	if res.Err != nil {
		res.Err = fmt.Errorf("failed to configure MPI: %s", res.Err)
		return res
	}

	res = b.compile(mpiCfg, env, sysCfg)
	if res.Err != nil {
		res.Stderr = fmt.Sprintf("failed to compile MPI: %s", res.Err)
		return res
	}

	res = b.install(mpiCfg, env, sysCfg)
	if res.Err != nil {
		res.Stderr = fmt.Sprintf("failed to install MPI: %s", res.Err)
		return res
	}

	return res
}

// UninstallHost uninstalls a version of MPI on the host that was previously installed by our tool
func (b *Builder) UninstallHost(mpiCfg *implem.Info, env *buildenv.Info, sysCfg *sys.Config) syexec.Result {
	var res syexec.Result

	if sysCfg.Persistent == "" {
		log.Println("Uninstalling MPI on host...")

		if mpiCfg.ID == implem.IMPI {
			return impi.RunScript(env, sysCfg, "uninstall")
		}
	} else {
		log.Printf("Persistent installs mode, not uninstalling MPI from host")
	}

	return res
}

// Load is the function that will figure out the function to call for various stages of the code configuration/compilation/installation/execution
func Load(mpiCfg *implem.Info) (Builder, error) {
	var builder Builder
	builder.Configure = GenericConfigure

	switch mpiCfg.ID {
	case implem.OMPI:
		builder.Configure = openmpi.Configure
		builder.GetConfigureExtraArgs = openmpi.GetExtraConfigureArgs
		//		builder.GetMpirunExtraArgs = openmpi.GetMpirunExtraArgs // deprecated
		builder.GetDeffileTemplateTags = openmpi.GetDeffileTemplateTags
	case implem.MPICH:
		builder.GetDeffileTemplateTags = mpich.GetDeffileTemplateTags
	case implem.IMPI:
		builder.GetDeffileTemplateTags = impi.GetDeffileTemplateTags
	}

	return builder, nil
}

func (b *Builder) createDefFileFromTemplate(defFileName string, mpiCfg *implem.Info, env *buildenv.Info, container *container.Config, sysCfg *sys.Config) (deffile.DefFileData, error) {
	var f deffile.DefFileData

	templateFileName := defFileName + ".tmpl"
	templateDefFile := filepath.Join(sysCfg.TemplateDir, templateFileName)
	container.DefFile = filepath.Join(env.BuildDir, defFileName)

	// Copy the definition file template to the temporary directory
	err := util.CopyFile(templateDefFile, container.DefFile)
	if err != nil {
		return f, fmt.Errorf("failed to copy %s to %s: %s", templateDefFile, container.DefFile, err)
	}

	// Copy the test file
	// todo: rely on app info instead of hardcoding
	testFile := filepath.Join(sysCfg.TemplateDir, "mpitest.c")
	destTestFile := filepath.Join(env.BuildDir, "mpitest.c")
	err = util.CopyFile(testFile, destTestFile)
	if err != nil {
		return f, fmt.Errorf("failed to copy %s to %s: %s", testFile, destTestFile, err)
	}

	// Update the definition file for the specific version of MPI we are testing
	f.Path = container.DefFile
	f.MpiImplm = mpiCfg
	f.InternalEnv = env
	f.Tags = b.GetDeffileTemplateTags()
	err = deffile.UpdateDeffileTemplate(f, sysCfg)
	if err != nil {
		return f, fmt.Errorf("unable to generate definition file from template: %s", err)
	}

	return f, nil
}

// GenerateDeffile generates the definition file for a MPI container.
func (b *Builder) GenerateDeffile(appInfo *app.Info, mpiCfg *implem.Info, env *buildenv.Info, container *container.Config, sysCfg *sys.Config) error {
	log.Println("- Generating Singularity definition file...")
	// Sanity checks
	if env.BuildDir == "" {
		return fmt.Errorf("invalid parameter(s)")
	}

	var defFileName string
	var f deffile.DefFileData
	var err error

	// For IMPI, we generate the definition file from a template, for other MPI implementations,
	// we create a definition file from scratch
	if mpiCfg.ID == implem.IMPI {
		defFileName = "ubuntu_intel.def"
		if sysCfg.NetPipe {
			defFileName = "ubuntu_intel_netpipe.def"
		}
		if sysCfg.IMB {
			defFileName = "ubuntu_intel_imb.def"
		}
		f, err = b.createDefFileFromTemplate(defFileName, mpiCfg, env, container, sysCfg)
		if err != nil {
			return fmt.Errorf("failed to create definition file from template: %s", err)
		}
	} else {
		defFileName = "ubuntu_" + mpiCfg.ID + "_" + appInfo.Name + ".def"
		container.DefFile = filepath.Join(env.BuildDir, defFileName)

		f.Distro = DefaultUbuntuDistro
		f.InternalEnv = env
		f.MpiImplm = mpiCfg
		f.Path = container.DefFile

		err = deffile.CreateDefaultDefFile(appInfo, &f, sysCfg)
		if err != nil {
			return fmt.Errorf("failed to create definition file: %s", err)
		}
	}

	log.Printf("-> Definition file created: %s\n", f.Path)

	// In debug mode, we save the def file that was generated to the scratch directory
	if sysCfg.Debug {
		backupFile := filepath.Join(sysCfg.ScratchDir, defFileName)
		log.Printf("-> Backing up %s to %s", f.Path, backupFile)
		err := util.CopyFile(f.Path, backupFile)
		if err != nil {
			log.Printf("-> error while backing up %s to %s", f.Path, backupFile)
		}
	}

	return nil
}
