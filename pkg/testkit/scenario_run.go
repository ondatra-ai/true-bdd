package testkit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
	"github.com/ondatra-ai/true-bdd/pkg/testkit/runner"
)

// ErrJudgeRefused is returned when the judge ruled against a scenario's
// clauses. A step failure rather than a cleanup-time note, so the
// scenario's own output names it alongside the steps that passed.
var ErrJudgeRefused = errors.New("judge ruled against the scenario's clauses")

// ScenarioRun is the harness's per-Scenario state: the fixture it prepared, the proxy
// setup it runs behind, and what the CLI left behind.
type ScenarioRun struct {
	T        *testing.T
	Scenario bddgo.Scenario
	Harness  *Harness

	Fixture  *runner.Fixture
	Proxy    ProxySetup
	Result   *runner.RunResult
	Recorder *runner.HarnessRecorder

	// judged carries the verdict from Judge into finish: Judge runs as the
	// scenario's last act (a FAIL can still stop the test); finish runs in
	// cleanup, where it cannot, but the record must still be written.
	judged       bool
	judgeVerdict runner.Verdict
}

// NewScenarioRun returns the per-scenario state constructor bddgo calls before
// the first step. It registers ONE cleanup (not three) that grades, then
// publishes, then records in order — t.Cleanup's LIFO would reverse that.
func NewScenarioRun(harness *Harness) func(*bddgo.World) (*ScenarioRun, error) {
	return func(world *bddgo.World) (*ScenarioRun, error) {
		state := &ScenarioRun{T: world.T, Scenario: world.Scenario, Harness: harness}

		name := FixtureName(world.Scenario)
		state.Recorder = runner.NewHarnessRecorder(
			harness.SessionRoot, name, harness.Modes.String(), harness.Usage)

		world.T.Cleanup(state.finish)

		return state, nil
	}
}

