# Preparation of the host system

Two modes of operations are currently supported:
- building the container images on the host, guaranteeing the latest version of the operating system used by the containers,
- using pre-made images from our registry, in case images cannot be built on the host.

## Using images from registry

Import the key used to signed the images: singularity key pull C7A1FB785121CB91D0965FB1CC509D21C10CC11D

## Locally building images

Two options are available: build image using the `sudo` command, i.e., use the `suid` workflow, or rely on `fakeroot` in which case, there is no need to execute any `sudo` command.

### fakeroot configuration

Singularity supports `fakeroot` for the creation of images without requiring the execution of a `sudo` command.
For more details about fakeroot and Singularity, please refer to the Singularity user documentation.

To enable `fakeroot` in the context of syvalidate, please edit the `singularity-mpi.conf` file of your target workspace (`~/.sympi` by default or whatever directory specified via the `SINGULARITY_INSTALL_DIR` environment variable) and ensure that you have the following entries:
- `force_sudo = false`
- `singularity_sudo_cmds = ` (no value for that specific key)
- `build_privilege = false`
- `force_unprivilege = true`

Note that `syvalidate`'s implementation assumes at the moment that if fakeroot is used, a Singularity installation without `suid` is necessary. As you can see through the options exposed in the configuration file, we can support many more configurations but they are not currently implemented. So please ensure that the version of Singularity you are using has been configured with the `--without-suid` option or use `sympi` to install it.

### sudo configuration

The tool relies heavily on the `singularity` command line for the creation of images. By default, that command is invoked with `sudo`, virtually requiring users to enter their password for every single experiment the tool is executing. When considering that the tool may run dozens of experiments over several hours, having to enter a password for every image creation is quickly cumbersome and potentially a source of unexpected failres. It is therefore strongly encouraged to setup `sudo` so that a password is not requested when executing `singularity` commands. To do so, update your `sudo` configuration as follow:

Execute:
```
sudo visudo
```

Add a line at the end of your `sudo` configuration file (warning, if the line is not added at the end of the file, another rule may overwrite it) as follow, assuming that `singularity` is installed in `/usr/local`:
```
<userid>    ALL=(root)  NOPASSWD: </path/to>/singularity
```

## Previously installed versions of MPI

The Singularity-mpi tool ignores any version of MPI manually installed on the host prior to using this tool. 

# Compilation

Please refer to the README.md file for compilation and installation instructions.

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

``syvalidate -configfile `pwd`/etc/openmpi.conf``

## Run the tool with the default Open MPI versions and Netpipe

``syvalidate -configfile `pwd`/etc/openmpi.conf -netpipe``

## Run the tool with the default Open MPI versions and IMB

``syvalidate -configfile `pwd`/etc/openmpi.conf -imb``

## Run the tool with the default MPICH versions and a simple helloworld test

``syvalidate -configfile `pwd`/etc/mpich.conf``

## Run the tool with the default MPICH versions and Netpipe

``syvalidate -configfile `pwd`/etc/mpich.conf -netpipe``

## Run the tool with the default MPICH versions and IMB

``syvalidate -configfile `pwd`/etc/mpich.conf -imb``

## Run the tool with the default Intel MPI versions and a simple helloworld test

``syvalidate -configfile `pwd`/etc/intel.conf``

## Run the tool with the default Intel MPI versions and Netpipe

``syvalidate -configfile `pwd`/etc/intel.conf -netpipe``

## Run the tool with the default Intel MPI versions and collective operation test

``syvalidate -configfile `pwd`/etc/intel.conf -imb``

These commands will run various MPI programs to test the compatibility between different versions:
- a basic HelloWorld test,
- NetPipe for points-to-point communications,
- IMB for collective communications.

However, more tests will be included over time.

When all the result files are detected, the tool will automatically create a file with the compatibility matrix
for instance, `openmpi_compatibility_matrix.txt` or `mpich_compatibility_matrix.txt`.
