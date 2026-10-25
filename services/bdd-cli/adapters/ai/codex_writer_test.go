package ai_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/adapters/ai"
	pkgerrors "github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/pkg/errors"
)

const codexWriterLabel = "writer"

func TestCodexWriterGetsAnIsolatedHomeAndRestrictedProfile(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "original-home")
	putCodexFile(t, filepath.Join(home, "auth.json"), `{"fixture":"credentials"}`)
	putCodexFile(t, filepath.Join(home, "config.toml"), "user config must not be copied")
	t.Setenv("CODEX_HOME", home)

	bin := filepath.Join(root, "bin")
	putCodexFile(t, filepath.Join(bin, "codex"), `#!/bin/sh
printf '%s\n' "$@" > args.txt
printf '%s\n' "$CODEX_HOME" > home.txt
[ "$(cat "$CODEX_HOME/auth.json")" = '{"fixture":"credentials"}' ] || exit 10
cp -p "$CODEX_HOME/auth.json" dummy-auth-copy.json
while [ "$1" != '-o' ]; do shift; done
printf 'done' > "$2"
printf '{"type":"turn.completed"}\n'
`)
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))

	_, err := ai.NewCodexProvider().Execute(t.Context(), ai.Request{
		WorkDir: root, TmpDir: filepath.Join(root, "tmp/run"), Label: codexWriterLabel, CodexWriter: true,
		Mode: ai.ExecutionMode{SourceWriteRoots: []string{"services/demo"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	generated := strings.TrimSpace(readCodexFile(t, filepath.Join(root, "home.txt")))

	assertPrivateCodexHome(t, generated)

	info, statErr := os.Stat(filepath.Join(root, "dummy-auth-copy.json"))
	if statErr != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("credential mode: %v, %v", info, statErr)
	}

	config := readCodexFile(t, filepath.Join(generated, "config.toml"))
	for _, want := range []string{`default_permissions = "writer"`, `model_reasoning_effort = "high"`,
		`model_context_window = 1050000`, `web_search = "disabled"`, `project_doc_max_bytes = 0`,
		`enabled = false`, `GOPROXY = "off"`, `GOFLAGS = "-trimpath -buildvcs=false"`,
		`"/private/tmp" = "deny"`, `"/private/var/tmp" = "deny"`,
		`/github.com/ondatra-ai" = "deny"`, `/cache/download/github.com/ondatra-ai" = "deny"`,
		filepath.Join(root, "services/demo") + `" = "write"`, filepath.Join(root, "tmp") + `" = "write"`,
		filepath.Join(root, "tmp/codex-homes") + `" = "deny"`} {
		if !strings.Contains(config, want) {
			t.Errorf("profile missing %q: %s", want, config)
		}
	}

	assertCodexCredentialsRemoved(t, generated)

	args := readCodexFile(t, filepath.Join(root, "args.txt"))
	if strings.Contains(args, "\n-s\n") || !strings.Contains(args, "--json\n") {
		t.Fatalf("writer args = %s", args)
	}

	if !strings.Contains(readCodexFile(t, filepath.Join(root, "tmp/run/writer-codex-events.jsonl")), "turn.completed") {
		t.Fatal("events not saved")
	}
}

func TestOtherCodexTurnsKeepTheInheritedHomeAndSandbox(t *testing.T) {
	for _, roots := range [][]string{nil, {"services/demo"}} {
		t.Run(strings.Join(roots, "/"), func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("CODEX_HOME", filepath.Join(root, "user-home"))
			bin := filepath.Join(root, "bin")
			putCodexFile(t, filepath.Join(bin, "codex"), `#!/bin/sh
printf '%s\n' "$CODEX_HOME" > home.txt
printf '%s\n' "$@" > args.txt
while [ "$1" != '-o' ]; do shift; done
printf 'answer' > "$2"
`)
			t.Setenv("PATH", bin+":"+os.Getenv("PATH"))

			_, err := ai.NewCodexProvider().Execute(t.Context(), ai.Request{
				WorkDir: root, TmpDir: root, Mode: ai.ExecutionMode{SourceWriteRoots: roots},
			})
			if err != nil {
				t.Fatal(err)
			}

			if strings.TrimSpace(readCodexFile(t, filepath.Join(root, "home.txt"))) != os.Getenv("CODEX_HOME") {
				t.Fatal("ordinary home changed")
			}

			args := readCodexFile(t, filepath.Join(root, "args.txt"))

			want := "read-only"
			if len(roots) > 0 {
				want = "workspace-write"
			}

			if !strings.Contains(args, "-s\n"+want+"\n") || strings.Contains(args, "--json") {
				t.Fatalf("ordinary args=%s", args)
			}
		})
	}
}

func TestCodexStartedWriterOutcomesAndStartFailures(t *testing.T) {
	for _, testCase := range []struct{ name, script, outcome string }{
		{"start failure", "#!/missing/interpreter\n", ""},
		{"usage limit", "#!/bin/sh\necho \"You've hit your usage limit\" >&2\nexit 7\n", "non-zero exit"},
		{"missing answer", "#!/bin/sh\n: no_answer\nexit 0\n", "empty output"},
		{"empty answer", "#!/bin/sh\nwhile [ \"$1\" != '-o' ]; do shift; done\nprintf ' \\n' > \"$2\"\n", "empty output"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			root := t.TempDir()
			home := filepath.Join(root, "auth")
			putCodexFile(t, filepath.Join(home, "auth.json"), "{}")
			t.Setenv("CODEX_HOME", home)

			bin := filepath.Join(root, "bin")
			putCodexFile(t, filepath.Join(bin, "codex"), testCase.script)
			t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
			_, err := ai.NewCodexProvider().Execute(t.Context(), ai.Request{
				WorkDir: root, TmpDir: filepath.Join(root, "tmp/run"), Label: codexWriterLabel, CodexWriter: true,
			})

			var failure *pkgerrors.CodexTurnError
			if got := errors.As(err, &failure); got != (testCase.outcome != "") || err == nil {
				t.Fatalf("error = %v; started = %v", err, got)
			}

			assertNoCodexCredentials(t, filepath.Join(root, "tmp"))

			if failure != nil && string(failure.Outcome) != testCase.outcome {
				t.Fatalf("outcome = %q", failure.Outcome)
			}
		})
	}
}

