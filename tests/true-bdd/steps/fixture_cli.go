package steps

import (
	"os"
	"path/filepath"
	"slices"

	proxycli "github.com/ondatra-ai/true-bdd/pkg/proxy/cli"
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

//truebdd:concept given "fixture AI CLIs"
func useShippedHalveWriter(state *State, _ []string) error {
	if state.Fixture.Name != "concept-bodies-extra" {
		return state.fail("a controlled shipped-Halve writer is not sanctioned for %q", state.Fixture.Name)
	}

	return useFixtureCLI(state, "cli", "codex")
}

//truebdd:concept given "fixture AI CLIs"
func useUnusedPatternProposal(state *State, _ []string) error {
	if state.Fixture.Name != "concept-steps-unused" {
		return state.fail("an unused-pattern proposal is not sanctioned for %q", state.Fixture.Name)
	}

	return useFixtureCLI(state, "cli", "claude")
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

	proxycli.FallbackPath(state, directory)

	return nil
}
