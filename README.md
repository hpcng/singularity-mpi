# Singularity-mpi

This tool aims at running numerous experiments whose results are all aggregated to create a compatibility matrix for a given MPI 
implementation (e.g., OpenMPI or MPICH). 
To do so, this tool runs different configurations of MPI on the host and within a 
container and check whether specific MPI applications/tests/benchmarks succeed.
Practically, the results for each test (e.g., a simple hello world or a point-to-point communication test) are saved in a result file; which once aggregated creates the compatibility matrix.

This is not meant to create an exhaustive compatibility matrix but rather an idea of what to expect since many parameters can impact the overall results (e.g., configuration of the host, configuration of the MPI implementation).

# Preparation of the source code

Before installation, please make sure that your GOPATH environment variable is correctly set and that $GOPATH/bin is in your PATH. This is required because we currently install binaries in $GOPATH/bin.
Then, simply clone the repository on your system: `mkdir -p $GOPATH/src/github.com/sylabs/ && cd $GOPATH/src/github.com/sylabs && git clone https://github.com/sylabs/singularity-mpi.git`, and run `cd $GOPATH/src/github.com/sylabs && make install`.

# Overview

The source code will lead to the creation of 3 different binaries:
- sycontainerize: a tool to help you create containers for your favorite applications.
- sympi: a tool to let you easily install various versions of Singularity, of MPI and manage/run some of your containers (especially when using sycontainerize).
- syvalidate: a tool that aims at helping you create MPI compatibility matrices.

For more details about each of these tools, please refer to the associated documentation, i.e., respectively README.sycontainerizei.md, README.sympi.md and README.syvalidate.md.

All these tools can rely on a workspace to manage MPI installations, containers and various version of Singularity. 

# Installation

To compile the tool, you just need to execute the following command from the top directory of the source code: `cd <path>; make install`.
This will generate three different binaries: `syvalidate`, `sycontainerize` and `sympi`.
The `syvalidate` command can be used to run various experiments. Running the `syvalidate -h` command displays a help 
message that describes different options you could use while running the tool.
The `sycontainerize` command can be used to easily create a container for any application. Running the `sycontainerize -h` command displays a help message that describes how the command can be used.
The `sympi` command can be used to easily manage various MPI installation on the host and easily execute containers using MPI. Running the `sympi -h` command displays a help message that describes how the command can be used.
