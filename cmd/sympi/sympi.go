// Copyright (c) 2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sylabs/singularity-mpi/internal/pkg/buildenv"
	"github.com/sylabs/singularity-mpi/internal/pkg/builder"
	"github.com/sylabs/singularity-mpi/internal/pkg/checker"
	"github.com/sylabs/singularity-mpi/internal/pkg/implem"
	"github.com/sylabs/singularity-mpi/internal/pkg/kv"
	"github.com/sylabs/singularity-mpi/internal/pkg/manifest"
	"github.com/sylabs/singularity-mpi/internal/pkg/sy"
	"github.com/sylabs/singularity-mpi/internal/pkg/sympierr"
	"github.com/sylabs/singularity-mpi/internal/pkg/sys"
	util "github.com/sylabs/singularity-mpi/internal/pkg/util/file"
	"github.com/sylabs/singularity-mpi/pkg/sympi"
)

func getContainerInstalls(entries []os.FileInfo) ([]string, error) {
	var containers []string
	for _, entry := range entries {
		matched, err := regexp.MatchString(sys.ContainerInstallDirPrefix+`.*`, entry.Name())
		if err != nil {
			return containers, fmt.Errorf("failed to parse %s: %s", entry, err)
		}
		if matched {
			containers = append(containers, strings.Replace(entry.Name(), sys.ContainerInstallDirPrefix, "", -1))
		}
	}
	return containers, nil
}

func getSingularityInstalls(basedir string, entries []os.FileInfo) ([]string, error) {
	var singularities []string

	for _, entry := range entries {
		matched, err := regexp.MatchString(sys.SingularityInstallDirPrefix+`.*`, entry.Name())
		if err != nil {
			return singularities, fmt.Errorf("failed to parse %s: %s", entry, err)
		}
		if matched {
			// Now we check if we have an install manifest for more information
			installManifest := filepath.Join(basedir, entry.Name(), "mconfig.MANIFEST")
			availVersion := strings.Replace(entry.Name(), sys.SingularityInstallDirPrefix, "", -1)

			if !util.PathExists(installManifest) {
				installManifest = filepath.Join(basedir, entry.Name(), "install.MANIFEST")
			}
			if util.PathExists(installManifest) {
				data, err := ioutil.ReadFile(installManifest)
				// Errors are not fatal, it means we just do not extract more information
				if err == nil {
					if strings.Contains(string(data), "--without-suid") {
						availVersion = availVersion + " [no-suid]"
					}
				}
			}
			singularities = append(singularities, availVersion)
		}
	}
	return singularities, nil
}

func displayInstalled(dir string, filter string) error {

	entries, err := ioutil.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read %s: %s", dir, err)
	}

	curMPIVersion := getLoadedMPI()
	curSingularityVersion := getLoadedSingularity()

	if filter == "all" || filter == "singularity" {
		singularities, err := getSingularityInstalls(dir, entries)
		if err != nil {
			return fmt.Errorf("unable to get the list of singularity installs on the host: %s", err)
		}
		if len(singularities) > 0 {
			fmt.Printf("Available Singularity installation(s) on the host:\n")
			for _, sy := range singularities {
				if curSingularityVersion != "" && strings.Contains(sy, curSingularityVersion) {
					sy = sy + " (L)"
				}
				fmt.Printf("\tsingularity:%s\n", sy)
			}
			fmt.Printf("\n")
		} else {
			fmt.Printf("No Singularity available on the host\n\n")
		}
	}

	if filter == "all" || filter == "mpi" {
		hostInstalls, err := sympi.GetHostMPIInstalls(entries)
		if err != nil {
			return fmt.Errorf("unable to get the install of MPIs installed on the host: %s", err)
		}
		if len(hostInstalls) > 0 {
			fmt.Printf("Available MPI installation(s) on the host:\n")
			for _, mpi := range hostInstalls {
				if mpi == curMPIVersion {
					mpi = mpi + " (L)"
				}
				fmt.Printf("\t%s\n", mpi)
			}
			fmt.Printf("\n")
		} else {
			fmt.Printf("No MPI available on the host\n\n")
		}
	}

	if filter == "all" || strings.Contains(filter, "container") {
		containers, err := getContainerInstalls(entries)
		if err != nil {
			return fmt.Errorf("unable to get the list of containers stored on the host: %s", err)
		}

		if len(containers) > 0 {
			fmt.Printf("Available container(s):\n\t")
			fmt.Println(strings.Join(containers, "\n\t"))
		} else {
			fmt.Printf("No container available\n\n")
		}
	}

	return nil
}

