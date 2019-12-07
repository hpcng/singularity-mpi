// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package syexec

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/sylabs/singularity-mpi/internal/pkg/manifest"
	"github.com/sylabs/singularity-mpi/internal/pkg/sys"
	util "github.com/sylabs/singularity-mpi/internal/pkg/util/file"
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

// SyCmd represents a command to be executed
type SyCmd struct {
	// Cmd represents the command to execute to submit the job
	Cmd *exec.Cmd

	// Timeout is the maximum time a command can run
	Timeout time.Duration

	// BinPath is the path to the binary to execute
	BinPath string

	// CmdArgs is a slice of string representing the command's arguments
	CmdArgs []string

	// ExecDir is the directory where to execute the command
	ExecDir string

	// Env is a slice of string representing the environment to be used with the command
	Env []string

	// Ctx is the context of the command to execute to submit a job
	Ctx context.Context

	// CancelFn is the function to cancel the command to submit a job
	CancelFn context.CancelFunc

	// ManifestDir is the directory where to create the manifest related to the command execution
	ManifestDir string

	// ManifestData is extra content to add to the manifest
	ManifestData []string

	// ManifestFileHash is a list of absolute path to files for which we want a hash in the manifest
	ManifestFileHash []string
}

func getFileHash(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	hasher := sha256.New()
	_, err = io.Copy(hasher, f)
	if err != nil {
		return ""
	}

	return hex.EncodeToString(hasher.Sum(nil))
}

func hashFiles(files []string) []string {
	var hashData []string

	for _, file := range files {
		hash := getFileHash(file)
		hashData = append(hashData, file+": "+hash)
	}

	return hashData
}

// Run executes a syexec command and creates the appropriate manifest (when possible)
func (c *SyCmd) Run() Result {
	var res Result

	cmdTimeout := c.Timeout
	if cmdTimeout == 0 {
		cmdTimeout = sys.CmdTimeout
	}

	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout*time.Minute)
	defer cancel()

	var stderr, stdout bytes.Buffer
	if c.Cmd == nil {
		c.Cmd = exec.CommandContext(ctx, c.BinPath, c.CmdArgs...)
		c.Cmd.Dir = c.ExecDir
		c.Cmd.Stdout = &stdout
		c.Cmd.Stderr = &stderr
	}

	log.Printf("-> Running %s %s\n", c.BinPath, strings.Join(c.CmdArgs, " "))
	err := c.Cmd.Run()
	res.Stderr = stderr.String()
	res.Stdout = stdout.String()
	if err != nil {
		res.Err = err
		return res
	}

	if c.ManifestDir != "" {
		path := filepath.Join(c.ManifestDir, "exec.MANIFEST")
		if !util.FileExists(path) {
			currentTime := time.Now()
			data := []string{"Command: " + c.BinPath + strings.Join(c.CmdArgs, " ") + "\n"}
			data = append(data, "Execution path: "+c.ExecDir)
			data = append(data, "Execution time: "+currentTime.Format("2006-01-02 15:04:05"))
			data = append(data, c.ManifestData...)

			filesToHash := []string{c.BinPath} // we always get the fingerprint of the binary we execute
			filesToHash = append(filesToHash, c.ManifestFileHash...)
			hashData := hashFiles(filesToHash)
			data = append(data, hashData...)

			err := manifest.Create(path, data)
			if err != nil {
				// This is not a fatal error, we just log it
				log.Printf("failed to create manifest: %s", err)
			}
			log.Printf("-> Manifest successfully created (%s)", path)

		} else {
			log.Printf("Manifest %s already exists, skipping...", err)
		}
	}

	return res
}
