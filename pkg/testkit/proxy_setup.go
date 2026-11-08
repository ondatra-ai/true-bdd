package testkit

import (
	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
	"github.com/ondatra-ai/true-bdd/pkg/testkit/internal/runner"
	"os"
	"path/filepath"
)

// proxySetup is one scenario's recording context: where the shim reads
// or writes cassettes, where its cursors live, and — in replay — the
// recorded outcome the run will be graded against.
type proxySetup struct {
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
	// JudgeEnv arms THIS process for the verdict call, and is empty when
	// the tests axis runs live. Applied around that call only: the engine
	// subprocess inherits this environment.
	JudgeEnv []string
	// JudgeStaging is non-empty when the tests axis records.
	JudgeStaging string
}

// resolveProxy decides where this scenario's AI calls come from, per
// caller: the services caller's turns and the judge's verdict are independent.
func (s *ScenarioRun) resolveProxy(name string) (proxySetup, error) {
	setup := proxySetup{}

	err := s.resolveTargetProxy(name, &setup)
	if err != nil {
		return proxySetup{}, err
	}

	err = s.resolveJudgeProxy(name, &setup)
	if err != nil {
		return proxySetup{}, err
	}

	return setup, nil
}

// resolveTargetProxy wires the engine's turns: live is unmediated, replay
// REFUSES a scenario with no recording (never skips), and record writes to
// staging, published only once the scenario passes.
func (s *ScenarioRun) resolveTargetProxy(name string, setup *proxySetup) error {
	if s.harness.Modes.Services == runner.ProxyModeLive {
		return nil
	}

	cassettes, err := s.harness.CassetteDir(name)
	if err != nil {
		return s.fail("resolving cassettes dir: %w", err)
	}

	setup.Cassettes = cassettes

	if s.harness.Modes.Services == runner.ProxyModeReplay {
		_, statErr := os.Stat(setup.Cassettes)
		if statErr != nil {
			return s.fail("%w: %s\n%s", ErrNoCassettes, name, RecordHint(s.Scenario))
		}

		golden, goldenErr := runner.ReadGolden(setup.Cassettes)
		if goldenErr != nil {
			return s.fail("%w: %s: %w\n%s",
				ErrNoGolden, name, goldenErr, RecordHint(s.Scenario))
		}

		setup.Golden = golden
	}

	if s.harness.Modes.Services == runner.ProxyModeRecord {
		staging, stagingErr := s.harness.prepareStaging(name)
		if stagingErr != nil {
			return s.fail("preparing staging cassettes: %w", stagingErr)
		}

		setup.Staging = staging
		setup.Cassettes = staging
	}

	// Cursor state lives under the run's engine-scratch tmp/ — inside
	// the snapshot window but excluded from the graded diff, and wiped
	// with the run dir.
	setup.StateDir = filepath.Join(
		runner.RunDir(s.harness.SessionRoot, name), "tmp", "aiproxy-state")
	setup.Env = runner.AIProxyEnv(
		s.harness.Modes.Services, s.harness.Shims.Services,
		setup.Cassettes, setup.StateDir, s.harness.Shims.All())

	return nil
}

// resolveJudgeProxy wires the verdict call, checked at SETUP rather than
// where the judge runs: the judge is a scenario's last act, so a missing
// shelf would otherwise burn a whole replay before saying so.
func (s *ScenarioRun) resolveJudgeProxy(name string, setup *proxySetup) error {
	mode := s.harness.Modes.Tests

	// A scenario with no `judge:` clause never reaches a judge, so it owes
	// no recording — demanding one would refuse most of the suite for a
	// call it was never going to make.
	if mode == runner.ProxyModeLive || !asksJudge(s.Scenario) {
		return nil
	}

	shelf, err := s.harness.JudgeShelf(name)
	if err != nil {
		return s.fail("resolving judge shelf: %w", err)
	}

	if mode == runner.ProxyModeReplay {
		_, statErr := os.Stat(shelf)
		if statErr != nil {
			return s.fail("%w: %s\n%s", ErrNoJudgeRecording, name, JudgeRecordHint(s.Scenario))
		}
	}

	if mode == runner.ProxyModeRecord {
		staging, stagingErr := s.harness.prepareJudgeStaging(name)
		if stagingErr != nil {
			return s.fail("preparing judge staging: %w", stagingErr)
		}

		setup.JudgeStaging = staging
		shelf = staging
	}

	stateDir := filepath.Join(
		runner.RunDir(s.harness.SessionRoot, name), "tmp", "aiproxy-state-judge")
	setup.JudgeEnv = runner.AIProxyEnv(
		mode, s.harness.Shims.Tests, shelf, stateDir, s.harness.Shims.All())

	return nil
}

// asksJudge reports whether this scenario carries a `judge:` clause.
func asksJudge(scenario bddgo.Scenario) bool {
	for _, step := range scenario.Steps {
		if step.Mode == bddgo.ModeRule {
			return true
		}
	}

	return false
}
