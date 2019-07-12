package checker

import (
	"fmt"
	"io/ioutil"
	"strings"
)

const (
	distroInfoFile = "/etc/os-release"
)

func checkDistro(distroFile string) (string, error) {
	data, err := ioutil.ReadFile(distroFile)
	if err != nil {
		return "", fmt.Errorf("failed to read %s: %s", distroFile, err)
	}
	content := string(data)

	// Split the content line by line
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "UBUNTU_CODENAME=") {
			codename := line[16:]
			return codename, nil
		}
	}

	return "", nil
}

// CheckDistro tries to detect the codename of the Linux distribution and returns it when possible, an empty string otherwise
func CheckDistro() (string, error) {
	return checkDistro(distroInfoFile)
}
