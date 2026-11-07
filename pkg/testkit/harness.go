package testkit

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	proxycli "github.com/ondatra-ai/true-bdd/pkg/proxy/cli"
	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
	"github.com/ondatra-ai/true-bdd/pkg/testkit/internal/runner"
)

// StagingDirName names the session-root directory of in-flight fixture recordings, outside the observed run tree.
const StagingDirName = ".cassettes-staging"

// ErrNoCassettes marks a replay requested without a dependency recording.
var ErrNoCassettes = errors.New("no cassettes for this scenario")

// ErrNoGolden marks a replay without a recorded outcome to compare.
var ErrNoGolden = errors.New("no recorded outcome for this scenario")

// ErrNoJudgeRecording marks a replay requested without a recorded semantic verdict.
var ErrNoJudgeRecording = errors.New("no recorded judge verdict for this scenario")

// harness is everything the suite builds once per `go test` invocation
// and every scenario then shares: the binary under test, where the
// session's run directories go, the shim, and the judge.
type harness struct {
	Service      string
	Dependencies []proxycli.Dependency
	RepoLayer    []string
	FixtureStep  string
	// BinPath is the service executable this run resolved.
	BinPath string
	// Binary builds the service only when a definition needs it.
	Binary *runner.LazyBinary
	// SessionRoot is tmp/test_run/<timestamp>/.
	SessionRoot string
	// Shims holds one aiproxy dir per caller. The services dir also supports
	// opted-in fixture observation live; ordinary live turns never arm it.
	Shims runner.ShimDirs
	// Modes is the per-caller mode this run was given.
	Modes runner.Modes
	// Judge grades clauses in every mode; replay serves recorded verdicts.
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
func (h *harness) FixtureDir(name string) string {
	return filepath.Join(h.FixturesDir, name)
}

// CassetteDir is a fixture's committed cassette directory in the source
// tree — what replay reads and what a passing record run publishes into.
func (h *harness) CassetteDir(name string) (string, error) {
	dir, err := filepath.Abs(filepath.Join(h.FixtureDir(name), "cassettes"))
	if err != nil {
		return "", fmt.Errorf("resolve cassettes dir for %s: %w", name, err)
	}

	return dir, nil
}

// JudgeShelf is the fixture's committed judge recordings — its own
// shelf, because both callers spawn `claude` and the shim matches by
// arrival order per binary name: one shared dir would cross their cursors.
func (h *harness) JudgeShelf(name string) (string, error) {
	dir, err := h.CassetteDir(name)
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "judge"), nil
}

// RequireEngine resolves the lazy build at a step, preserving an explicit oracle binary.
func (h *harness) RequireEngine() error {
	if h.BinPath != "" {
		return nil
	}

	path, err := h.Binary.Path()
	if err != nil {
		return fmt.Errorf("resolve engine: %w", err)
	}

	h.BinPath = path

	return nil
}

// prepareJudgeStaging gives a judge recording somewhere to land. It sits
// inside the services caller's staging root, so a run recording both publishes
// them in one rename.
func (h *harness) prepareJudgeStaging(name string) (string, error) {
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
func (h *harness) promoteJudgeShelf(name, staging string) error {
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
func (h *harness) prepareStaging(name string) (string, error) {
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
func (h *harness) promoteCassettes(name, staging string) error {
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

// JudgeRecordHint is the command to record one Scenario's verdict while replaying its service dependencies.
func JudgeRecordHint(scenario bddgo.Scenario) string {
	return "record it with: go test -tags bdd -run '^" + TestName(scenario) +
		"$' ./tests/" + scenario.Service + "/ '-mode=services:replay,tests:record'  # fixture: " +
		FixtureName(scenario)
}

// RecordHint is the command to re-record one Scenario, annotated with its id.
func RecordHint(scenario bddgo.Scenario) string {
	return "record it with: go test -tags bdd -run '^" + TestName(scenario) +
		"$' ./tests/" + scenario.Service + "/ -mode=services:record,tests:record  # fixture: " + FixtureName(scenario)
}
