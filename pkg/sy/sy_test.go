// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package sy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sylabs/singularity-mpi/pkg/implem"
	"github.com/sylabs/singularity-mpi/pkg/sys"
)

func TestGetImageURL(t *testing.T) {
	var mpiCfg implem.Info
	var sysCfg sys.Config

	sysCfg.EtcDir = filepath.Join(os.Getenv("GOPATH"), "etc")

	mpiCfg.ID = "openmpi"
	mpiCfg.Version = "4.0.0"

	url := GetImageURL(&mpiCfg, &sysCfg)
	if url == "" {
		t.Fatalf("failed to get image URL")
	}
}

func TestGetArchsFromSIFListOutput(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedOutput []string
	}{
		{
			name:           "empty string",
			input:          "",
			expectedOutput: []string{},
		},
		{
			name:           "valid input",
			input:          "3    |1       |NONE    |40960-133189632           |FS (Squashfs/*System/amd64)",
			expectedOutput: []string{"amd64"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := getArchsFromSIFListOutput(tt.input)
			if len(a) != len(tt.expectedOutput) {
				t.Fatalf("result was %s instead of %s", strings.Join(a, " "), strings.Join(tt.expectedOutput, " "))
			}
			for i := 0; i < len(a); i++ {
				if a[i] != tt.expectedOutput[i] {
					t.Fatalf("Element %d is %s instead of %s", i, a[i], tt.expectedOutput[i])
				}
			}
		})
	}
}
