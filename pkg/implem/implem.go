// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package implem

const (
	// OMPI is the identifier for Open MPI
	OMPI = "openmpi"

	// MPICH is the identifier for MPICH
	MPICH = "mpich"

	// IMPI is the identifier for Intel MPI
	IMPI = "intel"

	// Singularity is the identifier for Singularity
	SY = "singularity"
)

// Info gathers all data about a specific MPI implementation
type Info struct {
	// ID is the string idenfifying the MPI implementation
	ID string

	// Version is the version of the MPI implementation
	Version string

	// URL is the URL to use to get the MPI implementation
	URL string

	// Tarball is the name of the tarball of the MPI implementation
	Tarball string
}

// IsMPI checks if information passed in is an MPI implementation
func IsMPI(i *Info) bool {
	if i != nil && (i.ID == OMPI || i.ID == MPICH || i.ID == IMPI) {
		return true
	}

	return false
}
