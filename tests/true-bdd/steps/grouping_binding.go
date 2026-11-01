package steps

import "path/filepath"

//truebdd:concept when "a command runs under interference"
func runGroupingChangedBinding(state *State, _ []string) error {
	if state.Fixture == nil {
		return state.fail("%w", ErrNoTreePrepared)
	}

	err := state.Fixture.UseCommand("build steps --fix")
	if err != nil {
		return state.fail("%w", err)
	}

	err = state.Harness.RequireEngine()
	if err != nil {
		return state.fail("%w", err)
	}

	wrapper, err := filepath.Abs(filepath.Join(state.Fixture.Dir, "input", "run-after-check"))
	if err != nil {
		return state.fail("resolve binding-change wrapper: %w", err)
	}

	return executeEngine(state, "build steps --fix", wrapper, "TRUE_BDD_GROUPING_ENGINE="+state.Harness.BinPath)
}
