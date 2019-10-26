// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package util

import (
	"testing"
)

func TestDetectURLFormat(t *testing.T) {
	tests := []struct {
		url            string
		expectedResult string
	}{
		{
			url:            "file://aurl",
			expectedResult: FileURL,
		},
		{
			url:            "http://myurl",
			expectedResult: HttpURL,
		},
		{
			url:            "https://aurl",
			expectedResult: HttpURL,
		},
		{
			url:            "http://something/something.git",
			expectedResult: GitURL,
		},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			res := DetectURLType(tt.url)
			if res != tt.expectedResult {
				t.Fatalf("%s returned %s instead of %s", tt.url, res, tt.expectedResult)
			}
		})
	}
}