func TestCodexWriterRefusesAnEditRootSymlinkOutsideTheHostBeforeSpawning(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	putCodexFile(t, filepath.Join(root, "auth/auth.json"), "{}")
	t.Setenv("CODEX_HOME", filepath.Join(root, "auth"))

	err := os.Symlink(outside, filepath.Join(root, "escape"))
	if err != nil {
		t.Fatal(err)
	}

	_, err = ai.NewCodexProvider().Execute(t.Context(), ai.Request{
		WorkDir: root, TmpDir: filepath.Join(root, "tmp/run"), CodexWriter: true,
		Mode: ai.ExecutionMode{SourceWriteRoots: []string{"escape/service"}},
	})

	assertNoCodexCredentials(t, filepath.Join(root, "tmp"))

	var started *pkgerrors.CodexTurnError
	if !errors.Is(err, os.ErrPermission) || errors.As(err, &started) {
		t.Fatalf("refusal=%v", err)
	}
}

func putCodexFile(t *testing.T, path, body string) {
	t.Helper()

	err := disk.Write(path, []byte(body), disk.Private)
	if err != nil {
		t.Fatal(err)
	}

	err = os.Chmod(path, 0o700)
	if err != nil {
		t.Fatal(err)
	}
}

func readCodexFile(t *testing.T, path string) string {
	t.Helper()

	data, err := disk.Read(path)
	if err != nil {
		t.Fatal(err)
	}

	return string(data)
}

func assertPrivateCodexHome(t *testing.T, generated string) {
	t.Helper()

	entries, err := os.ReadDir(generated)
	if err != nil || len(entries) != 1 || entries[0].Name() != "config.toml" {
		t.Fatalf("generated home = %v, %v; want only config.toml after turn", entries, err)
	}
}

func assertCodexCredentialsRemoved(t *testing.T, home string) {
	t.Helper()

	_, err := os.Stat(filepath.Join(home, "auth.json"))
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("credentials survived turn in %s: %v", home, err)
	}
}

func assertNoCodexCredentials(t *testing.T, root string) {
	t.Helper()

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.Name() == "auth.json" {
			t.Errorf("credentials survived turn: %s", path)
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
