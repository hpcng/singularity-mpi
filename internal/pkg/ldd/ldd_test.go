// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package ldd

import (
	"strings"
	"testing"

	util "github.com/sylabs/singularity-mpi/internal/pkg/util/file"
)

func TestPackageDependenciesForFile(t *testing.T) {
	lddMod, err := Detect()
	if err != nil {
		t.Skip("unable to find suitable ldd module, skipping test")
	}

	testBin := "/bin/true"
	if !util.FileExists(testBin) {
		t.Skipf("%s not available, skipping test", testBin)
	}

	packages := lddMod.GetPackageDependenciesForFile(testBin)
	if len(packages) == 0 {
		t.Fatal("We did not find any dependencies, which is not possible")
	}

	t.Logf("Dependencies: %s", strings.Join(packages, ","))
}
