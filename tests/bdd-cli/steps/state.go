package steps

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/ondatra-ai/true-bdd/tests/libraries/bddgo"
	"github.com/ondatra-ai/true-bdd/tests/libraries/runner"
)

// ErrJudgeRefused is returned when the judge ruled against a scenario's
// clauses. A step failure rather than a cleanup-time note, so the
// scenario's own output names it alongside the steps that passed.
var ErrJudgeRefused = errors.New("judge ruled against the scenario's clauses")

// fixtureNamePattern reads the fixture tree's name out of a scenario's
// Given step. Kept here rather than only inside the step definition
// because the subtest name has to be known before any step runs — a
// scenario is `-run`-selected by the tree it drives, which is the name
// people already type.
var fixtureNamePattern = regexp.MustCompile(`the "([^"]+)" project tree`)

// FixtureName is the run-directory name for a scenario: the fixture tree
// it drives, falling back to the scenario id for a scenario whose Given
// step names no tree — which is a scenario this suite will refuse a few
// milliseconds later anyway, with a better message than a bad directory
// name could give.
//
// Still the tree rather than the scenario id, now that a scenario's Go
// test is named after the id instead. Everything that reads a run —
// the report server, the coverage tool, a person looking in
// tmp/test_run — finds a directory named after the thing it holds, and
// renaming that would be a change to all of them for no gain.
func FixtureName(scenario bddgo.Scenario) string {
	for _, step := range scenario.Steps {
		match := fixtureNamePattern.FindStringSubmatch(step.Text)
		if match != nil {
			return match[1]
		}
	}

	return scenario.ID
}

// TestName is the generated Go test function for a scenario, mirroring
// the engine's own derivation so a record hint names a filter that
// actually selects something.
func TestName(scenario bddgo.Scenario) string {
	var name strings.Builder

	name.WriteString("Test")

	for _, char := range scenario.ID {
		if unicode.IsLetter(char) || unicode.IsDigit(char) {
			name.WriteRune(char)
		}
	}

	return name.String()
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

	// judged carries the verdict from Judge into finish. The two are
	// separate moments on purpose: Judge runs as the scenario's last act,
	// where a FAIL can still stop the test; finish runs in cleanup, where
	// it cannot but where the record has to be written regardless.
	judged       bool
	judgeVerdict runner.Verdict
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

// Judge implements bddgo.Judgeable: it rules on the scenario's `judge:`
// clauses, once, after every other step passed.
//
// In replay it asks nothing of anyone. Every byte those clauses would
// read was materialised from a cassette, and the recording that produced
// them only exists because it satisfied these same clauses when it was
// made — so re-grading it would re-grade a fixed artefact, and would do
// it with a model's variance. The golden comparison in finish is what
// discharges them instead, and it is stricter: byte-for-byte rather than
// a reading.
func (s *State) Judge(clauses []bddgo.Step) error {
	texts := clauseTexts(clauses)

	// Recorded BEFORE the replay return, for the same reason the exit code
	// and the stdout patterns are: the run's record — and so the report's
	// expected-vs-actual column — must state what this run was held to.
	// Replay is the mode both gates run, so a record written only on the
	// live path would leave the clauses unrecoverable exactly where they
	// are read most.
	if s.Fixture != nil {
		s.Fixture.JudgeSpec = renderClauses(texts)
	}

	if s.Harness.Mode == runner.ProxyModeReplay {
		return nil
	}

	if s.Result == nil {
		return s.fail("%w", ErrNoRun)
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.Harness.JudgeTimeout)
	defer cancel()

	s.judgeVerdict = runner.Evaluate(ctx, runner.JudgeRequest{
		Cmd:     s.Fixture.Cmd,
		Clauses: texts,
		Diff:    s.Result.Diff,
	}, s.Harness.Judge)
	s.judged = true

	if !s.judgeVerdict.JudgeOK {
		return s.fail("%w: %s", ErrJudgeRefused, s.judgeVerdict.JudgeMsg)
	}

	return nil
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

// runTimeout is how long this scenario's CLI invocation may take: its own
// `timeout:` if it declared one, otherwise the suite default.
//
// The resolved value is carried onto the fixture rather than only returned,
// because the run record reads it there — a report that showed every
// budget as zero could not tell a 5-minute default run from the 15-minute
// one, which is the whole reason a scenario is allowed to say.
func (s *State) runTimeout() time.Duration {
	timeout := s.Harness.Timeout
	if s.Scenario.Timeout > 0 {
		timeout = s.Scenario.Timeout
	}

	if s.Fixture != nil {
		s.Fixture.Timeout = timeout
	}

	return timeout
}

// clauseTexts strips the clauses down to what the model is asked to rule
// on. The Given/When/Then keyword is punctuation for a reader; a rule
// numbered "1. But no criterion's messages changed" reads as a fragment.
func clauseTexts(clauses []bddgo.Step) []string {
	texts := make([]string, 0, len(clauses))
	for _, clause := range clauses {
		texts = append(texts, clause.Text)
	}

	return texts
}

// renderClauses is the clause list as one block of text, for the run
// record. Numbered the way the judge saw them, so a report and a judge
// transcript read the same.
func renderClauses(texts []string) string {
	if len(texts) == 0 {
		return ""
	}

	var buf strings.Builder

	for index, text := range texts {
		fmt.Fprintf(&buf, "%d. %s\n", index+1, text)
	}

	return buf.String()
}

// grade applies the check the scenario's own steps cannot make: in
// replay, that the recording still reproduces; in live and record, that
// a reader thought the new output satisfied the scenario's clauses.
func (s *State) grade() {
	verdict := s.finalVerdict()
	s.Recorder.ObserveVerdict(verdict)

	// A judge refusal already stopped the scenario as a step failure.
	// Reporting it again here would show one verdict as two.
	if !s.judged {
		for _, failure := range verdict.Failures {
			s.T.Errorf("  - %s", failure)
		}
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

// finalVerdict is what the record states, which is not always what the
// test's exit status states: a scenario can fail on a step and still owe
// the report an accurate account of what its recording did.
func (s *State) finalVerdict() runner.Verdict {
	if s.Harness.Mode == runner.ProxyModeReplay {
		return runner.EvaluateRecorded(
			s.Result, s.Proxy.Golden, s.Proxy.Cassettes, s.Proxy.StateDir)
	}

	if s.judged {
		return s.judgeVerdict
	}

	// A scenario with nothing for a model to rule on, outside replay:
	// its steps were the whole of its grading, and they have already run.
	return runner.Verdict{JudgeOK: true, GoldenOK: true}
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
