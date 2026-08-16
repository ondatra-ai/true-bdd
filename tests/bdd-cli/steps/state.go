package steps

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/ondatra-ai/true-bdd/tests/libraries/bddgo"
	"github.com/ondatra-ai/true-bdd/tests/libraries/runner"
)

// fixtureNamePattern reads the fixture tree's name out of a scenario's
// Given step. Kept here rather than only inside the step definition
// because the subtest name has to be known before any step runs — a
// scenario is `-run`-selected by the tree it drives, which is the name
// people already type.
var fixtureNamePattern = regexp.MustCompile(`the "([^"]+)" project tree`)

// FixtureName is the Go subtest name for a scenario: the fixture tree it
// drives, falling back to the scenario id for a scenario whose Given
// step names no tree — which is a scenario this suite will refuse a few
// milliseconds later anyway, with a better message than a bad subtest
// name could give.
func FixtureName(scenario bddgo.Scenario) string {
	for _, step := range scenario.Steps {
		match := fixtureNamePattern.FindStringSubmatch(step.Text)
		if match != nil {
			return match[1]
		}
	}

	return scenario.ID
}

// State is one scenario's world: the fixture it prepared, the proxy
// setup it runs behind, and what the CLI left behind.
type State struct {
	T        *testing.T
	Scenario bddgo.Scenario
	Harness  *Harness

	Fixture  *runner.Fixture
	Proxy    ProxySetup
	Result   *runner.RunResult
	Recorder *runner.HarnessRecorder
}

// ProxySetup is one scenario's recording context: where the shim reads
// or writes cassettes, where its cursors live, and — in replay — the
// recorded outcome the run will be graded against.
type ProxySetup struct {
	Env []string
	// Cassettes is the directory the shim uses this run: the fixture's
	// own in replay, the staging directory in record.
	Cassettes string
	StateDir  string
	// Staging is non-empty in record mode only, and is what a passing
	// run publishes.
	Staging string
	// Golden is the recorded outcome, loaded in replay mode only.
	Golden *runner.GoldenTree
}

// NewState returns the per-scenario state constructor bddgo calls before
// the first step.
//
// It registers ONE cleanup, and everything that has to happen after the
// last step happens inside it, in order: grade the run above its steps,
// publish a passing recording, then write the harness record. One
// cleanup rather than three because the order between them is the whole
// point — a record written before the verdict is a record with no
// verdict in it — and t.Cleanup's LIFO is a worse way to say that than a
// sequence of statements.
func NewState(harness *Harness) func(*bddgo.World) (*State, error) {
	return func(world *bddgo.World) (*State, error) {
		state := &State{T: world.T, Scenario: world.Scenario, Harness: harness}

		name := FixtureName(world.Scenario)
		state.Recorder = runner.NewHarnessRecorder(
			harness.SessionRoot, name, harness.Mode, harness.Usage)

		world.T.Cleanup(state.finish)

		return state, nil
	}
}

// finish grades what the steps could not and writes the run's record.
func (s *State) finish() {
	// A scenario that never ran the CLI has nothing to grade — an
	// undefined step, a tree that would not prepare. The record is still
	// written, because the run still happened and the report's
	// denominator counts it.
	if s.Result != nil {
		s.grade()
	}

	s.Recorder.ObserveFixture(s.Fixture)
	s.Recorder.Finish(s.T.Failed(), s.T.Skipped())
}

// grade applies the check the scenario's own steps cannot make: in
// replay, that the recording still reproduces; in live and record, that
// a reader thinks the new output satisfies the rubric.
func (s *State) grade() {
	verdict := s.verdict()
	s.Recorder.ObserveVerdict(verdict)

	for _, failure := range verdict.Failures {
		s.T.Errorf("  - %s", failure)
	}

	// t.Failed() covers the steps as well as the verdict, which is what
	// makes this the promotion gate: a recording is published only for a
	// scenario that passed WHOLE.
	if s.Harness.Mode != runner.ProxyModeRecord {
		return
	}

	if s.T.Failed() {
		dest, _ := s.Harness.CassetteDir(s.Fixture.Name)
		s.T.Logf("cassettes NOT recorded — the run failed; the rejected recording is at %s "+
			"and %s is unchanged", s.Proxy.Staging, dest)

		return
	}

	s.recordOutcome()
}

func (s *State) verdict() runner.Verdict {
	if s.Harness.Mode == runner.ProxyModeReplay {
		return runner.EvaluateRecorded(
			s.Result, s.Proxy.Golden, s.Proxy.Cassettes, s.Proxy.StateDir)
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.Harness.JudgeTimeout)
	defer cancel()

	return runner.Evaluate(ctx, s.Fixture, s.Result, s.Harness.Judge)
}

// recordOutcome writes the run's resulting tree beside its cassettes and
// publishes both, so a later replay has something deterministic to be
// graded against.
func (s *State) recordOutcome() {
	golden := runner.NewGoldenTree(s.Fixture.Name, s.Result.Diff)

	err := runner.WriteGolden(s.Proxy.Staging, golden)
	if err != nil {
		s.T.Errorf("write golden tree: %v", err)

		return
	}

	err = s.Harness.promoteCassettes(s.Fixture.Name, s.Proxy.Staging)
	if err != nil {
		s.T.Errorf("publish cassettes: %v", err)

		return
	}

	s.T.Logf("recorded outcome: %d file(s) outside tmp/", len(golden.Files))
}

// fail prefixes a step failure with the scenario id, so a failure read
// out of a long log says which scenario produced it without scrolling
// up. The caller's own `%w` survives: the id is spliced into the format
// string rather than wrapped around a second Errorf.
//
//nolint:err113 // the message IS the failure; callers pass %w wherever a sentinel exists.
func (s *State) fail(format string, args ...any) error {
	return fmt.Errorf("%s: "+format, append([]any{s.Scenario.ID}, args...)...)
}
