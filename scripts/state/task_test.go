package state_test

import (
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/scripts/state"
)

const session = "99847444-3f21-4d0e-8a2c-1b7f5e6d0c92"

func TestTaskOpensOnceAndIsShared(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()

	opened, err := state.Task(repo, session, "I want to avoid using sh scripts")
	if err != nil {
		t.Fatalf("Task: %v", err)
	}

	if !strings.HasSuffix(opened, "-99847444-i-want-to-avoid-using-sh-scripts") {
		t.Fatalf("the stem is %q, want <stamp>-<session8>-<slug>", opened)
	}

	// A second process — a nested `claude -p` worker — continues the same Task.
	again, err := state.Task(repo, "5976f8f6-0000-0000-0000-000000000000", "a worker prompt")
	if err != nil {
		t.Fatalf("Task, second caller: %v", err)
	}

	if again != opened {
		t.Fatalf("the second caller opened %q, want the active Task %q", again, opened)
	}
}

func TestTaskCreatesNoFile(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()

	opened, err := state.Task(repo, session, "a prompt")
	if err != nil {
		t.Fatalf("Task: %v", err)
	}

	for _, path := range []string{state.HistoryFile(repo, opened), state.LogFile(repo, opened)} {
		if fileExists(path) {
			t.Errorf("%s exists; a derived path is opened by its first writer, not by Task", path)
		}
	}
}

func TestDerivedPaths(t *testing.T) {
	t.Parallel()

	const stem = "20260827-061838-99847444-i-want-to-avoid"

	if got := state.HistoryFile("/repo", stem); got != "/repo/docs/history/"+stem+".md" {
		t.Errorf("HistoryFile = %q", got)
	}

	if got := state.LogFile("/repo", stem); got != "/repo/docs/history/"+stem+".log.json" {
		t.Errorf("LogFile = %q", got)
	}
}

func TestCursorKeyIsTruncatedAndNamed(t *testing.T) {
	t.Parallel()

	if got := state.CursorKey(session); got != "cursor:99847444" {
		t.Errorf("CursorKey = %q, want the session truncated to 8", got)
	}

	if got := state.CursorKey(""); got != "cursor:unknown" {
		t.Errorf("CursorKey(\"\") = %q, want a named slot", got)
	}
}

func TestSlugFallsBackWhenThePromptHasNoWords(t *testing.T) {
	t.Parallel()

	opened, err := state.Task(t.TempDir(), session, "!!! ???")
	if err != nil {
		t.Fatalf("Task: %v", err)
	}

	if !strings.HasSuffix(opened, "-msg") {
		t.Fatalf("the stem is %q, want the msg fallback", opened)
	}
}
