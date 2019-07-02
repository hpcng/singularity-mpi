# Singularity-mpi

This tool aims at running numerous experiments whose results are all aggregated to create a compatibility matrix for a given MPI implementation(OpenMPI or 
MPICH). In other terms, this tool will run different configurations of MPI on the host and within a container and check whether specific MPI applications/tests/benchmarks
succeed. This is not meant to create an exhaustive compatibility matrix but rather an idea of what to expect since many parameters can impact the overall 
results (e.g., configuration of the host, configuration of the MPI implementation).

# Preparation of the source code

Before installing this tool, please make sure that Go and Singularity are both properly installed.
Then, create the `$HOME/go/src/` directory: `mkdir -p $HOME/go/src/`.
Finally, check-out the source code: `cd $HOME/go/src/ && git clone https://github.com/sylabs/singularity-mpi.git`.

# Compilation

To compile the tool, you just need to execute the following command from the top directory of the source code: `cd $HOME/go/src/singularity-mpi && make`.
This will generate a `main` binary that can be used to run various experiments. Running `./main -h` command will display a help message that describes 
different options you could use while running the tool. 

# Experiments

The tool is based on the concept of an *experiment* - the experiment being generation of PASS/FAIL compatibility between different versions of MPI on the 
host and in the container. The tool achieves this by installing a specific version of MPI on the host and automatically creating a container image (currently
based on Ubuntu) with a specific version of MPI that will run certain MPI programs within it, the successful completion of which will result in a PASS 
compatibility. 

The version of a given MPI implementation to be used throughout the experiment is defined in a configuration file. For example, a default
configuration file for Open MPI is available in `etc/openmpi.conf` and a default configuration file for MPICH is available in `etc/mpich.conf`. Users *must* 
specify the configuration file on the command line when running the tool (see examples). 

Once you've tested the tool, view the ``opnempi-results.txt``/``mpich-results.txt`` to view results of various combinations of the versions and pick the 
host-container version combination most suitable to you.

---
**NOTE**

   The Singularity-mpi tool ignores any version of MPI manually installed on the host prior to using this tool. 

---

# Examples

## Run the tool with the default Open MPI versions and a simple helloworld test

``./main -configfile `pwd`/etc/openmpi.conf``

## Run the tool with the default Open MPI versions and Netpipe

``./main -configfile `pwd`/etc/openmpi.conf -netpipe``

## Run the tool with the default MPICH versions and a simple helloworld test

``./main -configfile `pwd`/etc/mpich.conf``

## Run the tool with the default MPICH versions and Netpipe

``./main -configfile `pwd`/etc/mpich.conf -netpipe``

These commands will run basic MPI programs (HelloWorld, points-to-point tests) to test the compatibility between different versions. However, more tests will 
be included over time.

