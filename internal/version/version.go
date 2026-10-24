// Package version exposes build metadata injected at link time with -X.
package version

import (
	"fmt"
	"runtime"
)

// DevelopmentVersion is the Version value used for local, untagged builds.
const DevelopmentVersion = "development build"

// Build metadata. Release builds override these with
// -ldflags "-X github.com/tecncr/nwm-hri-modbus-panel/internal/version.Version=1.2.3 ...".
var (
	Version = DevelopmentVersion
	Commit  = "n/a"
	Date    = ""
	Os      = runtime.GOOS
	Arch    = runtime.GOARCH
)

// String renders the version as printed by --version, e.g. "v1.2.3" or
// "development build".
func String() string {
	if Version == DevelopmentVersion || Version == "" {
		return DevelopmentVersion
	}
	return "v" + trimV(Version)
}

// Full returns a single line with every build field.
func Full(name string) string {
	date := Date
	if date == "" {
		date = "n/a"
	}
	return fmt.Sprintf("%s %s (commit %s, built %s, %s/%s)", name, String(), Commit, date, Os, Arch)
}

func trimV(s string) string {
	if len(s) > 0 && (s[0] == 'v' || s[0] == 'V') {
		return s[1:]
	}
	return s
}
