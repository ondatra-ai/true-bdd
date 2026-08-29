package state_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/scripts/state"
)

const task = "20260829-063558-543cd83-in-history"

func TestTaskLogPrefersTheRecordedName(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()
	set(t, repo, state.LogKey, "recorded.json")
	set(t, repo, state.TaskKey, task)

	want := filepath.Join(state.TaskLogDir(repo), "recorded.json")
	if got := state.TaskLog(repo); got != want {
		t.Fatalf("TaskLog = %q, want the recorded name %q", got, want)
	}
}

// The load-bearing case: every later program in the Task must land on this
// file, which is what makes one Task one log.
func TestTaskLogNamesItselfForTheTaskAndRecordsThat(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()
	set(t, repo, state.TaskKey, task)

	want := filepath.Join(state.TaskLogDir(repo), task+".json")
	if got := state.TaskLog(repo); got != want {
		t.Fatalf("TaskLog = %q, want %q", got, want)
	}

	if got := state.Get(repo, state.LogKey); got != task+".json" {
		t.Fatalf("the log key reads %q, want it recorded as %q", got, task+".json")
	}
}

// Not recording is the whole reason the case above is ever reached: the
// history hook installs its logger before state.Task has minted the stem, and
// a persisted random name would be inherited by every later program.
func TestTaskLogFallbackIsNotRecorded(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()

	want := filepath.Join(state.TaskLogDir(repo), state.NoTaskLog)
	if got := state.TaskLog(repo); got != want {
		t.Fatalf("TaskLog = %q, want the no-Task fallback %q", got, want)
	}

	if recorded := state.Get(repo, state.LogKey); recorded != "" {
		t.Fatalf("the fallback recorded %q, want nothing written", recorded)
	}
}

func TestInitRollsTheLogWithoutDeletingIt(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()
	set(t, repo, state.TaskKey, task)

	path := state.TaskLog(repo)

	err := disk.Append(path, []byte(`{"msg":"kept"}`), disk.Shared)
	if err != nil {
		t.Fatalf("Append: %v", err)
	}

	err = state.Init(repo)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	_, err = os.Stat(path)
	if err != nil {
		t.Fatalf("Init destroyed the previous Task's log (%v)", err)
	}

	if got := state.Get(repo, state.LogKey); got != "" {
		t.Fatalf("the log key survived Init as %q, want it dropped", got)
	}
}

func set(t *testing.T, repo, key, value string) {
	t.Helper()

	err := state.Set(repo, key, value)
	if err != nil {
		t.Fatalf("Set %s: %v", key, err)
	}
}
