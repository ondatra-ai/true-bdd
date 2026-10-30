package steps

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/runner"
)

// Invalid-answer Scenarios explicitly select a schema-breaking external CLI.
// The recording proxy stays first so its replies are recorded normally.
func useInvalidLinkCLI(state *State, _ []string) error {
	switch state.Fixture.Name {
	case "concept-link-malformed", "concept-link-missing-answer", "concept-link-incomplete",
		"concept-grouping-missing", "concept-grouping-repeat", "concept-grouping-rank",
		"concept-grouping-cap", "concept-grouping-unknown", "concept-grouping-same":
	default:
		return state.fail("invalid link replies are not sanctioned for %q", state.Fixture.Name)
	}

	directory, err := filepath.Abs(filepath.Join(state.Fixture.Dir, "input", "cli"))
	if err != nil {
		return state.fail("resolve fixture CLI: %w", err)
	}

	_, err = os.Stat(filepath.Join(directory, "claude"))
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
