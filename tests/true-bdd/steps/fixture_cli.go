package steps

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/runner"
)

// A host tree may supply its external Claude boundary under input/cli.
// The recording proxy stays first, so these deterministic replies are recorded normally.
func useTreeCLI(state *State) error {
	directory, err := filepath.Abs(filepath.Join(state.Fixture.Dir, "input", "cli"))
	if err != nil {
		return state.fail("resolve fixture CLI: %w", err)
	}

	_, err = os.Stat(filepath.Join(directory, "claude"))
	if os.IsNotExist(err) {
		return nil
	}

	if err != nil {
		return state.fail("stat fixture CLI: %w", err)
	}

	if state.Harness.Modes.Services == runner.ProxyModeLive {
		state.Proxy.Env = []string{"PATH=" + directory + string(os.PathListSeparator) + os.Getenv("PATH")}

		return nil
	}

	for index, entry := range state.Proxy.Env {
		if !strings.HasPrefix(entry, "PATH=") {
			continue
		}

		shim, rest, _ := strings.Cut(strings.TrimPrefix(entry, "PATH="), string(os.PathListSeparator))
		state.Proxy.Env[index] = "PATH=" + strings.Join([]string{shim, directory, rest}, string(os.PathListSeparator))
	}

	return nil
}
