// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package launcher

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/sylabs/singularity-mpi/internal/pkg/buildenv"
	"github.com/sylabs/singularity-mpi/internal/pkg/jm"
	"github.com/sylabs/singularity-mpi/internal/pkg/job"
	"github.com/sylabs/singularity-mpi/internal/pkg/kv"
	"github.com/sylabs/singularity-mpi/internal/pkg/network"
	"github.com/sylabs/singularity-mpi/internal/pkg/slurm"
	"github.com/sylabs/singularity-mpi/internal/pkg/syexec"
	"github.com/sylabs/singularity-mpi/internal/pkg/sys"
	util "github.com/sylabs/singularity-mpi/internal/pkg/util/file"
	"github.com/sylabs/singularity-mpi/internal/pkg/util/sy"
)

// Info gathers all the details to start a job
type Info struct {
	// Cmd represents the command to launch a job
	Cmd syexec.SyCmd
}

// PrepareLaunchCmd interacts with a job manager backend to figure out how to launch a job
func PrepareLaunchCmd(j *job.Job, jobmgr *jm.JM, hostEnv *buildenv.Info, sysCfg *sys.Config) (syexec.SyCmd, error) {
	var cmd syexec.SyCmd

	launchCmd, err := jobmgr.Submit(j, hostEnv, sysCfg)
	if err != nil {
		return cmd, fmt.Errorf("failed to create a launcher object: %s", err)
	}
	log.Printf("* Command object for '%s %s' is ready", launchCmd.BinPath, strings.Join(launchCmd.CmdArgs, " "))

	cmd.Ctx, cmd.CancelFn = context.WithTimeout(context.Background(), sys.CmdTimeout*time.Minute)
	cmd.Cmd = exec.CommandContext(cmd.Ctx, launchCmd.BinPath, launchCmd.CmdArgs...)
	cmd.Cmd.Stdout = &j.OutBuffer
	cmd.Cmd.Stderr = &j.ErrBuffer

	return cmd, nil
}

// Load gathers all the details to start running experiments or create containers for apps
//
// todo: should be in a different package (but where?)
func Load() (sys.Config, jm.JM, network.Info, error) {
	var cfg sys.Config
	var jobmgr jm.JM
	var net network.Info

	/* Figure out the directory of this binary */
	bin, err := os.Executable()
	if err != nil {
		return cfg, jobmgr, net, fmt.Errorf("cannot detect the directory of the binary")
	}
	cfg.BinPath = filepath.Dir(bin)
	cfg.EtcDir = filepath.Join(cfg.BinPath, "etc")
	cfg.TemplateDir = filepath.Join(cfg.EtcDir, "templates")
	cfg.OfiCfgFile = filepath.Join(cfg.EtcDir, "ofi.conf")
	cfg.CurPath, err = os.Getwd()
	if err != nil {
		return cfg, jobmgr, net, fmt.Errorf("cannot detect current directory")
	}

	cfg.SyConfigFile = sy.GetPathToSyMPIConfigFile()
	if util.PathExists(cfg.SyConfigFile) {
		kvs, err := kv.LoadKeyValueConfig(cfg.SyConfigFile)
		if err != nil {
			return cfg, jobmgr, net, fmt.Errorf("unable to load the tool's configuration: %s", err)
		}
		if kv.GetValue(kvs, slurm.EnabledKey) != "" {
			cfg.SlurmEnabled, err = strconv.ParseBool(kv.GetValue(kvs, slurm.EnabledKey))
			if err != nil {
				return cfg, jobmgr, net, fmt.Errorf("failed to load the Slurm configuration: %s", err)
			}
		}
	} else {
		log.Println("-> Creating configuration file...")
		path, err := sy.CreateMPIConfigFile()
		if err != nil {
			return cfg, jobmgr, net, fmt.Errorf("failed to create configuration file: %s", err)
		}
		log.Printf("... %s successfully created\n", path)
	}

	// Load the job manager component first
	jobmgr = jm.Detect()

	// Load the network configuration
	_ = network.Detect(&cfg)

	return cfg, jobmgr, net, nil
}