func getSyDetails(desc string) string {
	tokens := strings.Split(desc, ":")
	if len(tokens) != 2 {
		fmt.Println("invalid Singularity description string, execute 'sympi -list' to get the list of available installations")
		return ""
	}
	return tokens[1]
}

func getSyMPIBaseDir() string {
	baseDir := sys.GetSympiDir()
	// We need to make sure that we do not end up with a / we do not want
	if string(baseDir[len(baseDir)-1]) != "/" {
		baseDir = baseDir + "/"
	}
	return baseDir
}

func getLoadedSingularity() string {
	curPath := os.Getenv("PATH")
	pathTokens := strings.Split(curPath, ":")
	for _, t := range pathTokens {
		if strings.Contains(t, sys.SingularityInstallDirPrefix) {
			baseDir := getSyMPIBaseDir()
			t = strings.Replace(t, baseDir, "", -1)
			t = strings.Replace(t, sys.SingularityInstallDirPrefix, "", -1)
			t = strings.Replace(t, "/bin", "", -1)
			return strings.Replace(t, "-", ":", -1)
		}
	}

	return ""
}

func getLoadedMPI() string {
	curPath := os.Getenv("PATH")
	pathTokens := strings.Split(curPath, ":")
	for _, t := range pathTokens {
		if strings.Contains(t, sys.MPIInstallDirPrefix) {
			baseDir := getSyMPIBaseDir()
			t = strings.Replace(t, baseDir, "", -1)
			t = strings.Replace(t, sys.MPIInstallDirPrefix, "", -1)
			t = strings.Replace(t, "/bin", "", -1)
			return strings.Replace(t, "-", ":", -1)
		}
	}

	return ""
}

func loadSingularity(id string) error {
	// We can change the env multiple times during the execution of a single command
	// and these modifications will NOT be reflected in the actual environment until
	// we exit the command and let bash do some magic to update it. Fortunately, we
	// know that we can have one and only one Singularity in the environment of a
	// single time so when we load a specific version of Singularity, we make sure
	// that we remove a previous load changes.
	cleanedPath, cleanedLDLIB := sympi.GetCleanedUpSyEnvVars()

	ver := getSyDetails(id)
	if ver == "" {
		fmt.Println("invalid installation of MPI, execute 'sympi -list' to get the list of available installations")
		return nil
	}

	sympiDir := sys.GetSympiDir()
	syBaseDir := filepath.Join(sympiDir, sys.SingularityInstallDirPrefix+ver)
	syBinDir := filepath.Join(syBaseDir, "bin")
	syLibDir := filepath.Join(syBaseDir, "lib")

	path := strings.Join(cleanedPath, ":")
	ldlib := strings.Join(cleanedLDLIB, ":")
	path = syBinDir + ":" + path
	ldlib = syLibDir + ":" + ldlib

	file, err := sympi.GetEnvFile()
	if err != nil || !util.FileExists(file) {
		return fmt.Errorf("file %s does not exist", file)
	}

	err = sympi.UpdateEnvFile(file, path, ldlib)
	if err != nil {
		return fmt.Errorf("failed to update %s: %s", file, err)
	}

	return nil
}

func updateEnv(newPath []string, newLDLIB []string) error {
	// Sanity checks
	if len(newPath) == 0 {
		return fmt.Errorf("new PATH is empty")
	}

	file, err := sympi.GetEnvFile()
	if err != nil || !util.FileExists(file) {
		return fmt.Errorf("file %s does not exist", file)
	}
	err = sympi.UpdateEnvFile(file, strings.Join(newPath, ":"), strings.Join(newLDLIB, ":"))
	if err != nil {
		return fmt.Errorf("failed to update %s: %s", file, err)
	}

	return nil
}

func unloadSingularity() error {
	newPath, newLDLIB := sympi.GetCleanedUpSyEnvVars()

	return updateEnv(newPath, newLDLIB)
}

func unloadMPI() error {
	newPath, newLDLIB := sympi.GetCleanedUpMPIEnvVars()

	return updateEnv(newPath, newLDLIB)
}

func uninstallMPIfromHost(mpiDesc string, sysCfg *sys.Config) error {
	var mpiCfg implem.Info
	mpiCfg.ID, mpiCfg.Version = sympi.GetMPIDetails(mpiDesc)

	var buildEnv buildenv.Info
	err := buildenv.CreateDefaultHostEnvCfg(&buildEnv, &mpiCfg, sysCfg)
	if err != nil {
		return fmt.Errorf("failed to set host build environment: %s", err)
	}

	b, err := builder.Load(&mpiCfg)
	if err != nil {
		return fmt.Errorf("failed to load a builder: %s", err)
	}

	execRes := b.UninstallHost(&mpiCfg, &buildEnv, sysCfg)
	if execRes.Err != nil {
		return fmt.Errorf("failed to install MPI on the host: %s", execRes.Err)
	}

	return nil
}

