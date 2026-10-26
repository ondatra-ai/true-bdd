package ai_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/adapters/ai"
)

const codexStoryFile = "docs/story.yaml"

func TestCodexApplyGrantsFilesGlobsAndScratchExactly(t *testing.T) {
	root := t.TempDir()
	tools := t.TempDir()
	putCodexFile(t, filepath.Join(tools, "auth.json"), "{}")
	putCodexFile(t, filepath.Join(tools, "codex"), captureCodexRequest)
	t.Setenv("CODEX_HOME", tools)
	t.Setenv("PATH", tools+":"+os.Getenv("PATH"))
	putCodexFile(t, filepath.Join(root, codexStoryFile), "original")
	putCodexFile(t, filepath.Join(root, "tests/alpha/steps/existing.go"), "package steps")

	_, err := ai.NewCodexProvider().Execute(t.Context(), ai.Request{
		WorkDir: root, TmpDir: filepath.Join(root, "tmp/run"), Label: "apply", CodexApply: true,
		Mode: ai.ExecutionMode{AllowedTools: []string{"Write(docs/story.yaml)", "Write(docs/new.yaml)",
			"Write(docs/fresh/new.yaml)",
			"Edit(tests/*/steps/**)", "Write(tmp/**)"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	home := filepath.Join(root, "tmp/codex-homes/run/apply-codex-home")

	config := readCodexFile(t, filepath.Join(home, "config.toml"))
	for _, path := range []string{codexStoryFile, "docs/new.yaml", "docs/fresh/new.yaml", "tests", "tmp"} {
		if !strings.Contains(config, filepath.Join(root, path)+`" = "write"`) {
			t.Errorf("missing exact grant %s: %s", path, config)
		}
	}

	for _, path := range []string{"docs", "docs/fresh", "tests/alpha"} {
		if strings.Contains(config, filepath.Join(root, path)+`" = "write"`) {
			t.Errorf("broadened grant: %s", path)
		}
	}

	info, err := os.Stat(filepath.Join(root, codexStoryFile))
	if err != nil || info.IsDir() {
		t.Fatalf("file root became directory: %v", err)
	}

	assertCodexCredentialsRemoved(t, home)
}

func TestCodexApplyProvidesOnlyInstalledRuntimeExceptions(t *testing.T) {
	root := t.TempDir()
	tools := t.TempDir()
	putCodexFile(t, filepath.Join(tools, "auth.json"), "{}")
	putCodexFile(t, filepath.Join(tools, "codex"), runtimeBoundaryProbeScript)

	for _, name := range []string{"node", "npx", "docker"} {
		putCodexFile(t, filepath.Join(tools, name), "#!/bin/sh\nexit 0\n")
	}

	putCodexFile(t, filepath.Join(tools, "bin/npm-cli.js"), "#!/usr/bin/env node\n")

	err := os.Symlink("bin/npm-cli.js", filepath.Join(tools, "npm"))
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("TRUE_BDD_RUNTIME_MARKERS", tools)
	t.Setenv("CODEX_HOME", tools)
	t.Setenv("PATH", tools+":"+os.Getenv("PATH"))

	_, err = ai.NewCodexProvider().Execute(t.Context(), ai.Request{
		WorkDir: root, TmpDir: filepath.Join(root, "tmp/run"), Label: "apply", CodexApply: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	config := readCodexFile(t, filepath.Join(root, "tmp/codex-homes/run/apply-codex-home/config.toml"))
	for _, name := range []string{"node", "npx", "docker"} {
		if !strings.Contains(config, filepath.Join(tools, name)+`" = "read"`) {
			t.Errorf("runtime %s unavailable", name)
		}
	}

	if strings.Contains(config, tools+`" = "read"`) {
		t.Fatal("runtime exposed adjacent files")
	}
}

func TestCodexApplySingleFileAndFileGlobLive(t *testing.T) {
	root := t.TempDir()
	tools := t.TempDir()
	putCodexFile(t, filepath.Join(tools, "auth.json"), "{}")
	putCodexFile(t, filepath.Join(tools, "codex"), singleFileProbeScript)
	putCodexFile(t, filepath.Join(root, codexStoryFile), "original")
	putCodexFile(t, filepath.Join(root, "tests/a/alpha.go"), "original")
	t.Setenv("CODEX_HOME", tools)
	t.Setenv("PATH", tools+":"+os.Getenv("PATH"))

	_, err := ai.NewCodexProvider().Execute(t.Context(), ai.Request{
		WorkDir: root, TmpDir: filepath.Join(root, "tmp/run"), Label: "files", CodexApply: true,
		Mode: ai.ExecutionMode{AllowedTools: []string{"Write(docs/story.yaml)", "Write(docs/new.yaml)",
			"Write(docs/fresh/new.yaml)",
			"Write(tests/**/*.go)"}},
	})
	if os.Getenv("TRUE_BDD_CODEX_SANDBOX_PROBE") != "" {
		t.Log(readCodexFile(t, filepath.Join(root, "tmp/probe.log")))

		if readCodexFile(t, filepath.Join(root, "tests/a/new.go")) != "created" {
			t.Fatal("matching file not created")
		}

		_, statErr := os.Stat(filepath.Join(root, "tests/a/beta.txt"))
		if !os.IsNotExist(statErr) {
			t.Fatalf("unmatched new file survived: %v", statErr)
		}
	}

	if err != nil {
		t.Fatal(err)
	}
}

const singleFileProbeScript = `#!/bin/sh
set -eu
if [ -n "${TRUE_BDD_CODEX_SANDBOX_PROBE:-}" ]; then
 probe="$TRUE_BDD_CODEX_SANDBOX_PROBE"
 "$probe" sandbox -C "$PWD" -P writer /bin/sh -ec '
  printf changed > docs/story.yaml
  printf created > docs/new.yaml
  printf created > docs/fresh/new.yaml
  printf changed > tests/a/alpha.go
  printf created > tests/a/new.go
  printf forbidden > tests/a/beta.txt
 ' > tmp/probe.log 2>&1
 deny() {
  printf 'DENY: %s\n' "$*" >> tmp/probe.log
  if "$probe" sandbox -C "$PWD" -P writer "$@" >> tmp/probe.log 2>&1; then exit 71; fi
  tail -n 2 tmp/probe.log | grep 'Operation not permitted' >/dev/null || exit 72
 }
 deny /bin/sh -c 'printf forbidden > docs/sibling.yaml'
 deny /bin/sh -c 'printf forbidden > docs/fresh/sibling.yaml'
 deny /bin/sh -ec 'rm docs/story.yaml; mkdir docs/story.yaml; printf forbidden > docs/story.yaml/child'
 deny /bin/sh -ec 'rm docs/new.yaml; mkdir docs/new.yaml; printf forbidden > docs/new.yaml/child'
 printf 'GRANTED: existing file, absent file, existing and new recursive file-glob matches\n' >> tmp/probe.log
fi
while [ "$1" != '-o' ]; do shift; done
printf done > "$2"
`

func TestCodexApplyRefusesOutsideScratchBeforeCreatingIt(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	scratch := filepath.Join(outside, "new-scratch")

	_, err := ai.NewCodexProvider().Execute(t.Context(), ai.Request{WorkDir: root, TmpDir: scratch, CodexApply: true})
	if err == nil {
		t.Fatal("outside scratch accepted")
	}

	_, statErr := os.Stat(scratch)
	if !os.IsNotExist(statErr) {
		t.Fatalf("outside scratch created: %v", statErr)
	}
}

const runtimeBoundaryProbeScript = `#!/bin/sh
set -eu
if [ -n "${TRUE_BDD_CODEX_SANDBOX_PROBE:-}" ]; then
 "$TRUE_BDD_CODEX_SANDBOX_PROBE" sandbox -C "$PWD" -P writer /bin/sh -ec '
  if cat "$1/auth.json"; then exit 81; fi
  if cat "$1/bin/npm-cli.js"; then exit 82; fi
  if command -v npm; then exit 83; fi
 ' sh "$TRUE_BDD_RUNTIME_MARKERS" > tmp/runtime-probe.log 2>&1
fi
while [ "$1" != '-o' ]; do shift; done
printf done > "$2"
`
