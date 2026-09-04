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

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
	"github.com/ondatra-ai/true-bdd/pkg/testkit/runner"
)

// StagingDirName holds record mode's in-flight cassettes, one
// subdirectory per fixture, at the session root — not inside the run
// directory, whose contents land in the judge's graded diff/prompt.
const StagingDirName = ".cassettes-staging"

// stagingPerm is the mode of the per-fixture staging directory: the
// same 0o755 every other directory this harness creates uses.

// ErrNoCassettes is returned when a replay run finds no recording for a
// scenario. Not a skip: an un-recorded scenario is an un-run scenario,
// and a skip is the one outcome a green suite can hide.
var ErrNoCassettes = errors.New("no cassettes for this scenario")

// ErrNoGolden is returned when a cassette directory carries no recorded
// outcome — a recording made before goldens existed, which replay cannot
// grade against.
var ErrNoGolden = errors.New("no recorded outcome for this scenario")

// ErrNoJudgeRecording marks a fixture asked to replay a verdict it has
// never recorded. Infrastructure, not a verdict: routing it through the
// judge's own refusal would suppress the golden mismatch beside it.
var ErrNoJudgeRecording = errors.New("no recorded judge verdict for this scenario")

// Harness is everything the suite builds once per `go test` invocation
// and every scenario then shares: the binary under test, where the
// session's run directories go, the shim, and the judge.
type Harness struct {
	// BinPath is the true-bdd binary this run built.
	BinPath string
	// SessionRoot is tmp/test_run/<timestamp>/.
	SessionRoot string
	// Shims holds one aiproxy dir per caller. A caller running live has
	// no dir at all.
	Shims runner.ShimDirs
	// Modes is the per-caller mode this run was given.
	Modes runner.Modes
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

// JudgeShelf is the fixture's committed judge recordings — its own
// shelf, because both callers spawn `claude` and the shim matches by
// arrival order per binary name: one shared dir would cross their cursors.
func (h *Harness) JudgeShelf(name string) (string, error) {
	dir, err := h.CassetteDir(name)
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "judge"), nil
}

// prepareJudgeStaging gives a judge recording somewhere to land. It sits
// inside the services caller's staging root, so a run recording both publishes
// them in one rename.
func (h *Harness) prepareJudgeStaging(name string) (string, error) {
	staging := filepath.Join(h.SessionRoot, StagingDirName, name, "judge")

	err := disk.RemoveTree(staging)
	if err != nil {
		return "", fmt.Errorf("clear judge staging: %w", err)
	}

	err = disk.Dir(staging, disk.Shared)
	if err != nil {
		return "", fmt.Errorf("create judge staging: %w", err)
	}

	return staging, nil
}

// promoteJudgeShelf publishes only the judge recordings, for a run whose
// services caller was not recording — a whole-dir rename would delete the engine
// cassettes this run replayed from.
func (h *Harness) promoteJudgeShelf(name, staging string) error {
	dest, err := h.JudgeShelf(name)
	if err != nil {
		return err
	}

	err = disk.RemoveTree(dest)
	if err != nil {
		return fmt.Errorf("clear judge shelf: %w", err)
	}

	//nolint:forbidigo // a directory publish, not a file write.
	err = os.Rename(staging, dest)
	if err != nil {
		return fmt.Errorf("publish judge shelf to %s: %w", dest, err)
	}

	return nil
}

// prepareStaging gives a record-mode run an empty directory to write
// into. Empty rather than appended-to, so a shorter re-recording cannot
// leave a longer one's trailing calls behind.
func (h *Harness) prepareStaging(name string) (string, error) {
	staging := filepath.Join(h.SessionRoot, StagingDirName, name)

	err := disk.RemoveTree(staging)
	if err != nil {
		return "", fmt.Errorf("clear staging cassettes: %w", err)
	}

	err = disk.Dir(staging, disk.Shared)
	if err != nil {
		return "", fmt.Errorf("create staging cassettes: %w", err)
	}

	return staging, nil
}

// promoteCassettes publishes a recording into the source tree — only for
// a PASSED scenario: promoting a failed run's cassettes would make replay
// reproduce that failure forever, with the real evidence already overwritten.
func (h *Harness) promoteCassettes(name, staging string) error {
	dest, err := h.CassetteDir(name)
	if err != nil {
		return err
	}

	err = disk.RemoveTree(dest)
	if err != nil {
		return fmt.Errorf("clear cassettes dir: %w", err)
	}

	// No keep file is needed: golden.json is written for every recording,
	// even one that spawns no AI CLI, so the dir survives a clone.
	//nolint:forbidigo // a directory publish, not a file write.
	err = os.Rename(staging, dest)
	if err != nil {
		return fmt.Errorf("publish cassettes to %s: %w", dest, err)
	}

	return nil
}

// JudgeRecordHint is the command that records one scenario's judge
// verdict without re-running a single engine turn: the services caller keeps
// replaying, so the recording costs one model call and nothing else.
func JudgeRecordHint(scenario bddgo.Scenario) string {
	return "record it with: go test -tags bdd -run '^" + TestName(scenario) +
		"$' ./tests/bdd-cli/ '-mode=services:replay,tests:record'  # fixture: " +
		FixtureName(scenario)
}

// RecordHint is the command that re-records one scenario, printed by
// every failure a re-recording would fix. The fixture name rides along
// as a trailing comment, since a reader thinks in trees, not scenario ids.
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
