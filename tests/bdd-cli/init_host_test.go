package bddcli_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/cli"
	"github.com/ondatra-ai/true-bdd/pkg/cli/gotool"
	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

// The installed binary must initialize and reach a real process boundary outside its source tree.
func TestInitializedHostReachesFirstAITurn(t *testing.T) {
	t.Parallel()
	host := t.TempDir()

	binary := filepath.Join(t.TempDir(), "bin", "true-bdd")

	err := gotool.Build(cli.Options{}, "../..", binary, "./services/bdd-cli")
	if err != nil {
		t.Fatal(err)
	}

	env := append(os.Environ(), "PATH="+filepath.Join(host, "fake-bin")+string(os.PathListSeparator)+os.Getenv("PATH"))
	for _, command := range []string{"claude", "codex"} {
		path := filepath.Join(host, "fake-bin", command)
		writeHostFile(t, path, "#!/bin/sh\ncat >/dev/null\nprintf '%s\\n' "+command+" >> ai-reached\nexit 3\n")

		err := disk.Executable(path)
		if err != nil {
			t.Fatal(err)
		}
	}

	runInstalled(t, host, binary, env, "init")
	runInstalled(t, host, binary, env, "init")
	writeHostFile(t, filepath.Join(host, "go.mod"), "module fixture/initialized\n\ngo 1.25.0\n")
	writeHostFile(t, filepath.Join(host, "docs/product/product.yaml"), "title: Calculator\n")
	writeHostFile(t, filepath.Join(host, "docs/scenarios.yaml"), "scenarios: {}\n")
	writeHostFile(t, filepath.Join(host, "docs/architecture/architecture.yaml"), `architecture:
  services:
    - name: calc
      path: services/calc
      language: go
  testing:
    framework: go-test
    commands:
      record: go test -json -count=1 ./tests/calc/...
      replay: go test -json -count=1 ./tests/calc/...
      live: go test -json -count=1 ./tests/calc/...
`)
	writeHostFile(t, filepath.Join(host, "tests/calc/calc_test.go"), `package calc_test
import "testing"
func TestNeedsImplementation(t *testing.T) { t.Fatal("implement calculator") }
`)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	command := exec.CommandContext(ctx, binary, "build", "code", "--fix")
	command.Dir, command.Env = host, env

	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("fake AI exits 3, but run succeeded: %s", output)
	}

	reached, readErr := disk.Read(filepath.Join(host, "ai-reached"))
	if readErr != nil {
		t.Fatalf("never reached fake AI: %v\n%s", readErr, output)
	}

	if string(reached) != "claude\n" {
		t.Fatalf("first turn = %q, want one Claude planning turn", reached)
	}

	if !strings.Contains(string(output), "AI turn") {
		t.Fatalf("missing AI turn record: %s", output)
	}
}

func writeHostFile(t *testing.T, path, content string) {
	t.Helper()

	err := disk.Write(path, []byte(content), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}
}

func runInstalled(t *testing.T, host, binary string, env []string, verb string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	command := exec.CommandContext(ctx, binary, verb)
	command.Dir, command.Env = host, env

	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("%s: %v\n%s", verb, err, output)
	}
}