func parseSingularityInstallParams(params []string, sysCfg *sys.Config) error {
	for _, p := range params {
		switch p {
		case "no-suid":
			sysCfg.Nopriv = true
			sysCfg.SudoSyCmds = []string{}
		}
	}

	return nil
}

func installSingularity(id string, params []string, sysCfg *sys.Config) error {
	// We create a new sysCfg structure just for this command since we may have passed
	// installation parameters that will change the behavior extracted from the configuration
	// file.
	var mySysCfg sys.Config
	mySysCfg = *sysCfg
	err := parseSingularityInstallParams(params, &mySysCfg)
	if err != nil {
		return fmt.Errorf("failed to parse Singularity installation parameters: %s", err)
	}

	kvs, err := sy.LoadSingularityReleaseConf(&mySysCfg)
	if err != nil {
		return fmt.Errorf("failed to load data about Singularity releases: %s", err)
	}

	var sy implem.Info
	sy.ID = implem.SY
	tokens := strings.Split(id, ":")
	if len(tokens) != 2 {
		return fmt.Errorf("%s had an invalid format, it should of the form 'singularity:<version>'", id)
	}

	sy.Version = tokens[1]
	sy.URL = kv.GetValue(kvs, sy.Version)

	b, err := builder.Load(&sy)
	if err != nil {
		return fmt.Errorf("failed to load a builder: %s", err)
	}
	if !mySysCfg.Nopriv {
		b.PrivInstall = true
	}

	var buildEnv buildenv.Info
	buildEnv.InstallDir = filepath.Join(sys.GetSympiDir(), sys.SingularityInstallDirPrefix+sy.Version)
	buildEnv.ScratchDir = filepath.Join(sys.GetSympiDir(), sys.SingularityScratchDirPrefix+sy.Version)

	// Building any version of Singularity, even if limiting ourselves to Singularity >= 3.0.0, in
	// a generic way is not trivial, the installation procedure changed quite a bit over time. The
	// best option at the moment is to assume that Singularity is simply a standard Go software
	// with all the associated requirements, e.g., to be built from:
	//   GOPATH/src/github.com/sylab/singularity
	buildEnv.BuildDir = filepath.Join(sys.GetSympiDir(), sys.SingularityBuildDirPrefix+sy.Version, "src", "github.com", "sylabs")
	err = util.DirInit(buildEnv.ScratchDir)
	if err != nil {
		return fmt.Errorf("failed to initialize %s: %s", buildEnv.ScratchDir, err)
	}
	defer os.RemoveAll(buildEnv.ScratchDir)
	err = util.DirInit(buildEnv.BuildDir)
	if err != nil {
		return fmt.Errorf("failed to initializat %s: %s", buildEnv.BuildDir, err)
	}
	defer os.RemoveAll(buildEnv.BuildDir)

	execRes := b.InstallOnHost(&sy, &buildEnv, &mySysCfg)
	if execRes.Err != nil {
		return fmt.Errorf("failed to install %s: %s", id, execRes.Err)
	}

	// Create manifest for the Singularity binary
	syBin := filepath.Join(buildEnv.InstallDir, "bin", "singularity")
	manifestPath := filepath.Join(buildEnv.InstallDir, "singularity.MANIFEST")
	hashes := manifest.HashFiles([]string{syBin})
	err = manifest.Create(manifestPath, hashes)
	if err != nil {
		// This is not an error, we just log the error
		log.Printf("failed to create the MANIFEST for %s\n", id)
	}

	return nil
}

func listAvail(sysCfg *sys.Config) error {
	fmt.Println("The following versions of Singularity can be installed:")
	cfgFile := filepath.Join(sysCfg.EtcDir, "singularity.conf")
	kvs, err := kv.LoadKeyValueConfig(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to load configuration from %s: %s", cfgFile, err)
	}
	for _, e := range kvs {
		fmt.Printf("\tsingularity:%s\n", e.Key)
	}

	fmt.Println("The following versions of Open MPI can be installed:")
	cfgFile = filepath.Join(sysCfg.EtcDir, "openmpi.conf")
	kvs, err = kv.LoadKeyValueConfig(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to load configuration from %s: %s", cfgFile, err)
	}
	for _, e := range kvs {
		fmt.Printf("\topenmpi:%s\n", e.Key)
	}

	fmt.Println("The following versions of MPICH can be installed:")
	cfgFile = filepath.Join(sysCfg.EtcDir, "mpich.conf")
	kvs, err = kv.LoadKeyValueConfig(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to load configuration from %s: %s", cfgFile, err)
	}
	for _, e := range kvs {
		fmt.Printf("\tmpich:%s\n", e.Key)
	}

	return nil
}

