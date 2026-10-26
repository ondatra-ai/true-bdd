package codexrun_test

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/scripts/codexrun"
)

const selectedEffort = "xhigh"

func TestModelAndEffortSurviveResume(t *testing.T) {
	for _, kind := range []string{readerKind, codeKind} {
		t.Run(kind, func(t *testing.T) {
			for _, session := range []bool{false, true} {
				t.Run(map[bool]string{false: "fresh", true: "session"}[session], func(t *testing.T) {
					root, opts := fixture(t)

					opts.Kind, opts.Model, opts.Effort = kind, "gpt-6-astra", selectedEffort
					if kind == codeKind {
						opts.Branch, opts.Base = "impl/model", mainBranch
					}

					t.Setenv("MODE", "fail")
					record, err := codexrun.New(root).Start(t.Context(), opts)
					assertStatus(t, record, err, "failed")
					assertModelRecord(t, root, record)

					prompt := read(t, filepath.Join(root, "capture.stdin"))
					if kind == codeKind && !strings.Contains(prompt, "at xhigh reasoning effort") {
						t.Fatal("writer brief has wrong effort")
					}

					if !session {
						record.SessionID = ""
						storeRecord(t, root, record)
					}

					t.Setenv("MODE", "")
					t.Setenv("COMMIT", "yes")
					record, err = codexrun.New(root).Resume(t.Context(), opts.Name, "retry")
					assertStatus(t, record, err, doneStatus)
					assertModelRecord(t, root, record)

					args := read(t, filepath.Join(root, "capture.args"))
					if !strings.Contains(args, "-m\ngpt-6-astra\n-c\nmodel_reasoning_effort=\"xhigh\"\n") ||
						strings.Count(args, "model_reasoning_effort=") != 1 || strings.Contains(args, "resume\n") != session {
						t.Fatalf("wrong retry argv: %s", args)
					}
				})
			}
		})
	}
}

func assertModelRecord(t *testing.T, root string, record codexrun.Record) {
	t.Helper()

	var saved codexrun.Record

	err := json.Unmarshal([]byte(read(t, filepath.Join(root, "tmp/codex", record.Name, "run.json"))), &saved)
	if err != nil {
		t.Fatal(err)
	}

	if saved.Model != "gpt-6-astra" || saved.Effort != selectedEffort ||
		record.Model != saved.Model || record.Effort != saved.Effort {
		t.Fatalf("model/effort round trip: saved %+v, returned %+v", saved, record)
	}

	if !strings.Contains(record.Summary(), "model: gpt-6-astra\neffort: xhigh\n") {
		t.Fatal(record.Summary())
	}
}

func TestEffortValidation(t *testing.T) {
	for _, effort := range []string{"bogus", "HIGH", " high", "minimal", "xhigh\""} {
		t.Run(effort, func(t *testing.T) {
			runner := codexrun.New(t.TempDir())

			_, err := runner.Start(t.Context(), codexrun.Options{Name: "bad-effort", Kind: readerKind, Effort: effort})
			if !errors.Is(err, codexrun.ErrRefused) || !strings.Contains(err.Error(), "--effort must be") {
				t.Fatalf("invalid effort accepted or not a usage refusal: %v", err)
			}
		})
	}

	for _, effort := range []string{"", "low", "medium", "high", selectedEffort, "max", "ultra"} {
		t.Run("valid-"+effort, func(t *testing.T) {
			root, opts := fixture(t)
			opts.Effort = effort
			record, err := codexrun.New(root).Start(t.Context(), opts)
			assertStatus(t, record, err, doneStatus)
		})
	}
}

func TestDefaultModelAndEffortSummary(t *testing.T) {
	for _, kind := range []string{readerKind, codeKind} {
		record := codexrun.Record{Options: codexrun.Options{Kind: kind}}

		effort := "configured default"
		if kind == codeKind {
			effort = "high"
		}

		if !strings.Contains(record.Summary(), "model: configured default\neffort: "+effort+"\n") {
			t.Fatal(record.Summary())
		}
	}
}
