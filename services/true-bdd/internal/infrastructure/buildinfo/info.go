// Package buildinfo resolves the engine's build identity from stamps and Go metadata.
package buildinfo

import "runtime/debug"

// Info identifies the engine build.
type Info struct {
	Version string
	Commit  string
}

// Resolve resolves each build field independently, without reading process state.
func Resolve(versionStamp, commitStamp string, info *debug.BuildInfo) Info {
	if info != nil {
		if versionStamp == "" {
			versionStamp = info.Main.Version
		}

		for _, setting := range info.Settings {
			if commitStamp == "" && setting.Key == "vcs.revision" {
				commitStamp = setting.Value
			}
		}
	}

	if versionStamp == "" {
		versionStamp = "(devel)"
	}

	if commitStamp == "" {
		commitStamp = "unknown"
	}

	return Info{Version: versionStamp, Commit: commitStamp}
}
