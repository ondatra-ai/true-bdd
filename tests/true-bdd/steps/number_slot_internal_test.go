package steps

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/runner"
)

// A number slot's `no` is zero (the catalogue's template grammar): a run
// that exited 0 passes `the command exits with code no`.
func TestTheExitCodeStepReadsNoAsZero(t *testing.T) {
	t.Parallel()

	state := &State{T: t, Result: &runner.RunResult{ExitCode: 0}}

	err := assertExitCode(state, []string{"no"})
	if err != nil {
		t.Fatalf("exit code no on a run that exited 0: %v", err)
	}
}

// An empty file has no lines, which `has exactly no lines` says.
func TestTheLineCountStepReadsNoAsZero(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	err := os.WriteFile(filepath.Join(dir, "empty.txt"), nil, 0o600)
	if err != nil {
		t.Fatalf("write the empty file: %v", err)
	}

	state := &State{T: t, Result: &runner.RunResult{TmpDir: dir}}

	err = assertFileLineCount(state, []string{"empty.txt", "no"})
	if err != nil {
		t.Fatalf("an empty file has exactly no lines: %v", err)
	}
}

// A run that created nothing created exactly no files matching any glob.
func TestTheCreatedFilesCountStepReadsNoAsZero(t *testing.T) {
	t.Parallel()

	state := &State{T: t, Result: &runner.RunResult{}}

	err := assertFilesMatchingCreated(state, []string{"no", "tests/*.go"})
	if err != nil {
		t.Fatalf("exactly no files created on a run that created none: %v", err)
	}
}

// A Story whose criteria list is empty has no acceptance criteria.
func TestTheStoryCriteriaCountStepReadsNoAsZero(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	err := os.WriteFile(filepath.Join(dir, "story.yaml"),
		[]byte("story:\n  id: s\n  acceptance_criteria: []\n"), 0o600)
	if err != nil {
		t.Fatalf("write the story: %v", err)
	}

	state := &State{T: t, Result: &runner.RunResult{TmpDir: dir}}

	err = assertStoryCriteriaExactCount(state, []string{"story.yaml", "no"})
	if err != nil {
		t.Fatalf("a Story with an empty criteria list has no acceptance criteria: %v", err)
	}
}

// A Registry whose only Scenario no Story claims has no Scenario from any
// acceptance criterion.
func TestTheRegistryCountStepReadsNoAsZero(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	err := os.WriteFile(filepath.Join(dir, "scenarios.yaml"),
		[]byte("scenarios:\n  E2E-001:\n    description: d\n    user_stories: []\n"), 0o600)
	if err != nil {
		t.Fatalf("write the registry: %v", err)
	}

	state := &State{T: t, Result: &runner.RunResult{TmpDir: dir}}

	err = assertRegistryScenarioCount(state, []string{"no", "scenarios.yaml", "s-001-x"})
	if err != nil {
		t.Fatalf("exactly no scenarios come from an unclaimed criterion: %v", err)
	}
}

// An engine log with no "Spawning test runner" record shows exactly no
// test runners spawned.
func TestTheTestRunnerCountStepReadsNoAsZero(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	err := os.MkdirAll(filepath.Join(dir, "tmp"), 0o755)
	if err != nil {
		t.Fatalf("create the engine's tmp dir: %v", err)
	}

	err = os.WriteFile(filepath.Join(dir, engineLogRelPath),
		[]byte(`{"level":"INFO","msg":"Walk starting"}`+"\n"), 0o600)
	if err != nil {
		t.Fatalf("write the engine log: %v", err)
	}

	state := &State{T: t, Result: &runner.RunResult{TmpDir: dir}}

	err = assertTestRunnerCount(state, []string{"no"})
	if err != nil {
		t.Fatalf("exactly no test runners on a log that spawned none: %v", err)
	}
}

// `true` exits 0, which `exits with code no` says.
func TestTheShellCommandStepReadsNoAsZero(t *testing.T) {
	t.Parallel()

	state := &State{T: t, Result: &runner.RunResult{TmpDir: t.TempDir()}}

	err := assertCommandExitCode(state, []string{"true", ".", "no"})
	if err != nil {
		t.Fatalf("a shell command that exits 0 exits with code no: %v", err)
	}
}
