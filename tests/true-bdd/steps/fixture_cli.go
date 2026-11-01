package steps

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/runner"
)

// The recording proxy stays first, so the schema-breaking CLI's replies are recorded normally.
//
//truebdd:concept given "fixture AI CLIs"
func useInvalidLinkCLI(state *State, _ []string) error {
	sanctioned := []string{"concept-link-malformed", "concept-link-missing-answer", "concept-link-incomplete",
		"concept-grouping-missing", "concept-grouping-repeat", "concept-grouping-rank",
		"concept-grouping-cap", "concept-grouping-unknown", "concept-grouping-same"}
	if !slices.Contains(sanctioned, state.Fixture.Name) {
		return state.fail("invalid link replies are not sanctioned for %q", state.Fixture.Name)
	}

	return useFixtureCLI(state, filepath.Join("input", "cli"), "claude")
}

// The writer's verdict that the interface cannot serve is data a real writer never reached (#263).
//
//truebdd:concept given "fixture AI CLIs"
func useCannotServeWriter(state *State, _ []string) error {
	if state.Fixture.Name != "build-steps-cannot-serve" {
		return state.fail("a controlled cannot-serve writer is not sanctioned for %q", state.Fixture.Name)
	}

	return useFixtureCLI(state, "cli", "codex")
}

// useFixtureCLI puts the fixture's directory holding program right behind the suite's
// programs, or before every PATH entry in live mode.
func useFixtureCLI(state *State, dir, program string) error {
	directory, err := filepath.Abs(filepath.Join(state.Fixture.Dir, dir))
	if err != nil {
		return state.fail("resolve fixture CLI: %w", err)
	}

	_, err = os.Stat(filepath.Join(directory, program))
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
