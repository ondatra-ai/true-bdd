package steps

import (
	"path/filepath"
)

//truebdd:concept given "machine environment"
func useOfflineLibraryProxy(state *State, _ []string) error {
	if state.Fixture == nil {
		return state.fail("%w", ErrNoTreePrepared)
	}

	directory := state.WorkingDirectory()
	state.SetEnv("GOPROXY=file://"+filepath.Join(directory, "empty-module-proxy"),
		"GOSUMDB=off", "GONOPROXY=none", "GOPRIVATE=")

	return nil
}
