package buildinfo

import "runtime/debug"

//nolint:gochecknoglobals // Link-time stamps are set using -ldflags -X.
var (
	version string
	commit  string
)

// Current returns the stamped build identity, falling back to Go build metadata.
func Current() Info {
	info, _ := debug.ReadBuildInfo()

	return Resolve(version, commit, info)
}
