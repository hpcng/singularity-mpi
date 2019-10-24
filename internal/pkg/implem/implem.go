// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package implem

const (
	// OMPI is the identifier for Open MPI
	OMPI  = "openmpi"
	// MPICH is the identifier for MPICH
	MPICH = "mpich"
	// IMPI is the identifier for Intel MPI
	IMPI  = "intel"
)

// Info gathers all data about a specific MPI implementation
type Info struct {
	ID      string
	Version string
	URL     string
	Tarball string
}
