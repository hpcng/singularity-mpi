// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package launcher

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/sylabs/singularity-mpi/internal/pkg/app"
	"github.com/sylabs/singularity-mpi/internal/pkg/buildenv"
	"github.com/sylabs/singularity-mpi/internal/pkg/implem"
	"github.com/sylabs/singularity-mpi/internal/pkg/jm"
	"github.com/sylabs/singularity-mpi/internal/pkg/job"
	"github.com/sylabs/singularity-mpi/internal/pkg/kv"
	"github.com/sylabs/singularity-mpi/internal/pkg/mpi"
	"github.com/sylabs/singularity-mpi/internal/pkg/network"
	"github.com/sylabs/singularity-mpi/internal/pkg/results"
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
func prepareLaunchCmd(j *job.Job, jobmgr *jm.JM, hostEnv *buildenv.Info, sysCfg *sys.Config) (syexec.SyCmd, error) {
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
	cfg.EtcDir = filepath.Join(os.Getenv("GOPATH"), "src", "github.com", "sylabs", "singularity-mpi", "etc")
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
	cfg.SingularityBin, err = exec.LookPath("singularity")
	if err != nil {
		log.Printf("[WARN] failed to find the Singularity binary")
	}
	cfg.SudoBin, err = exec.LookPath("sudo")
	if err != nil {
		return cfg, jobmgr, net, fmt.Errorf("sudo not available: %s", err)
	}

	// Parse and load the sympi configuration file
	sympiKVs, err := sy.LoadMPIConfigFile()
	if err != nil {
		log.Printf("failed to run configuration from singularity-mpi configuration file: %s", err)
	}
	val := kv.GetValue(sympiKVs, sy.NoPrivKey)
	cfg.Nopriv = false
	nopriv, err := strconv.ParseBool(val)
	if nopriv {
		cfg.Nopriv = true
	}
	val = kv.GetValue(sympiKVs, sy.SudoCmdsKey)
	if val != "" {
		cfg.SudoSyCmds = strings.Split(val, " ")
	}

	// Load the job manager component first
	jobmgr = jm.Detect()

	// Load the network configuration
	_ = network.Detect(&cfg)

	return cfg, jobmgr, net, nil
}

// SaveErrorDetails gathers and stores execution details when the execution of a container failed.
func SaveErrorDetails(hostMPI *implem.Info, containerMPI *implem.Info, sysCfg *sys.Config, res *syexec.Result) error {
	experimentName := hostMPI.Version + "-" + containerMPI.Version
	targetDir := filepath.Join(sysCfg.BinPath, "errors", hostMPI.ID, experimentName)

	// If the directory exists, we delete it to start fresh
	err := util.DirInit(targetDir)
	if err != nil {
		return fmt.Errorf("impossible to initialize directory %s: %s", targetDir, err)
	}

	stderrFile := filepath.Join(targetDir, "stderr.txt")
	stdoutFile := filepath.Join(targetDir, "stdout.txt")

	fstderr, err := os.Create(stderrFile)
	if err != nil {
		return err
	}
	defer fstderr.Close()
	_, err = fstderr.WriteString(res.Stderr)
	if err != nil {
		return err
	}

	fstdout, err := os.Create(stdoutFile)
	if err != nil {
		return err
	}
	defer fstdout.Close()
	_, err = fstdout.WriteString(res.Stdout)
	if err != nil {
		return err
	}

	return nil
}

// Run executes a container with a specific version of MPI on the host
func Run(appInfo *app.Info, hostMPI *mpi.Config, hostBuildEnv *buildenv.Info, containerMPI *mpi.Config, jobmgr *jm.JM, sysCfg *sys.Config) (results.Result, syexec.Result) {
	var execRes syexec.Result
	var expRes results.Result

	// mpiJob describes the job
	var mpiJob job.Job
	mpiJob.HostCfg = &hostMPI.Implem
	mpiJob.Container = &containerMPI.Container
	mpiJob.App.BinPath = appInfo.BinPath
	mpiJob.NNodes = 2
	mpiJob.NP = 2

	// We submit the job
	var submitCmd syexec.SyCmd
	submitCmd, execRes.Err = prepareLaunchCmd(&mpiJob, jobmgr, hostBuildEnv, sysCfg)
	if execRes.Err != nil {
		execRes.Err = fmt.Errorf("failed to prepare the launch command: %s", execRes.Err)
		expRes.Pass = false
		return expRes, execRes
	}

	var stdout, stderr bytes.Buffer
	submitCmd.Cmd.Stdout = &stdout
	submitCmd.Cmd.Stderr = &stderr
	defer submitCmd.CancelFn()

	// Regex to catch errors where mpirun returns 0 but is known to have failed because displaying the help message
	var re = regexp.MustCompile(`^(\n?)Usage:`)

	err := submitCmd.Cmd.Run()
	// Get the command out/err
	execRes.Stderr = stderr.String()
	execRes.Stdout = stdout.String()
	// And add the job out/err (for when we actually use a real job manager such as Slurm)
	execRes.Stdout += mpiJob.GetOutput(&mpiJob, sysCfg)
	execRes.Stderr += mpiJob.GetError(&mpiJob, sysCfg)
	if err != nil || submitCmd.Ctx.Err() == context.DeadlineExceeded || re.Match(stdout.Bytes()) {
		log.Printf("[INFO] mpirun command failed - stdout: %s - stderr: %s - err: %s\n", stdout.String(), stderr.String(), err)
		execRes.Err = err
		err = SaveErrorDetails(&hostMPI.Implem, &containerMPI.Implem, sysCfg, &execRes)
		if err != nil {
			execRes.Err = fmt.Errorf("impossible to cleanly handle error: %s", err)
			expRes.Pass = false
			return expRes, execRes
		}
		expRes.Pass = false
		return expRes, execRes
	}

	expRes.Pass = true
	return expRes, execRes
}
