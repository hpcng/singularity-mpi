package experiments

import "strings"

const (
	distroCodenameTag = "DISTROCODENAME"
)

// UpdateDefFileDistroCodename replace the tag for the distro codename in a definition file by the actual target distro codename
func UpdateDefFileDistroCodename(data, distro string) string {
	return strings.Replace(data, distroCodenameTag, distro, -1)
}
