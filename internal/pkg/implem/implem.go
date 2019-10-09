// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package implem

const (
	OMPI  = "openmpi"
	MPICH = "mpich"
	IMPI  = "intel"
)

type Info struct {
	ID      string
	Version string
	URL     string
	Tarball string
}
