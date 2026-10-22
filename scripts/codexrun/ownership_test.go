package codexrun_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ondatra-ai/true-bdd/scripts/codexrun"
)

func TestFixRejectsBusyWorktree(t *testing.T) {
	root, opts := fixture(t)
	runner := codexrun.New(root)
	opts.Kind = codeKind
	opts.Name = "busy-code"
	opts.Base = mainBranch
	opts.Branch = "impl/busy"

	t.Setenv("COMMIT", "yes")
	record, err := runner.Start(t.Context(), opts)
	assertStatus(t, record, err, doneStatus)
	record.Status = runningStatus
	record.PID = os.Getpid()
	storeRecord(t, root, record)

	opts.Name = "busy-fix"
	opts.Kind = fixKind
	opts.Base = ""

	_, err = runner.Start(t.Context(), opts)
	if err == nil {
		t.Fatal("gave a held worktree to another writer")
	}

	_, err = os.Stat(filepath.Join(root, "tmp/codex", opts.Name))
	if !os.IsNotExist(err) {
		t.Fatal("refusal created an attempt")
	}
}

func TestFourWriterCeiling(t *testing.T) {
	root, opts := fixture(t)

	runner := codexrun.New(root)
	for _, name := range []string{"one", "two", "three", "four"} {
		held := codexrun.Record{Options: codexrun.Options{Name: name, Kind: codeKind},
 Status: runningStatus, PID: os.Getpid(),
			Worktree: filepath.Join(root, "tmp/codex-worktrees", name)}
		storeRecord(t, root, held)
	}

	opts.Kind = codeKind
	opts.Name = "fifth"
	opts.Base = mainBranch
	opts.Branch = "impl/fifth"

	_, err := runner.Start(t.Context(), opts)
	if err == nil {
		t.Fatal("accepted fifth live writer")
	}
}

func TestFixRejectsUnownedCheckout(t *testing.T) {
	root, opts := fixture(t)
	runner := codexrun.New(root)
	git(t, root, "switch", "-c", "unowned")

	opts.Kind = fixKind
	opts.Name = "unowned-fix"
	opts.Branch = "unowned"

	_, err := runner.Start(t.Context(), opts)
	if err == nil {
		t.Fatal("handed the launching checkout to a writer")
	}
}

func TestRetryCanCreateMissingCodeWorkspace(t *testing.T) {
	root, opts := fixture(t)
	runner := codexrun.New(root)
	opts.Kind = codeKind
	opts.Name = "prepare-retry"
	opts.Base = mainBranch
	opts.Branch = "impl/prepare"

	t.Setenv("GIT_FAIL_ADD", "yes")
	record, err := runner.Start(t.Context(), opts)
	assertStatus(t, record, err, "failed")
	t.Setenv("GIT_FAIL_ADD", "")
	t.Setenv("COMMIT", "yes")
	record, err = runner.Resume(t.Context(), opts.Name, "create the worktree this time")
	assertStatus(t, record, err, doneStatus)
}
