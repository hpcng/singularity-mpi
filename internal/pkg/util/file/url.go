// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package util

// Constants defining the URL types
const (
	// UnsupportedURLTyps is a constant for a type of URL that we do not support yet
	UnsupportedURLType = ""

	// FileURL is a constant for a file-based URL
	FileURL = "file"

	// HttpURL is a constant for a HTTP-based URL
	HttpURL = "http"
)

// DetectURLType detects the type of the URL that is passed in.
func DetectURLType(url string) string {
	if url[:7] == "file://" {
		return FileURL
	}

	if url[:4] == "http" {
		return HttpURL
	}

	// Unsupported type
	return UnsupportedURLType
}
