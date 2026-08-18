// Package steps holds the bdd-cli suite's step definitions: the Go code
// that executes the Given/When/Then lines of every registry scenario
// whose `service:` is bdd-cli.
//
// Every definition here was authored by `build tests --fix`: a scenario
// arrives in the registry, the walk reports which of its steps bind to
// nothing, and a model writes the ones that do not. Nothing in this
// package is meant to be written by hand, and a step that reads as
// though it were is a step whose scenario said something the vocabulary
// could not.
package steps

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ondatra-ai/true-bdd/tests/libraries/bddgo"
	"github.com/ondatra-ai/true-bdd/tests/libraries/runner"
)

// StagingDirName holds record mode's in-flight cassettes, one
// subdirectory per fixture, at the session root.
//
// At the session root and not inside the fixture's own run directory:
// cassettes are written DURING the run, and everything written inside
// the run directory lands in the post-run snapshot and therefore in the
// prompt the judge grades. A hundred kilobytes of recorded prompts in
// that diff would drown the handful of files the fixture is actually
// about.
//
// The reporter counts a session subdirectory as a fixture only when it
// holds an engine log or a harness record, so this one is invisible to
// the report while still sitting next to the run it belongs to.
const StagingDirName = ".cassettes-staging"

// stagingPerm is the mode of the per-fixture staging directory: the
// same 0o755 every other directory this harness creates uses.
const stagingPerm = 0o755

// ErrNoCassettes is returned when a replay run finds no recording for a
// scenario. Not a skip: an un-recorded scenario is an un-run scenario,
// and a skip is the one outcome a green suite can hide.
var ErrNoCassettes = errors.New("no cassettes for this scenario")

// ErrNoGolden is returned when a cassette directory carries no recorded
// outcome — a recording made before goldens existed, which replay cannot
// grade against.
var ErrNoGolden = errors.New("no recorded outcome for this scenario")

// Harness is everything the suite builds once per `go test` invocation
// and every scenario then shares: the binary under test, where the
// session's run directories go, the shim, and the judge.
type Harness struct {
	// BinPath is the true-bdd binary this run built.
	BinPath string
	// SessionRoot is tmp/test_run/<timestamp>/.
	SessionRoot string
	// ShimDir holds the aiproxy symlinks, prepended to the CLI
	// subprocess's PATH. Empty in live mode.
	ShimDir string
	// Mode is live, record or replay.
	Mode string
	// Judge grades live and record runs. Never called in replay.
	Judge runner.Judge
	// FixturesDir is where a scenario's Given step looks for its tree.
	FixturesDir string
	// Timeout caps the CLI run when the fixture declares none.
	Timeout time.Duration
	// JudgeTimeout caps the judge's own call, on its own context, so a
	// CLI that hit its deadline still gets a verdict.
	JudgeTimeout time.Duration
	// Usage is the harness process's slog sink, from which the judge's
	// cost is billed to the fixture that caused it.
	Usage *runner.UsageSink
}

// FixtureDir is where the named fixture's tree lives.
func (h *Harness) FixtureDir(name string) string {
	return filepath.Join(h.FixturesDir, name)
}

// CassetteDir is a fixture's committed cassette directory in the source
// tree — what replay reads and what a passing record run publishes into.
func (h *Harness) CassetteDir(name string) (string, error) {
	dir, err := filepath.Abs(filepath.Join(h.FixtureDir(name), "cassettes"))
	if err != nil {
		return "", fmt.Errorf("resolve cassettes dir for %s: %w", name, err)
	}

	return dir, nil
}

// prepareStaging gives a record-mode run an empty directory to write
// into. Empty rather than appended-to, so a shorter re-recording cannot
// leave a longer one's trailing calls behind.
func (h *Harness) prepareStaging(name string) (string, error) {
	staging := filepath.Join(h.SessionRoot, StagingDirName, name)

	err := os.RemoveAll(staging)
	if err != nil {
		return "", fmt.Errorf("clear staging cassettes: %w", err)
	}

	err = os.MkdirAll(staging, stagingPerm)
	if err != nil {
		return "", fmt.Errorf("create staging cassettes: %w", err)
	}

	return staging, nil
}

// promoteCassettes publishes a recording into the source tree. Called
// only for a scenario that PASSED: a cassette set is a recording of
// correct behaviour, and one taken from a failed run would make replay
// reproduce the failure forever — green in record mode today, red in
// replay mode tomorrow, with the evidence already overwritten.
//
// Leaving a failed run's cassettes in staging is the other half of that:
// the previous, good cassettes are still in place, and the rejected ones
// are still on disk to look at.
func (h *Harness) promoteCassettes(name, staging string) error {
	dest, err := h.CassetteDir(name)
	if err != nil {
		return err
	}

	err = os.RemoveAll(dest)
	if err != nil {
		return fmt.Errorf("clear cassettes dir: %w", err)
	}

	// No keep file is needed: golden.json is written for every recording,
	// including a scenario that spawns no AI CLI at all, so the directory
	// is never empty and always survives a clone.
	err = os.Rename(staging, dest)
	if err != nil {
		return fmt.Errorf("publish cassettes to %s: %w", dest, err)
	}

	return nil
}

// RecordHint is the command that re-records one scenario, printed by
// every failure a re-recording would fix.
//
// The fixture name rides along in a trailing comment because the test
// function is named after the scenario id and nobody thinks in ids —
// what a person remembers is the tree, and pasting the line has to work
// without them first having to look the id up.
func RecordHint(scenario bddgo.Scenario) string {
	return "record it with: go test -tags bdd -run '^" + TestName(scenario) +
		"$' ./tests/bdd-cli/ -mode=record  # fixture: " + FixtureName(scenario)
}

// Options builds the bddgo options for this suite: where the registry
// and the spec live, and which suite entry to run.
func Options(repoRoot string) bddgo.Options {
	return bddgo.Options{
		Registry:     filepath.Join(repoRoot, "docs", "scenarios.yaml"),
		Architecture: filepath.Join(repoRoot, "docs", "architecture", "architecture.yaml"),
		Suite:        "bdd-cli",
	}
}
