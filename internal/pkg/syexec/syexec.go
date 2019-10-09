// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package syexec

import (
	"context"
	"os/exec"
)

// Result represents the result of the execution of a command
type Result struct {
	// Err is the Go error associated to the command execution
	Err error
	// Stdout is the messages that were displayed on stdout during the execution of the command
	Stdout string
	// Stderr is the messages that were displayed on stderr during the execution of the command
	Stderr string
}

type SyCmd struct {
	// Cmd represents the command to execute to submit the job
	Cmd *exec.Cmd

	BinPath string
	CmdArgs []string
	Env     []string

	// Ctx is the context of the command to execute to submit a job
	Ctx context.Context

	// CancelFn is the function to cancel the command to submit a job
	CancelFn context.CancelFunc
}
