package ai_test

import (
	"bytes"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/adapters/ai"
	pkgerrors "github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/pkg/errors"
)

type codexGlobFixture struct {
	name, action                       string
	patterns                           []string
	existing, wantFailure, wantWarning bool
	writer                             bool
	wantFile                           string
}

func TestCodexApplyEnforcesConfiguredGlobsAfterTurn(t *testing.T) {
	for _, fixture := range []codexGlobFixture{
		{name: "empty matches", action: "mkdir -p tests/a; printf created > tests/a/new.go", wantFile: "tests/a/new.go"},
		{name: "matching change", existing: true, action: "printf changed > tests/a/alpha.go", wantFile: "tests/a/alpha.go"},
		{name: "unmatched creation", existing: true, action: "printf forbidden > tests/a/new.txt"},
		{name: "unreadable unmatched creation", existing: true,
			action: "printf forbidden > tests/a/new.txt; chmod 000 tests/a/new.txt"},
		{name: "unreadable unmatched change", existing: true, action: "chmod 000 tests/a/keep.txt",
			wantFailure: true, wantWarning: true},
		{name: "writer violation stays fatal", existing: true, writer: true,
			action:      "printf forbidden > tests/a/new.txt; printf changed > tests/a/keep.txt; exit 9",
			wantFailure: true, wantWarning: true},
		{name: "unmatched change", existing: true, action: "printf changed > tests/a/keep.txt",
			wantFailure: true, wantWarning: true},
		{name: "unmatched deletion", existing: true, action: "rm tests/a/keep.txt", wantFailure: true, wantWarning: true},
		{name: "unmatched rename", existing: true, action: "mv tests/a/keep.txt tests/a/new.txt",
			wantFailure: true, wantWarning: true},
		{name: "unmatched mode", existing: true, action: "chmod 600 tests/a/keep.txt", wantFailure: true, wantWarning: true},
		{name: "failed turn cleanup", existing: true, action: "printf forbidden > tests/a/new.txt; exit 9",
			wantFailure: true},
		{name: "union of grants", existing: true, action: "printf changed > tests/a/keep.txt",
			patterns: []string{"./tests/**/*.go", "./tests/a/keep.txt"},
			wantFile: "tests/a/keep.txt"},
		{name: "directory glob", action: "mkdir -p tests/new/steps; printf created > tests/new/steps/new.go; " +
			"printf forbidden > tests/new/new.txt",
			patterns: []string{"./tests/*/steps/**"},
			wantFile: "tests/new/steps/new.go"},
		{name: "classes and question marks", action: "mkdir -p tests/ab; printf created > tests/ab/file1.go; " +
			"printf forbidden > tests/ab/new.txt",
			patterns: []string{"./tests/a[bc]/file?.go"},
			wantFile: "tests/ab/file1.go"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			probeCodexGlob(t, fixture)
		})
	}
}

func probeCodexGlob(t *testing.T, fixture codexGlobFixture) {
	t.Helper()
	root, tools := t.TempDir(), t.TempDir()
	putCodexFile(t, filepath.Join(tools, "auth.json"), "{}")
	putCodexFile(t, filepath.Join(tools, "codex"), codexGlobProbeScript)

	if fixture.existing {
		putCodexFile(t, filepath.Join(root, "tests/a/alpha.go"), "original")
		putCodexFile(t, filepath.Join(root, "tests/a/keep.txt"), "original")
	}

	t.Setenv("CODEX_HOME", tools)
	t.Setenv("PATH", tools+":"+os.Getenv("PATH"))
	t.Setenv("TRUE_BDD_GLOB_ACTION", fixture.action)

	patterns := fixture.patterns
	if patterns == nil {
		patterns = []string{"./tests/**/*.go"}
	}

	var logs bytes.Buffer

	previous := slog.Default()

	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	mode := ai.ExecutionMode{}
	for _, pattern := range patterns {
		mode.AllowedTools = append(mode.AllowedTools, "Write("+pattern+")")
	}

	_, err := ai.NewCodexProvider().Execute(t.Context(), ai.Request{
		WorkDir: root, TmpDir: filepath.Join(root, "tmp/run"), Label: "glob", CodexApply: true, CodexWriter: fixture.writer,
		Mode: mode,
	})
	if (err != nil) != fixture.wantFailure {
		t.Fatalf("turn error = %v; logs=%s", err, &logs)
	}

	var recoverable *pkgerrors.CodexTurnError
	if fixture.wantWarning && errors.As(err, &recoverable) {
		t.Fatalf("glob violation became recoverable: %v", err)
	}

	assertCodexGlobResult(t, fixture, root, logs.String())
}

func assertCodexGlobResult(t *testing.T, fixture codexGlobFixture, root, logs string) {
	t.Helper()

	if fixture.wantFile != "" {
		content := readCodexFile(t, filepath.Join(root, fixture.wantFile))
		if content != "created" && content != "changed" {
			t.Fatalf("matching write lost: %q", content)
		}
	}

	_, statErr := os.Lstat(filepath.Join(root, "tests/a/new.txt"))
	if !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("unmatched new file survived: %v", statErr)
	}

	_, statErr = os.Lstat(filepath.Join(root, "tests/new/new.txt"))
	if !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("unmatched new file survived: %v", statErr)
	}

	wantWarnings := 0
	if fixture.wantWarning {
		wantWarnings = 1
	}

	if count := strings.Count(logs, "level=WARN"); count != wantWarnings {
		t.Fatalf("warnings=%d; logs=%s", count, logs)
	}

	if fixture.wantWarning && !strings.Contains(logs, "tests/a/keep.txt") {
		t.Fatalf("warning omitted path: %s", logs)
	}

	assertCodexCredentialsRemoved(t, filepath.Join(root, "tmp/codex-homes/run/glob-codex-home"))
}

const codexGlobProbeScript = `#!/bin/sh
set -eu
if [ -n "${TRUE_BDD_CODEX_SANDBOX_PROBE:-}" ]; then
 "$TRUE_BDD_CODEX_SANDBOX_PROBE" sandbox -C "$PWD" -P writer /bin/sh -ec "$TRUE_BDD_GLOB_ACTION"
else
 /bin/sh -ec "$TRUE_BDD_GLOB_ACTION"
fi
while [ "$1" != '-o' ]; do shift; done
printf done > "$2"
`
