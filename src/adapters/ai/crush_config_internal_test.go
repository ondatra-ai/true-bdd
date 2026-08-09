package ai

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCrushGuardCommandQuotesTheExecutable checks the hook command
// survives a path with shell metacharacters.
//
// crush parses this string as a shell command AND fails OPEN when a
// hook cannot run — so an unquoted `$` or `'` in the binary's path does
// not merely break the hook, it silently removes the only write gate.
func TestCrushGuardCommandQuotesTheExecutable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		exe  string
	}{
		{"plain path", "/usr/local/bin/true-bdd"},
		{"space", "/Users/a b/bin/true-bdd"},
		{"dollar", "/opt/$USER/true-bdd"},
		{"single quote", "/opt/pete's tools/true-bdd"},
		{"semicolon", "/opt/x;rm -rf ~/true-bdd"},
		{"backtick", "/opt/`whoami`/true-bdd"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			command := crushGuardCommand(testCase.exe)

			if !strings.HasSuffix(command, " "+crushGuardSubcommand) {
				t.Fatalf("command %q lost its subcommand", command)
			}

			quoted := strings.TrimSuffix(command, " "+crushGuardSubcommand)

			if !strings.HasPrefix(quoted, "'") || !strings.HasSuffix(quoted, "'") {
				t.Errorf("executable %q was emitted unquoted as %q", testCase.exe, quoted)
			}

			// Round-trip: undoing POSIX single-quote escaping must give
			// back exactly the original path.
			inner := strings.TrimSuffix(strings.TrimPrefix(quoted, "'"), "'")
			if got := strings.ReplaceAll(inner, `'\''`, "'"); got != testCase.exe {
				t.Errorf("round trip = %q, want %q", got, testCase.exe)
			}
		})
	}
}

// TestWriteCrushConfigIsolatesEachTurn checks two turns sharing a TmpDir
// get their own config. A fixed filename would let a later turn's
// policy — including its write roots — replace one already in use.
func TestWriteCrushConfigIsolatesEachTurn(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	req := Request{TmpDir: tmpDir, WorkDir: t.TempDir(), Mode: ExecutionMode{}}

	first, err := writeCrushConfig(req, "/usr/local/bin/true-bdd")
	if err != nil {
		t.Fatalf("first config: %v", err)
	}

	second, err := writeCrushConfig(req, "/usr/local/bin/true-bdd")
	if err != nil {
		t.Fatalf("second config: %v", err)
	}

	if first == second {
		t.Fatalf("both turns wrote %s — a concurrent turn would overwrite "+
			"the live config", first)
	}

	for _, dir := range []string{first, second} {
		content, readErr := os.ReadFile(filepath.Join(dir, "crush.json"))
		if readErr != nil {
			t.Fatalf("read %s: %v", dir, readErr)
		}

		var decoded crushConfig

		jsonErr := json.Unmarshal(content, &decoded)
		if jsonErr != nil {
			t.Fatalf("config in %s is not valid JSON: %v", dir, jsonErr)
		}

		if len(decoded.Hooks.PreToolUse) == 0 {
			t.Errorf("config in %s has no PreToolUse hook — the write gate is gone", dir)
		}
	}
}
