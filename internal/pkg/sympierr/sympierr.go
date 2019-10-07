// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package sympierr

import "errors"

// ErrNotAvailable is the error returned when an element that is being looked up is not available
var ErrNotAvailable = errors.New("item not available")

// ErrFileExists is the error returned when trying to access a file that does not exist
var ErrFileExists = errors.New("file already exists")
