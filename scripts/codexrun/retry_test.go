package codexrun_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ondatra-ai/true-bdd/scripts/codexrun"
)

func TestFreshRetryKeepsBriefAndHandsBack(t *testing.T) {
	root, opts := fixture(t)
	runner := codexrun.New(root)

	t.Setenv("MODE", "fail")
	// Failed issue preparation leaves no session; the original brief can disappear.
	write(t, filepath.Join(root, "bin/gh"), "#!/bin/sh\nexit 1\n")

	opts.Issues = []string{"175"}
	record, err := runner.Start(t.Context(), opts)
	assertStatus(t, record, err, "failed")

	if record.SessionID != "" {
		t.Fatal("session on preparation failure")
	}

	err = os.Remove(opts.Brief)
	if err != nil {
		t.Fatal(err)
	}

	write(t, filepath.Join(root, "bin/gh"), "#!/bin/sh\necho '{\"number\":175,\"body\":\"body\"}'\n")
	record, err = runner.Resume(t.Context(), opts.Name, "try again with the saved brief")
	assertStatus(t, record, err, "failed")

	prompt := read(t, filepath.Join(root, "capture.stdin"))
	if !strings.Contains(prompt, "Review issue #175") || !strings.Contains(prompt, "try again") {
		t.Fatal(prompt)
	}

	if !strings.Contains(record.Summary(), "hand-back:") {
		t.Fatal(record.Summary())
	}

	_, err = runner.Resume(t.Context(), opts.Name, "third")
	if err == nil {
		t.Fatal("third attempt accepted")
	}
}

func TestCrushRetryStaysOnCrush(t *testing.T) {
	root, opts := fixture(t)
	runner := codexrun.New(root)
	opts.Kind = codeKind
	opts.Name = "crush-retry"
	opts.Base = mainBranch
	opts.Branch = "impl/crush-retry"

	t.Setenv("LOGIN", "out")
	t.Setenv("MODE", "fail")
	record, err := runner.Start(t.Context(), opts)
	assertStatus(t, record, err, "failed")
	t.Setenv("LOGIN", "")
	t.Setenv("MODE", "")
	t.Setenv("COMMIT", "yes")
	record, err = runner.Resume(t.Context(), opts.Name, "finish the commit")
	assertStatus(t, record, err, doneStatus)

	if record.Agent != crushAgent || record.Attempt != 2 {
		t.Fatalf("%+v", record)
	}

	prompt := read(t, filepath.Join(root, "capture.stdin"))
	if !strings.Contains(prompt, "finish the commit") || !strings.Contains(prompt, "git status") {
		t.Fatal(prompt)
	}
}

func TestWriterRetryRejectsMovedBranch(t *testing.T) {
	root, opts := fixture(t)
	runner := codexrun.New(root)
	opts.Kind = codeKind
	opts.Name = "moved"
	opts.Base = mainBranch
	opts.Branch = "impl/moved"

	t.Setenv("MODE", "fail")
	record, err := runner.Start(t.Context(), opts)
	assertStatus(t, record, err, "failed")
	git(t, record.Worktree, "switch", "-c", "different")

	_, err = runner.Resume(t.Context(), opts.Name, "retry")
	if err == nil {
		t.Fatal("accepted moved branch")
	}

	persisted := read(t, filepath.Join(root, "tmp/codex/moved/run.json"))
	if !strings.Contains(persisted, `"attempt": 1`) {
		t.Fatal("refusal consumed attempt")
	}
}

func TestRunningResumeAndWriterClaim(t *testing.T) {
	root, opts := fixture(t)
	runner := codexrun.New(root)
	record, err := runner.Start(t.Context(), opts)
	assertStatus(t, record, err, doneStatus)
	record.Status = runningStatus
	record.PID = os.Getpid()
	storeRecord(t, root, record)

	_, err = runner.Resume(t.Context(), opts.Name, "retry")
	if err == nil {
		t.Fatal("resumed a live command")
	}

	record.PID = -1
	storeRecord(t, root, record)
	record, err = runner.Resume(t.Context(), opts.Name, "recover dead command")
	assertStatus(t, record, err, doneStatus)
}

func storeRecord(t *testing.T, root string, record codexrun.Record) {
	t.Helper()

	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}

	write(t, filepath.Join(root, "tmp/codex", record.Name, "run.json"), string(data))
}

func TestCancellationDuringPreparation(t *testing.T) {
	root, opts := fixture(t)
	runner := codexrun.New(root)
	runner.Grace = 20 * time.Millisecond

	t.Setenv("LOGIN", "silent")

	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	record, err := runner.Start(ctx, opts)
	assertStatus(t, record, err, "failed")

	if record.SessionID != "" {
		t.Fatal("agent started during cancelled preparation")
	}
}
