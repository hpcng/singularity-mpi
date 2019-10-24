// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package slurm

const (
	// SlurmParitionKey is the key to use to retrieve the optinal parition id that
	// can be specified in the tool's configuration file.
	PartitionKey = "slurm_partition"

	// EnabledKey is the key used in the singularity-mpi.conf file to specify if Slurm shall be used
	EnabledKey = "enable_slurm"

	// ScriptCmdPrefix is the prefix to add to a script
	ScriptCmdPrefix = "#SBATCH"
)
