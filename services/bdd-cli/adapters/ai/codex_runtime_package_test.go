package ai_test

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/adapters/ai"
)

const validRuntimePackage = "valid-package"

func TestCodexApplyValidatesRuntimePackageBoundary(t *testing.T) {
	for _, location := range []string{validRuntimePackage, "wrong-name", "missing-manifest", "host", "home",
		".codex", ".claude", "work", ".ssh", ".aws", ".config"} {
		t.Run(location, func(t *testing.T) {
			root, tools, userHome, installed := t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()
			packageRoot := codexProbePackageRoot(location, root, userHome, installed)

			manifest := `{"name":"npm"}`
			if location == "wrong-name" {
				manifest = `{"name":"other"}`
			}

			if location != "missing-manifest" {
				putCodexFile(t, filepath.Join(packageRoot, "package.json"), manifest)
			}

			putCodexFile(t, filepath.Join(packageRoot, "bin/npm-cli.js"), "#!/usr/bin/env node\n")

			err := os.Symlink(filepath.Join(packageRoot, "bin/npm-cli.js"), filepath.Join(tools, "npm"))
			if err != nil {
				t.Fatal(err)
			}

			putCodexFile(t, filepath.Join(tools, "auth.json"), "{}")
			putCodexFile(t, filepath.Join(tools, "codex"), captureCodexRequest)
			putCodexFile(t, filepath.Join(tools, "npx"), "#!/bin/sh\nexit 0\n")
			t.Setenv("HOME", userHome)
			t.Setenv("CODEX_HOME", tools)
			t.Setenv("PATH", tools+":"+os.Getenv("PATH"))

			var logs bytes.Buffer

			previous := slog.Default()

			slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
			t.Cleanup(func() { slog.SetDefault(previous) })

			_, err = ai.NewCodexProvider().Execute(t.Context(), ai.Request{
				WorkDir: root, TmpDir: filepath.Join(root, "tmp/run"), Label: "package-boundary", CodexApply: true,
			})
			if err != nil {
				t.Fatal(err)
			}

			assertCodexRuntimePackage(t, location, root, packageRoot, logs.String())
		})
	}
}

func assertCodexRuntimePackage(t *testing.T, location, root, packageRoot, logs string) {
	t.Helper()

	resolved, err := filepath.EvalSymlinks(packageRoot)
	if err != nil {
		t.Fatal(err)
	}

	config := readCodexFile(t, filepath.Join(root, "tmp/codex-homes/run/package-boundary-codex-home/config.toml"))

	granted := strings.Contains(config, resolved+`" = "read"`)
	if granted != (location == validRuntimePackage) {
		t.Fatalf("package granted=%v; location=%s", granted, location)
	}

	_, linkErr := os.Lstat(filepath.Join(root, "tmp/codex-shell-home/runtime-bin/npm"))
	if (linkErr == nil) != granted {
		t.Fatalf("refused runtime linked: %v", linkErr)
	}

	warnings := strings.Count(logs, "level=WARN")
	if (granted && warnings != 0) || (!granted && warnings != 1) {
		t.Fatalf("warnings=%d; logs=%s", warnings, logs)
	}
}

func codexProbePackageRoot(location, root, userHome, installed string) string {
	packageRoot := filepath.Join(installed, "npm")

	switch location {
	case "host":
		packageRoot = filepath.Join(root, "npm")
	case "home":
		packageRoot = userHome
	case ".codex", ".claude", "work", ".ssh", ".aws", ".config":
		packageRoot = filepath.Join(userHome, location, "npm")
	}

	return packageRoot
}