func importContainerImg(imgPath string, sysCfg *sys.Config) error {
	// Check the architecture of the container, if does not match, error out
	arch, err := sy.GetSIFArchs(imgPath, sysCfg)
	if err != nil {
		return fmt.Errorf("failed to extract architecture from %s: %s", imgPath, err)
	}

	if !sys.CompatibleArch(arch) {
		return fmt.Errorf("%s's architecture is incompatible with host", imgPath)
	}

	// Copy the image in the proper directory under SyMPI
	imgName := filepath.Base(imgPath)
	targetDir := filepath.Join(sys.GetSympiDir(), sys.ContainerInstallDirPrefix+strings.Replace(imgName, ".sif", "", -1))
	err = os.MkdirAll(targetDir, 0755)
	if err != nil {
		return fmt.Errorf("unable to create %s: %s", targetDir, err)
	}
	targetFile := filepath.Join(targetDir, imgName)
	err = util.CopyFile(imgPath, targetFile)
	if err != nil {
		return fmt.Errorf("unable to copy %s to %s: %s", imgPath, targetDir, err)
	}

	return nil
}

func exportContainerImg(containerID string) string {
	// Figure out the path to the image
	imgStoredPath := filepath.Join(getSyMPIBaseDir(), sys.ContainerInstallDirPrefix+containerID, containerID+".sif")
	if !util.FileExists(imgStoredPath) {
		log.Printf("%s does not exist", imgStoredPath)
		return ""
	}

	// Copy the image to /tmp
	targetPath := filepath.Join("/tmp", containerID+".sif")
	err := util.CopyFile(imgStoredPath, targetPath)
	if err != nil {
		log.Printf("failed to copy image from %s to %s: %s", imgStoredPath, targetPath, err)
		return ""
	}

	// Return the path
	return targetPath
}

