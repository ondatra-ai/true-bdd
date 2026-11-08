package ai_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/pelletier/go-toml/v2"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/adapters/ai"
)

func TestCodexWriterShellHereDocumentsUseGrantedScratch(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	tools := t.TempDir()
	putCodexFile(t, filepath.Join(tools, "auth.json"), "{}")
	putCodexFile(t, filepath.Join(tools, "codex"), `#!/bin/sh
set -eu
for scratch in tmp/codex-shell-home tmp/codex-tmp tmp/codex-go-build; do
 [ -d "$scratch" ] || exit 10
done
cp "$CODEX_HOME/config.toml" profile.txt
while [ "$1" != '-o' ]; do shift; done
printf done > "$2"
`)
	t.Setenv("CODEX_HOME", tools)
	t.Setenv("PATH", tools+":"+os.Getenv("PATH"))

	_, err = ai.NewCodexProvider().Execute(t.Context(), ai.Request{
		WorkDir: root, TmpDir: filepath.Join(root, "tmp/run"), Label: "shell", CodexApply: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	var profile struct {
		Shell struct {
			Inherit string            `toml:"inherit"`
			Set     map[string]string `toml:"set"`
		} `toml:"shell_environment_policy"`
		Permissions struct {
			Writer struct {
				Filesystem map[string]string `toml:"filesystem"`
			} `toml:"writer"`
		} `toml:"permissions"`
	}

	err = toml.Unmarshal([]byte(readCodexFile(t, filepath.Join(root, "profile.txt"))), &profile)
	if err != nil {
		t.Fatal(err)
	}

	if profile.Shell.Inherit != "none" {
		t.Fatalf("shell inherits %q", profile.Shell.Inherit)
	}

	want := filepath.Join(root, "tmp/codex-tmp/zsh")
	if profile.Shell.Set["TMPPREFIX"] != want {
		t.Fatalf("TMPPREFIX = %q, want %q", profile.Shell.Set["TMPPREFIX"], want)
	}

	if profile.Permissions.Writer.Filesystem[filepath.Join(root, "tmp")] != "write" {
		t.Fatal("here-document scratch parent is not granted")
	}

	t.Run("zsh here-document", func(t *testing.T) {
		runCodexShellHereDocument(t, root, profile.Shell.Set)
	})
}

func runCodexShellHereDocument(t *testing.T, root string, environment map[string]string) {
	t.Helper()

	zsh, err := exec.LookPath("zsh")
	if err != nil {
		t.Skip("zsh is not installed")
	}

	command := exec.CommandContext(t.Context(), zsh, "-f", "-c", "cat <<'EOF'\nwriter scratch\nEOF\n")
	command.Dir = root

	command.Env = make([]string, 0, len(environment))
	for key, value := range environment {
		command.Env = append(command.Env, key+"="+value)
	}

	output, err := command.CombinedOutput()
	if err != nil || string(output) != "writer scratch\n" {
		t.Fatalf("zsh here-document: %v; output = %q", err, output)
	}
}
