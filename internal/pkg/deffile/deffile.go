// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package deffile

import (
	"strings"
)

const (
	distroCodenameTag = "DISTROCODENAME"
)

// UpdateDefFileDistroCodename replace the tag for the distro codename in a definition file by the actual target distro codename
func UpdateDistroCodename(data, distro string) string {
	return strings.Replace(data, distroCodenameTag, distro, -1)
}
