# Installation

Please refer to README.md for installation instructions.

# Configuration file for the creation of a new container for your favorite application

In order to transparently and easy create a container for your favorite application, you simply need
to create a configuration file that gathers high-level data about the target application. The configuration
file is a key/value configuration file where the following keys are required:

- `app_name` which is a string representing the container. When using sycontainerize in persistent mode, the
container image will be installed in the current SyMPI workspace and you will be able to run it with different
versions of MPI, assuming your application is based on MPI.
- `app_url` which is the URL where to fetch the source code of your application. The URL can be a http/https URL, a file (starting with `file://`), or the URL of a Git repository. The tool will figure out how to get the source ready from the URL.
- `app_compile_cmd` which is the command to execute to compile your application, e.g., `make` or `mpicc -o myapp.exe myapp.c`.
- `mpi_model` which is the string representing the MPI model to use. We currently support two models: `hybrid` and `bind`. For details about these two models, please refer to the Singularity User Documentation.
- `mpi` which is the string representing the MPI implementation that you wish to use, i.e., at the moment `openmpi` or `mpich`.
- `container_mpi` which is the version of the MPI implementation to be used with the application. If `mpi` is set to `openmpi` and `container_mpi` to 3.0.4, it means that openmpi-3.0.4 should be installed.
- `distro` is the identifier of the target Linux distribution to be used in the container. Ubuntu Disco, CentOS 6 and CentOS 7 have been tested.
- `registry` is the name of your target Sylabs' registry if you want the image to be automatically uploaded. Note that it requires you to be logged in the service and correctly setup your keyring. Please refer to the Singularity User Documentation for details. This entry is optional.

# Example

Here is the configuration file to create a container for NetPIPE 5.1.4 with Open MPI 4.0.2 and Ubuntu Disco.

```
app_name = ubuntu-disco-openmpi-4.0.2-netpipe-5.1.4-bind
app_url = http://netpipe.cs.ksu.edu/download/NetPIPE-5.1.4.tar.gz
app_exe = NPmpi
app_compile_cmd = make mpi
mpi_model = bind
mpi = openmpi
container_mpi = 4.0.2
distro = ubuntu:disco
```

# Usage

Please run `sycontainerize -h` to display a help message that describes how the command can be used