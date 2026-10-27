package steps

import (
	"path/filepath"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/runner"
)

func useOfflineLibraryProxy(state *State, _ []string) error {
	if state.Fixture == nil {
		return state.fail("%w", ErrNoTreePrepared)
	}

	directory := runner.RunDir(state.Harness.SessionRoot, state.Fixture.Name)
	state.Proxy.Env = append(state.Proxy.Env, "GOPROXY=file://"+filepath.Join(directory, "empty-module-proxy"),
		"GOSUMDB=off", "GONOPROXY=none", "GOPRIVATE=")

	return nil
}
