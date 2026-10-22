package codexrun_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ondatra-ai/true-bdd/scripts/codexrun"
)

const readerKind = "review"

type claimResult struct {
	record codexrun.Record
	err    error
}

func delayGit(t *testing.T, root, command string) {
	t.Helper()

	path := filepath.Join(root, "bin/git")
	script := read(t, path)
	pause := `#!/bin/sh
if [ "$1" = "` + command + `" ] && [ ! -f "$CAPTURE.validating" ]; then
 "$CODEX_TEST_GIT" "$@" > "$CAPTURE.validation-output"
 validation_status=$?
 cd / || exit 1
 : > "$CAPTURE.validating"
 while [ ! -f "$CAPTURE.release" ]; do sleep 0.01; done
 cat "$CAPTURE.validation-output"
 exit "$validation_status"
fi
`
	write(t, path, strings.Replace(script, "#!/bin/sh\n", pause, 1))
}

func waitForValidation(t *testing.T, root string) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		_, err := os.Stat(filepath.Join(root, "capture.validating"))
		if err == nil {
			return
		}

		time.Sleep(10 * time.Millisecond)
	}

	t.Fatal("Git validation did not start")
}

func delayedClaim(t *testing.T, root string, run func(context.Context) (codexrun.Record, error)) <-chan claimResult {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	result := make(chan claimResult, 1)

	done := make(chan struct{})
	go func() {
		defer close(done)

		record, err := run(ctx)
		result <- claimResult{record: record, err: err}
	}()

	t.Cleanup(func() {
		cancel()
		write(t, filepath.Join(root, "capture.release"), "")

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("delayed claim did not stop")
		}
	})
	waitForValidation(t, root)

	return result
}

func awaitClaim(t *testing.T, result <-chan claimResult) claimResult {
	t.Helper()

	select {
	case got := <-result:
		return got
	case <-time.After(2 * time.Second):
		t.Fatal("claim blocked behind Git validation")

		return claimResult{}
	}
}

func TestDelayedValidationAllowsUnrelatedClaims(t *testing.T) {
	for _, resume := range []bool{false, true} {
		t.Run(map[bool]string{false: "start", true: "resume"}[resume], func(t *testing.T) {
			root, opts := fixture(t)
			runner := codexrun.New(root)
			opts.Name, opts.Kind, opts.Branch, opts.Base = "slow", codeKind, "impl/slow", mainBranch
			command := "check-ref-format"

			if resume {
				t.Setenv("MODE", "fail")
				record, err := runner.Start(t.Context(), opts)
				assertStatus(t, record, err, "failed")
				t.Setenv("MODE", "")

				command = "worktree"
			}

			t.Setenv("COMMIT", "yes")
			delayGit(t, root, command)
			slow := delayedClaim(t, root, func(ctx context.Context) (codexrun.Record, error) {
				if resume {
					return runner.Resume(ctx, opts.Name, "retry")
				}

				return runner.Start(ctx, opts)
			})
			reader := make(chan claimResult, 1)

			go func() {
				record, err := runner.Start(t.Context(), codexrun.Options{Name: "reader", Kind: readerKind, Brief: opts.Brief})
				reader <- claimResult{record: record, err: err}
			}()

			got := awaitClaim(t, reader)
			assertStatus(t, got.record, got.err, doneStatus)
			write(t, filepath.Join(root, "capture.release"), "")
			got = awaitClaim(t, slow)
			assertStatus(t, got.record, got.err, doneStatus)
		})
	}
}

func TestClaimRechecksNameAfterValidation(t *testing.T) {
	root, opts := fixture(t)
	runner := codexrun.New(root)
	opts.Name, opts.Kind, opts.Branch, opts.Base = "contested", codeKind, "impl/contested", mainBranch

	delayGit(t, root, "check-ref-format")
	slow := delayedClaim(t, root, func(ctx context.Context) (codexrun.Record, error) { return runner.Start(ctx, opts) })
	reader := make(chan claimResult, 1)

	go func() {
		record, err := runner.Start(t.Context(), codexrun.Options{Name: opts.Name, Kind: readerKind, Brief: opts.Brief})
		reader <- claimResult{record: record, err: err}
	}()

	got := awaitClaim(t, reader)
	assertStatus(t, got.record, got.err, doneStatus)
	write(t, filepath.Join(root, "capture.release"), "")

	if awaitClaim(t, slow).err == nil {
		t.Fatal("published a stale claim over the run that reserved its name")
	}

	if !strings.Contains(read(t, filepath.Join(root, "tmp/codex", opts.Name, "run.json")), `"kind": "review"`) {
		t.Fatal("overwrote the winning run")
	}
}

func TestClaimRechecksWriterOwnershipAfterValidation(t *testing.T) {
	root, opts := fixture(t)
	runner := codexrun.New(root)
	opts.Name, opts.Kind, opts.Branch, opts.Base = "slow", codeKind, "impl/shared", mainBranch

	t.Setenv("COMMIT", "yes")
	delayGit(t, root, "worktree")
	slow := delayedClaim(t, root, func(ctx context.Context) (codexrun.Record, error) { return runner.Start(ctx, opts) })
	winner := opts
	winner.Name = "winner"
	record, err := runner.Start(t.Context(), winner)
	assertStatus(t, record, err, doneStatus)
	write(t, filepath.Join(root, "capture.release"), "")

	if awaitClaim(t, slow).err == nil {
		t.Fatal("published after another writer claimed the branch during validation")
	}

	_, err = os.Stat(filepath.Join(root, "tmp/codex", opts.Name))
	if !os.IsNotExist(err) {
		t.Fatal("stale writer claim consumed an attempt")
	}
}

func TestResumeRechecksAttemptAfterValidation(t *testing.T) {
	root, opts := fixture(t)
	runner := codexrun.New(root)
	opts.Name, opts.Kind, opts.Branch, opts.Base = "retry", codeKind, "impl/retry", mainBranch

	t.Setenv("MODE", "fail")
	record, err := runner.Start(t.Context(), opts)
	assertStatus(t, record, err, "failed")
	t.Setenv("MODE", "")
	t.Setenv("COMMIT", "yes")
	delayGit(t, root, "worktree")
	slow := delayedClaim(t, root, func(ctx context.Context) (codexrun.Record, error) {
		return runner.Resume(ctx, opts.Name, "delayed retry")
	})
	record, err = runner.Resume(t.Context(), opts.Name, "winning retry")
	assertStatus(t, record, err, doneStatus)
	write(t, filepath.Join(root, "capture.release"), "")

	if awaitClaim(t, slow).err == nil {
		t.Fatal("published a stale retry after another command used the attempt")
	}

	if got := read(t, filepath.Join(root, "tmp/codex", opts.Name, "resume.md")); !strings.Contains(got, "winning retry") {
		t.Fatal("overwrote the winning retry")
	}
}
