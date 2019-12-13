// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package app

// Info gathers information about a given application
type Info struct {
	// Name is the name of the application
	Name string

	// BinName is the name of the binary to start executing the application
	BinName string

	// BinPath is the path to the binary to start executing the application
	BinPath string

	// Source is the URL to get the source. It can be a single file or a URI to a file to download
	Source string

	// InstallCmd is the command to use to install the application
	InstallCmd string

	// ExpectedRankOutput specifies what is the expected output from EACH rank
	// A few keyword can be used for runtime-specific parameters
	// Use '#NP' to specify the job size
	// Use '#RANK' to specify the rank number
	ExpectedRankOutput string
}
