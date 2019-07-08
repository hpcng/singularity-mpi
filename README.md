# Singularity-mpi

This tool aims at running experiments from which the results can be used to create a compatibility matrix for a given MPI implementation. In other terms, this tool will run different configurations of MPI on the host and within a container and check whether specific MPI applications/tests/benchmarks succeed. This is not meant to create an exhaustive compatibility matrix but rather an idea of what to expect since many parameters can impact the overall results (e.g., configuration of the host, configuration of the MPI implememtation).

# Preparation of the source code

Before installing this tool, please make sure that Go and Singularity are both properly installed.
Then, create the `$HOME/go/src/` directory: `mkdir -p $HOME/go/src/`.
Finally, check-out the source code: `cd $HOME/go/src/ && git clone https://github.com/sylabs/singularity-mpi.git`.

# Preparation of the host system

The tool relies heavily on the singularity command line for the creation of images. That command is invoked with sudo, virtually requiring users to enter their password for every single experiment the tool is executing. When considering that the tool may run dozens of experiments over several hours, having to enter a password for every image creation is quickly cumbersome and potentially a source of unexpected failres. It is therefore strongly encouraged to setup sudo so that a password is not requested when executing singularity commands. To do so, update your sudo configuration as follow:

Execute:
sudo visudo

Add a line at the end of your sudo configuration file (warning, if the line is not added at the end of the file, another rule may overload it) as follow, assuming that singularity is installed in /usr/local:

<userid>    ALL=(root)  NOPASSWD: /usr/local/bin/singularity

# Compilation

To compile the tool, you just need to execute the following command from the top directory of the source code: `cd $HOME/go/src/singularity-mpi && make`.
This will generate a `main` binary that can be used to run various experients. To display the help message, simply run the `./main -h` command.

# Experiments

The tool is based on the concept of *experiment*, an experiment being a set of versions of a given MPI implementation (an MPI implementation being for instance Open MPI or MPICH) and a test to run against a specific version on the host and in the container. As a result, the tool will automatically install a specific version of MPI on the host and automatically create a container image (currently based on Ubuntu) with a specific version of MPI as well.

The version of a given MPI implementation to be used throughout the experiment is defined in a configuration file. For example, a default configuration file for Open MPI is available in `etc/openmpi.conf` and a default configuration file for MPICH is available in `etc/mpich.conf`. Users *must* specify the configuration file on the command line when running the tool (see examples).

# Examples

## Run the tool with the default Open MPI versions and a simple helloworld test

``./main -configfile `pwd`/etc/openmpi.conf``

## Run the tool with the default Open MPI versions and Netpipe

``./main -configfile `pwd`/etc/openmpi.conf -netpipe``

## Run the tool with the default MPICH versions and a simple helloworld test

``./main -configfile `pwd`/etc/mpich.conf``

## Run the tool with the default MPICH versions and Netpipe

``./main -configfile `pwd`/etc/mpich.conf -netpipe``


