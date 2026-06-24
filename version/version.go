package version

import "runtime/debug"

var Version = "dev"

func BuildInfo() (string, bool) {
	if info, ok := debug.ReadBuildInfo(); ok {
		return info.String(), true
	}
	return "", false
}