// Judge implements bddgo.Judgeable: rules on `judge:` clauses once, after
// every step passed — in EVERY mode. Replay hands it a byte-identical diff
// run after run, which is what makes a verdict disagreement measurable.
func (s *ScenarioRun) Judge(clauses []bddgo.Step) error {
	texts := clauseTexts(clauses)

	// Recorded BEFORE the replay return: the report's expected-vs-actual
	// column needs it, and replay is the mode both gates actually run —
	// recording only on the live path would leave it missing where read most.
	if s.Fixture != nil {
		s.Fixture.JudgeSpec = renderClauses(texts)
	}

	if s.Result == nil {
		return s.fail("%w", ErrNoRun)
	}

	if s.Fixture == nil {
		return s.fail("%w", ErrNoTreePrepared)
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.Harness.JudgeTimeout)
	defer cancel()

	// Armed for the call and no longer: what this process exports is what
	// the next fixture's engine subprocess inherits.
	if len(s.Proxy.JudgeEnv) > 0 {
		restore := runner.ArmProcess(s.Proxy.JudgeEnv)
		defer restore()
	}

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
func (s *ScenarioRun) finish() {
	// A scenario that never ran the CLI has nothing to grade (an undefined
	// step, a tree that failed to prepare) — but the record is still
	// written, since the report's denominator counts the run regardless.
	if s.Result != nil {
		s.grade()
	}

	s.Recorder.ObserveFixture(s.Fixture)
	s.Recorder.Finish(s.T.Failed(), s.T.Skipped())

	// Last act of the scenario, not of the CLI run: a Then step that
	// re-runs the host suite needs the stack the run left standing.
	if s.Result != nil && s.Fixture != nil {
		runner.Teardown(s.Result.TmpDir, s.Fixture.TeardownCmds)
	}
}

// runTimeout is how long this scenario's CLI invocation may take: its own
// `timeout:` if declared, else the suite default. Carried onto the
// fixture too, since the run record reads it there for the report.
func (s *ScenarioRun) runTimeout() time.Duration {
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
func (s *ScenarioRun) grade() {
	verdict := s.finalVerdict()
	s.Recorder.ObserveVerdict(verdict)

	// A judge refusal already stopped the scenario as a step failure, and
	// reporting it here too would show one verdict as two. Golden and
	// census failures reach the test through this line or nowhere.
	if verdict.JudgeOK {
		for _, failure := range verdict.Failures {
			s.T.Errorf("  - %s", failure)
		}
	}

	s.publishRecordings()
}

// publishRecordings promotes whatever this run recorded. t.Failed() covers
// the steps as well as the verdict, which is what makes this the promotion
// gate: a recording is published only for a scenario that passed WHOLE.
func (s *ScenarioRun) publishRecordings() {
	recordingServices := s.Harness.Modes.Services == runner.ProxyModeRecord
	if !recordingServices && s.Proxy.JudgeStaging == "" {
		return
	}

	if s.T.Failed() {
		dest, _ := s.Harness.CassetteDir(s.fixtureName())
		s.T.Logf("recordings NOT published — the run failed; the rejected recording is at %s "+
			"and %s is unchanged", s.stagingRoot(), dest)

		return
	}

	// A recording services caller publishes by renaming its whole staging dir, and
	// the judge's shelf sits inside it — one rename carries both.
	if recordingServices {
		s.recordOutcome()

		return
	}

	err := s.Harness.promoteJudgeShelf(s.fixtureName(), s.Proxy.JudgeStaging)
	if err != nil {
		s.T.Errorf("publish judge shelf: %v", err)

		return
	}

	s.T.Logf("recorded the judge verdict; the engine's cassettes are untouched")
}

// fixtureName is the tree this scenario's Given step names, the same name
// its recorder, run dir and staging use: read from the scenario, so it holds
// whether or not a step definition kept the Fixture it loaded.
func (s *ScenarioRun) fixtureName() string {
	return FixtureName(s.Scenario)
}

// stagingRoot is whichever staging directory this run wrote into.
func (s *ScenarioRun) stagingRoot() string {
	if s.Proxy.Staging != "" {
		return s.Proxy.Staging
	}

	return s.Proxy.JudgeStaging
}

// finalVerdict is what the record states, which is not always what the
// test's exit status states: a scenario can fail on a step and still owe
// the report an accurate account of what its recording did.
func (s *ScenarioRun) finalVerdict() runner.Verdict {
	verdict := runner.Verdict{JudgeOK: true, GoldenOK: true}

	// Golden and census are WHEN-stage fidelity: they prove the replay
	// reproduced the recorded tree and consumed every recorded turn.
	if s.Harness.Modes.Services == runner.ProxyModeReplay {
		verdict = runner.EvaluateRecorded(
			s.Result, s.Proxy.Golden, s.Proxy.Cassettes, s.Proxy.StateDir)
	}

	// The judge grades the outcome's semantics, in every mode. Neither
	// check subsumes the other, so both must hold.
	if s.judged {
		verdict.JudgeOK = s.judgeVerdict.JudgeOK

		// A judge that passed states no reason, and the golden count
		// EvaluateRecorded left here is worth more than that emptiness.
		if s.judgeVerdict.JudgeMsg != "" {
			verdict.JudgeMsg = s.judgeVerdict.JudgeMsg
		}

		verdict.JudgeModel = s.judgeVerdict.JudgeModel
		verdict.JudgeInputHash = s.judgeVerdict.JudgeInputHash
		verdict.JudgeStartedAt = s.judgeVerdict.JudgeStartedAt
		verdict.JudgeEndedAt = s.judgeVerdict.JudgeEndedAt
		verdict.JudgeSystemPrompt = s.judgeVerdict.JudgeSystemPrompt
		verdict.JudgeUserPrompt = s.judgeVerdict.JudgeUserPrompt
		verdict.JudgeResponse = s.judgeVerdict.JudgeResponse
		verdict.Failures = append(verdict.Failures, s.judgeVerdict.Failures...)
	}

	return verdict
}

// recordOutcome writes the run's resulting tree beside its cassettes and
// publishes both, so a later replay has something deterministic to be
// graded against.
func (s *ScenarioRun) recordOutcome() {
	golden := runner.NewGoldenTree(s.fixtureName(), s.Result.Diff)

	err := runner.WriteGolden(s.Proxy.Staging, golden)
	if err != nil {
		s.T.Errorf("write golden tree: %v", err)

		return
	}

	err = s.Harness.promoteCassettes(s.fixtureName(), s.Proxy.Staging)
	if err != nil {
		s.T.Errorf("publish cassettes: %v", err)

		return
	}

	s.T.Logf("recorded outcome: %d file(s) outside tmp/", len(golden.Files))
}

// fail prefixes a step failure with the scenario id via format-string splicing, preserving the caller's `%w`.
//
//nolint:err113 // the message IS the failure; callers pass %w wherever a sentinel exists.
func (s *ScenarioRun) fail(format string, args ...any) error {
	return fmt.Errorf("%s: "+format, append([]any{s.Scenario.ID}, args...)...)
}
