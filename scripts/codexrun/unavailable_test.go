package codexrun_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ondatra-ai/true-bdd/scripts/codexrun"
)

const (
	unavailableStatus = "unavailable"
	signedOutStatus   = "signed-out"
	limitStatus       = "limit"
	spendCapStatus    = "spend-cap"
)

func TestCodexOutEndsWriterUnavailable(t *testing.T) {
	for _, test := range []struct{ name, login, mode, reason string }{
		{name: signedOutStatus, login: "out", reason: signedOutStatus},
		{name: "stalled-login", login: "silent", reason: unavailableStatus},
		{name: "missing-login", login: "missing", reason: unavailableStatus},
		{name: "broken-login", login: "broken", reason: unavailableStatus},
		{name: "expired-token", mode: "expired", reason: signedOutStatus},
		{name: "usage-limit", mode: limitStatus, reason: limitStatus},
		{name: "workspace-spend-cap", mode: spendCapStatus, reason: spendCapStatus},
	} {
		t.Run(test.name, func(t *testing.T) {
			root, opts := fixture(t)
			opts.Name, opts.Kind, opts.Branch, opts.Base = unavailableStatus, codeKind, "impl/unavailable", mainBranch

			t.Setenv("LOGIN", test.login)
			t.Setenv("MODE", test.mode)
			t.Setenv("COMMIT", "yes")

			if test.login == "missing" {
				err := os.Remove(filepath.Join(root, "bin", codexCommand))
				if err != nil {
					t.Fatal(err)
				}

				t.Setenv("PATH", filepath.Join(root, "bin")+":/usr/bin:/bin")
			}

			runner := codexrun.New(root)
			runner.Stall, runner.Grace = 500*time.Millisecond, 10*time.Millisecond
			record, err := runner.Start(t.Context(), opts)
			assertStatus(t, record, err, unavailableStatus)

			assertUnavailableRecord(t, root, record, test.reason)
		})
	}
}

func assertUnavailableRecord(t *testing.T, root string, record codexrun.Record, reason string) {
	t.Helper()

	if record.Agent != "codex" || record.CodexStatus != reason ||
		!strings.Contains(record.Summary(), "reason: Codex") ||
		!strings.Contains(record.Summary(), "status: unavailable\n") ||
		!strings.Contains(record.Summary(), "codex status: "+reason+"\n") ||
		!strings.Contains(record.Summary(), "Claude") {
		t.Fatal(record.Summary())
	}

	var saved codexrun.Record

	data := read(t, filepath.Join(root, "tmp/codex", record.Name, "run.json"))

	err := json.Unmarshal([]byte(data), &saved)
	if err != nil {
		t.Fatal(err)
	}

	if saved.Status != unavailableStatus || saved.CodexStatus != reason {
		t.Fatalf("saved record: %+v", saved)
	}
}

func TestResumeRefusesUnavailableWriterWithoutConsumingAttempt(t *testing.T) {
	const partialEdit = "yes"

	for _, test := range []struct{ mode, partial string }{
		{mode: limitStatus}, {mode: limitStatus, partial: partialEdit},
		{mode: spendCapStatus}, {mode: spendCapStatus, partial: partialEdit},
	} {
		t.Run(test.mode+"/partial="+test.partial, func(t *testing.T) {
			root, opts := fixture(t)
			opts.Name, opts.Kind, opts.Branch, opts.Base = unavailableStatus, codeKind, "impl/unavailable", mainBranch

			t.Setenv("MODE", test.mode)
			t.Setenv("PARTIAL", test.partial)

			runner := codexrun.New(root)
			record, err := runner.Start(t.Context(), opts)
			assertStatus(t, record, err, unavailableStatus)

			if test.partial != "" && read(t, filepath.Join(record.Worktree, "partial.go")) != "partial edit" {
				t.Fatal("missing partial edit")
			}

			path := filepath.Join(root, "tmp/codex", opts.Name, "run.json")
			saved := read(t, path)
			t.Setenv("MODE", "")

			_, err = runner.Resume(t.Context(), opts.Name, "retry after recovery")
			if !errors.Is(err, codexrun.ErrRefused) || !strings.Contains(err.Error(), unavailableStatus) {
				t.Fatalf("resume: %v", err)
			}

			if read(t, path) != saved {
				t.Fatal("refusal changed the recorded attempt")
			}

			_, err = os.Stat(filepath.Join(root, "tmp/codex", opts.Name, "run-2.log"))
			if !os.IsNotExist(err) {
				t.Fatal("refusal created another attempt log")
			}
		})
	}
}