func main() {
	verbose := flag.Bool("v", false, "Enable verbose mode")
	debug := flag.Bool("d", false, "Enable debug mode")
	list := flag.Bool("list", false, "List all MPIs and Singularity versions on the host, and all MPI containers. 'singularity', 'mpi' and 'container' can be used as filters.")
	load := flag.String("load", "", "The version of MPI/Singularity installed on the host to load")
	unload := flag.String("unload", "", "Unload current version of MPI/Singularity that is used, e.g., sympi -unload [mpi|singularity]")
	install := flag.String("install", "", "MPI/Singularity to install, e.g., openmpi:4.0.2 or singularity:master; for Singularity, the option -no-suid can also be used.")
	nosetuid := flag.Bool("no-suid", false, "When and only when installing Singularity, you may use the -no-suid flag to ensure a full userspace installation")
	uninstall := flag.String("uninstall", "", "MPI implementation to uninstall, e.g., openmpi:4.0.2")
	run := flag.String("run", "", "Run a container")
	avail := flag.Bool("avail", false, "List all available versions of MPI implementations and Singularity that can be installed on the host")
	config := flag.Bool("config", false, "Check and configure the system for SyMPI")
	importCmd := flag.String("import", "", "Import an existing image into SyMPI, e.g., -import <path/to/image>")
	export := flag.String("export", "", "Export a container image")

	flag.Parse()

	// Initialize the log file. Log messages will both appear on stdout and the log file if the verbose option is used
	logFile := util.OpenLogFile("sympi")
	defer logFile.Close()
	if *verbose || *debug || *config {
		nultiWriters := io.MultiWriter(os.Stdout, logFile)
		log.SetOutput(nultiWriters)
	} else {
		log.SetOutput(ioutil.Discard)
	}

	sysCfg := sympi.GetDefaultSysConfig()
	sysCfg.Verbose = *verbose
	sysCfg.Debug = *debug
	// Save the options passed in through the command flags
	if sysCfg.Debug || *config {
		sysCfg.Verbose = true
		err := checker.CheckSystemConfig()
		if err != nil && err != sympierr.ErrSingularityNotInstalled {
			fmt.Printf("\nThe system is not correctly setup.\nOn Debian based systems, the following commands can ensure that all required packages are install:\n" +
				"\tsudo apt -y install build-essential \\ \n" +
				"\t\tlibssl-dev \\ \n" +
				"\t\tuuid-dev \\ \n" +
				"\t\tlibgpgme11-dev \\ \n" +
				"\t\tsquashfs-tools \\ \n" +
				"\t\tlibseccomp-dev \\ \n" +
				"\t\twget \\ \n" +
				"\t\tpkg-config \\ \n" +
				"\t\tgit \\ \n" +
				"\t\tcryptsetup \\ \n" +
				"\t\ttar bzip2 \\ \n" +
				"\t\tgcc gfortran g++ make \\ \n" +
				"\t\tsquashfs-tools \\ \n" +
				"\t\tuidmap\n" +
				"On RPM based systems:\n" +
				"\tyum groupinstall -y 'Development Tools' && \\ \n" +
				"\t\tsudo yum install -y openssl-devel \\ \n" +
				"\t\tlibuuid-devel \\ \n" +
				"\t\tlibseccomp-devel \\ \n" +
				"\t\twget \\ \n" +
				"\t\tsquashfs-tools \\ \n" +
				"\t\tcryptsetup shadow-utils \\ \n" +
				"\t\tgcc gcc-gfortran gcc-c++ make \\ \n")
			fmt.Printf("On RPM systems, you may also want to run the following commands as root to enable fakeroot:\n\tgrubby --args=\"user_namespace.enable=1\" --update-kernel=\"$(grubby --default-kernel)\" \\ \n" +
				"\tsudo echo \"user.max_user_namespaces=15000\" >> /etc/sysctl.conf\n")
			log.Fatalf("System not setup properly: %s", err)
		}
	}

	envFile, err := sympi.GetEnvFile()
	if err != nil || !util.FileExists(envFile) {
		fmt.Println("SyMPI is not initialize, please run the 'sympi_init' command first")
		os.Exit(1)
	}

	sympiDir := sys.GetSympiDir()

	if *config {
		os.Exit(0)
	}

	if *list {
		filter := "all"
		if len(os.Args) >= 3 {
			filter = os.Args[2]
		}
		displayInstalled(sympiDir, filter)
	}

	if *load != "" {
		re := regexp.MustCompile(`^singularity:`)
		if re.Match([]byte(*load)) {
			err := loadSingularity(*load)
			if err != nil {
				log.Fatalf("impossible to load Singularity: %s", err)
			}
		} else {
			err := sympi.LoadMPI(*load)
			if err != nil {
				log.Fatalf("impossible to load MPI: %s", err)
			}
		}
	}

	if *unload != "" {
		switch *unload {
		case "mpi":
			err := unloadMPI()
			if err != nil {
				log.Fatalf("impossible to unload MPI: %s", err)
			}
		case "singularity":
			err := unloadSingularity()
			if err != nil {
				log.Fatalf("impossible to unload Singularity: %s", err)
			}
		default:
			log.Fatalf("unload only access the following arguments: mpi, singularity")
		}
	}

	if *install != "" {
		re := regexp.MustCompile("^singularity")

		if re.Match([]byte(*install)) {
			// It is possible to pass parameters in when installing Singularity
			var singularityParameters []string
			if *nosetuid {
				singularityParameters = append(singularityParameters, "no-suid")
			}
			err := installSingularity(*install, singularityParameters, &sysCfg)
			if err != nil {
				log.Fatalf("failed to install Singularity %s: %s", *install, err)
			}
		} else {
			err := sympi.InstallMPIonHost(*install, &sysCfg)
			if err != nil {
				log.Fatalf("failed to install MPI %s: %s", *install, err)
			}
		}
	}

	if *uninstall != "" {
		err := uninstallMPIfromHost(*uninstall, &sysCfg)
		if err != nil {
			log.Fatalf("impossible to uninstall %s: %s", *uninstall, err)
		}
	}

	if *run != "" {
		err := sympi.RunContainer(*run, nil, &sysCfg)
		if err != nil {
			fmt.Printf("Impossible to run container %s: %s\n", *run, err)
			os.Exit(1)
		}

	}

	if *avail {
		err := listAvail(&sysCfg)
		if err != nil {
			log.Fatalf("impossible to list available software that can be installed")
		}
	}

	if *importCmd != "" {
		err := importContainerImg(*importCmd, &sysCfg)
		if err != nil {
			log.Fatalf("failed to import container: %s", err)
		}
	}

	if *export != "" {
		imgPath := exportContainerImg(*export)
		if imgPath == "" {
			log.Fatalf("failed to export container %s", *export)
		}
		fmt.Printf("Container successfully exported: %s\n", imgPath)
	}
}
