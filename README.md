# Singularity-mpi

This tool aims at running numerous experiments whose results are all aggregated to create a compatibility matrix for a given MPI 
implementation (e.g., OpenMPI or MPICH). 
To do so, this tool runs different configurations of MPI on the host and within a 
container and check whether specific MPI applications/tests/benchmarks succeed.
Practically, the results for each test (e.g., a simple hello world or a point-to-point communication test) are saved in a result file; which once aggregated creates the compatibility matrix.

This is not meant to create an exhaustive compatibility matrix but rather an idea of what to expect since many parameters can impact the overall results (e.g., configuration of the host, configuration of the MPI implementation).

# Preparation of the source code

Before installing this tool, please make sure that Go and Singularity are both properly installed.
Then, create the `$HOME/go/src/github.com/sylabs` directory: `mkdir -p $HOME/go/src/github.com/sylabs`.
Finally, check-out the source code: `cd $HOME/go/src/github.com/sylabs && git clone https://github.com/sylabs/singularity-mpi.git`.

# Preparation of the host system

Two mode of operations are currently supported:
- building the container images on the host, guaranteeing the latest version of the operating system used by the containers,
- using pre-made images from our registry, in case images cannot be built on the host.

## Using images from registry

Import the key used to signed the images: singularity key pull C7A1FB785121CB91D0965FB1CC509D21C10CC11D

## Locally building images

###sudo configuration

The tool relies heavily on the `singularity` command line for the creation of images. That command is invoked with `sudo`, virtually requiring users to enter their password for every single experiment the tool is executing. When considering that the tool may run dozens of experiments over several hours, having to enter a password for every image creation is quickly cumbersome and potentially a source of unexpected failres. It is therefore strongly encouraged to setup `sudo` so that a password is not requested when executing `singularity` commands. To do so, update your `sudo` configuration as follow:

Execute:
```
sudo visudo
```

Add a line at the end of your `sudo` configuration file (warning, if the line is not added at the end of the file, another rule may overwrite it) as follow, assuming that `singularity` is installed in `/usr/local`:
```
<userid>    ALL=(root)  NOPASSWD: /usr/local/bin/singularity
```

## Previously installed versions of MPI

The Singularity-mpi tool ignores any version of MPI manually installed on the host prior to using this tool. 

# Compilation

To compile the tool, you just need to execute the following command from the top directory of the source code: `cd $HOME/go/src/github.com/sylabs/singularity-mpi && make`.
This will generate a `main` binary that can be used to run various experiments. Running `./main -h` command will display a help 
message that describes different options you could use while running the tool. 

# Experiments

The tool is based on the concept of *experiments*, which consist of running on specific test with specific versions of MPI on the host and in the container and result in PASS/FAIL data. The result file (e.g., ``openmpi-results.txt``) is composed of multiple lines, each line describing a specific experiment and its result.

The tool achieves this by installing a specific version of MPI on the host and automatically creating a container image
(currently based on Ubuntu) with a specific version of MPI that will run certain MPI programs within it to test the comptibility. 

The version of a given MPI implementation to be used throughout an experiment is defined in a configuration file. For example, a 
default configuration file for Open MPI is available in `etc/openmpi.conf` and a default configuration file for MPICH is available 
in `etc/mpich.conf`. Users *must* specify the configuration file on the command line when running the tool (see examples). 

Once the tool has completed, view the ``openmpi-results.txt``/``mpich-results.txt`` to view results of various combinations of the 
versions and pick the host-container version combination most suitable to you.

# Tests

At the moment, we support two tests:
- hello world: ensuring that basic short-lived wire-up and termination mechanisms are working correctly.
- NetPipe: ensuring that point-to-point communications run correctly.

# Examples

## Run the tool with the default Open MPI versions and a simple helloworld test

``./main -configfile `pwd`/etc/openmpi.conf``

## Run the tool with the default Open MPI versions and Netpipe

``./main -configfile `pwd`/etc/openmpi.conf -netpipe``

## Run the tool with the default Open MPI versions and IMB

``./main -configfile `pwd`/etc/openmpi.conf -imb``

## Run the tool with the default MPICH versions and a simple helloworld test

``./main -configfile `pwd`/etc/mpich.conf``

## Run the tool with the default MPICH versions and Netpipe

``./main -configfile `pwd`/etc/mpich.conf -netpipe``

## Run the tool with the default MPICH versions and IMB

``./main -configfile `pwd`/etc/mpich.conf -imb``

## Run the tool with the default Intel MPI versions and a simple helloworld test

``./main -configfile `pwd`/etc/intel.conf``

## Run the tool with the default Intel MPI versions and Netpipe

``./main -configfile `pwd`/etc/intel.conf -netpipe``

## Run the tool with the default Intel MPI versions and collective operation test

``./main -configfile `pwd`/etc/intel.conf -imb``

These commands will run various MPI programs to test the compatibility between different versions:
- a basic HelloWorld test,
- NetPipe for points-to-point communications,
- IMB for collective communications.

However, more tests will be included over time.
