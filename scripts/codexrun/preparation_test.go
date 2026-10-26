package codexrun_test

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ondatra-ai/true-bdd/scripts/codexrun"
)

const (
	codexCommand = "codex"
	gitCommand   = "git"
)

func TestPrintingPreparationHasHardDeadline(t *testing.T) {
	for _, command := range []string{codexCommand, "gh", gitCommand} {
		t.Run(command, func(t *testing.T) {
			root, opts := fixture(t)
			runner := codexrun.New(root)
			runner.Stall = 200 * time.Millisecond
			runner.Limit = 3 * time.Second
			runner.Grace = 10 * time.Millisecond

			write(t, filepath.Join(root, "bin", command), "#!/bin/sh\nwhile :; do echo preparing; sleep 0.01; done\n")

			if command == "gh" {
				opts.Issues = []string{"175"}
			}

			if command == gitCommand {
				opts.Name, opts.Kind, opts.Branch, opts.Base = "prepare", codeKind, "impl/prepare", mainBranch
			}

			began := time.Now()

			record, err := runner.Start(t.Context(), opts)
			if elapsed := time.Since(began); elapsed > time.Second {
				t.Fatalf("printing preparation exceeded its hard deadline: %s", elapsed)
			}

			if command == gitCommand {
				if err == nil || !strings.Contains(err.Error(), "timed out") {
					t.Fatalf("Git preparation: %+v %v", record, err)
				}

				return
			}

			status := "failed"
			if command == codexCommand {
				status = unavailableStatus
			}

			assertStatus(t, record, err, status)

			if record.SessionID != "" {
				t.Fatal("agent started after preparation timed out")
			}
		})
	}
}
