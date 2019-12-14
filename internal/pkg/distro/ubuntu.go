// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package distro

func ubuntuCodenameToVersion(codename string) string {
	switch codename {
	case "eoan":
		return "19.10"
	case "disco":
		return "19.04"
	case "bionic":
		return "18.04"
	case "xenial":
		return "16.04"
	default:
		return ""
	}
}
