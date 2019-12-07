// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package manifest

import (
	"fmt"
	"os"
	"strings"
)

// Create a new manifest
func Create(filepath string, entries []string) error {
	f, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("failed to create %s: %s", filepath, err)
	}

	_, err = f.WriteString(strings.Join(entries, "\n"))
	if err != nil {
		return fmt.Errorf("failed to write to %s: %s", filepath, err)
	}

	return nil
}
