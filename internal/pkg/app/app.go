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

	// ExpectedNote specifies what is the expected note from an application
	//
	// A note is the result of an application-specific parsing/analysis of the
	// application's output that is specific to this framework. For instance,
	// for netpipe, the expected note is something like 'max bandwidth: 44.773 Gbps; latency: 50.609 nsecs'
	// todo: should support regexp here
	ExpectedNote string
}
