package ai_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/adapters/ai"
	pkgerrors "github.com/ondatra-ai/true-bdd/services/true-bdd/internal/pkg/errors"
)

func TestCodexWriterCreatesMissingEditRootsBeforeSpawning(t *testing.T) {
	root := t.TempDir()
	tools := t.TempDir()
	putCodexFile(t, filepath.Join(tools, "auth.json"), "{}")
	putCodexFile(t, filepath.Join(tools, "codex"), missingCodexRootScript)
	t.Setenv("CODEX_HOME", tools)
	t.Setenv("PATH", tools+":"+os.Getenv("PATH"))

	for range 2 {
		_, err := ai.NewCodexProvider().Execute(t.Context(), ai.Request{
			WorkDir: root, TmpDir: filepath.Join(root, "tmp/run"), Label: codexWriterLabel, CodexWriter: true,
			Mode: ai.ExecutionMode{SourceWriteRoots: []string{"services/fresh/**", "services/other"}},
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	if readCodexFile(t, filepath.Join(root, "services/fresh/created.txt")) != "written" {
		t.Fatal("writer did not write inside its edit root")
	}

	assertCodexDirectoryEntries(t, root, []string{"services", "tmp"})
	assertCodexDirectoryEntries(t, filepath.Join(root, "services"), []string{"fresh", "other"})

	if os.Getenv("TRUE_BDD_CODEX_SANDBOX_PROBE") != "" {
		t.Log(readCodexFile(t, filepath.Join(root, "tmp/probe.log")))
	}
}

func TestCodexWriterRefusesMissingEditRootUnderOutsideSymlink(t *testing.T) {
	for _, target := range []string{".", "missing"} {
		t.Run(target, func(t *testing.T) {
			root := t.TempDir()
			outside := t.TempDir()
			tools := t.TempDir()
			putCodexFile(t, filepath.Join(tools, "auth.json"), "{}")
			putCodexFile(t, filepath.Join(tools, "codex"), unexpectedCodexSpawnScript)
			t.Setenv("CODEX_HOME", tools)
			t.Setenv("PATH", tools+":"+os.Getenv("PATH"))

			err := os.Symlink(filepath.Join(outside, target), filepath.Join(root, "services"))
			if err != nil {
				t.Fatal(err)
			}

			_, err = ai.NewCodexProvider().Execute(t.Context(), ai.Request{
				WorkDir: root, TmpDir: filepath.Join(root, "tmp/run"), Label: codexWriterLabel, CodexWriter: true,
				Mode: ai.ExecutionMode{SourceWriteRoots: []string{"services/fresh"}},
			})

			var started *pkgerrors.CodexTurnError
			if err == nil || errors.As(err, &started) {
				t.Fatalf("want a refusal before spawn, got %v", err)
			}

			_, statErr := os.Stat(filepath.Join(root, "spawned"))
			if !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("writer spawned: %v", statErr)
			}

			entries, readErr := os.ReadDir(outside)
			if readErr != nil || len(entries) != 0 {
				t.Fatalf("outside directory changed: %v, %v", entries, readErr)
			}
		})
	}
}

func assertCodexDirectoryEntries(t *testing.T, path string, want []string) {
	t.Helper()

	entries, err := os.ReadDir(path)
	if err != nil {
		t.Fatal(err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}

	if !slices.Equal(names, want) {
		t.Fatalf("entries in %s = %v; want %v", path, names, want)
	}
}

const unexpectedCodexSpawnScript = `#!/bin/sh
touch spawned
while [ "$1" != '-o' ]; do shift; done
printf done > "$2"
`

const missingCodexRootScript = `#!/bin/sh
set -eu
[ -d services/fresh ] || exit 41
[ -d services/other ] || exit 42
if [ -n "${TRUE_BDD_CODEX_SANDBOX_PROBE:-}" ]; then
 "$TRUE_BDD_CODEX_SANDBOX_PROBE" sandbox -C "$PWD" -P writer /bin/sh -c \
  'printf written > services/fresh/created.txt; printf written > services/other/created.txt' > tmp/probe.log 2>&1
 "$TRUE_BDD_CODEX_SANDBOX_PROBE" sandbox -C "$PWD" -P writer /bin/sh -c \
  'mkdir forbidden' >> tmp/probe.log 2>&1 && exit 43
 grep 'Operation not permitted' tmp/probe.log || exit 44
else
 printf written > services/fresh/created.txt
fi
while [ "$1" != '-o' ]; do shift; done
printf done > "$2"
`
