// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package util

import (
	"regexp"
	"strings"
)

// CleanupString cleans up a string. This is convenient when a benchmark
// returns colored text to the terminal that we then try to parse.
func CleanupString(str string) string {
	// Remove all color escape sequences from string
	reg := regexp.MustCompile(`\\x1b\[[0-9]+m`)
	str = reg.ReplaceAllString(str, "")

	str = strings.Replace(str, `\x1b`+"[0m", "", -1)
	return strings.Replace(str, `\x1b`+"[33m", "", -1)
}
