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

	"github.com/gvallee/go_util/pkg/util"
	"github.com/gvallee/kv/pkg/kv"
	"github.com/sylabs/singularity-mpi/internal/pkg/job"
	"github.com/sylabs/singularity-mpi/internal/pkg/network"
	"github.com/sylabs/singularity-mpi/internal/pkg/slurm"
	"github.com/sylabs/singularity-mpi/pkg/app"
	"github.com/sylabs/singularity-mpi/pkg/buildenv"
	"github.com/sylabs/singularity-mpi/pkg/implem"
	"github.com/sylabs/singularity-mpi/pkg/jm"
	"github.com/sylabs/singularity-mpi/pkg/mpi"
	"github.com/sylabs/singularity-mpi/pkg/results"
	"github.com/sylabs/singularity-mpi/pkg/sy"
	"github.com/sylabs/singularity-mpi/pkg/syexec"
	"github.com/sylabs/singularity-mpi/pkg/sys"
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
	cmd.Cmd.Env = launchCmd.Env

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
	cfg.EtcDir = filepath.Join(os.Getenv("GOPATH"), "etc")
	cfg.TemplateDir = filepath.Join(cfg.EtcDir, "templates")
	cfg.OfiCfgFile = filepath.Join(cfg.EtcDir, "sympi_ofi.conf")
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

func checkOutput(output string, expected string) bool {
	return strings.Contains(output, expected)
}

func checkJobOutput(output string, expectedOutput string, jobInfo *job.Job) bool {
	if jobInfo.NP > 0 {
		expected := strings.ReplaceAll(expectedOutput, "#NP", strconv.Itoa(jobInfo.NP))
		for i := 0; i < jobInfo.NP; i++ {
			curExpectedOutput := strings.ReplaceAll(expected, "#RANK", strconv.Itoa(i))
			if checkOutput(output, curExpectedOutput) {
				return true
			}
		}
		return false
	}
	return checkOutput(output, expectedOutput)
}

func expectedOutput(stdout string, stderr string, appInfo *app.Info, jobInfo *job.Job) bool {
	if appInfo.ExpectedRankOutput == "" {
		log.Println("App does not define any expected output, skipping check...")
		return true
	}

	// The output can be on stderr or stdout, we just cannot know in advanced.
	// For instance, some MPI applications sends output to stderr by default
	matched := checkJobOutput(stdout, appInfo.ExpectedRankOutput, jobInfo)
	if !matched {
		matched = checkJobOutput(stderr, appInfo.ExpectedRankOutput, jobInfo)
	}

	return matched
}

// Run executes a container with a specific version of MPI on the host
func Run(appInfo *app.Info, hostMPI *mpi.Config, hostBuildEnv *buildenv.Info, containerMPI *mpi.Config, jobmgr *jm.JM, sysCfg *sys.Config, args []string) (results.Result, syexec.Result) {
	var newjob job.Job
	var execRes syexec.Result
	var expRes results.Result
	expRes.Pass = true

	if hostMPI != nil {
		newjob.HostCfg = &hostMPI.Implem
	}

	if containerMPI != nil {
		newjob.Container = &containerMPI.Container
	}

	newjob.App.BinPath = appInfo.BinPath
	if len(args) == 0 {
		newjob.NNodes = 2
		newjob.NP = 2
	} else {
		newjob.Args = args
	}

	// We submit the job
	var submitCmd syexec.SyCmd
	submitCmd, execRes.Err = prepareLaunchCmd(&newjob, jobmgr, hostBuildEnv, sysCfg)
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
	execRes.Err = err
	// And add the job out/err (for when we actually use a real job manager such as Slurm)
	execRes.Stdout += newjob.GetOutput(&newjob, sysCfg)
	execRes.Stderr += newjob.GetError(&newjob, sysCfg)

	// We can be facing different types of error
	if err != nil {
		// The command simply failed and the Go runtime caught it
		expRes.Pass = false
		log.Printf("[ERROR] Command failed - stdout: %s - stderr: %s - err: %s\n", stdout.String(), stderr.String(), err)
	}
	if submitCmd.Ctx.Err() == context.DeadlineExceeded {
		// The command timed out
		expRes.Pass = false
		log.Printf("[ERROR] Command timed out - stdout: %s - stderr: %s\n", stdout.String(), stderr.String())
	}
	if expRes.Pass {
		if re.Match(stdout.Bytes()) {
			// mpirun actually failed, exited with 0 as return code but displayed the usage message (so nothing really ran)
			expRes.Pass = false
			log.Printf("[ERROR] mpirun failed and returned help messafe - stdout: %s - stderr: %s\n", stdout.String(), stderr.String())
		}
		if !expectedOutput(execRes.Stdout, execRes.Stderr, appInfo, &newjob) {
			// The output is NOT the expected output
			expRes.Pass = false
			log.Printf("[ERROR] Run succeeded but output is not matching expectation - stdout: %s - stderr: %s\n", stdout.String(), stderr.String())
		}
	}

	// For any error, we save details to give a chance to the user to analyze what happened
	if !expRes.Pass {
		if hostMPI != nil && containerMPI != nil {
			err = SaveErrorDetails(&hostMPI.Implem, &containerMPI.Implem, sysCfg, &execRes)
			if err != nil {
				// We only log the error because the most important error is the error
				// that happened while executing the command
				log.Printf("impossible to cleanly handle error: %s", err)
			}
		} else {
			log.Println("Not an MPI job, not saving error details")
		}
	}

	return expRes, execRes
}
