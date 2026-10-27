package release_test

import (
	"github.com/ondatra-ai/true-bdd/pkg/cli/git"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func sourceRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write(t, filepath.Join(root, "go.mod"), "module github.com/ondatra-ai/true-bdd\n\ngo 1.25.0\n")
	write(t, filepath.Join(root, "pkg/testkit/bddgo/harness.go"), "package bddgo\n")
	fixture := filepath.Join(root, "tests/bdd-cli/fixtures/build-code-spec-scenario/input")
	write(t, filepath.Join(fixture, "go.mod"), "module fixture/stepsrun\n\ngo 1.25.0\n")
	write(t, filepath.Join(fixture, "services/calc/calc.go"), "package calc\n")
	write(t, filepath.Join(fixture, "docs/architecture/architecture.yaml"), `architecture:
  testing:
    framework: go-test
    libraries:
      - module: github.com/ondatra-ai/true-bdd
        version: v0.0.0
`)

	write(t, filepath.Join(fixture, "true-bdd/true-bdd.yaml"), "poison\n")
	write(t, filepath.Join(fixture, "templates/poison"), "poison\n")

	for _, args := range [][]string{
		{"init", "--template="}, {"add", "."},
		{"-c", "core.hooksPath=/dev/null", "-c", "user.name=Test",
			"-c", "user.email=test@localhost", "-c", "commit.gpgsign=false", "commit", "-m", "source"},
	} {
		command := exec.CommandContext(t.Context(), "git", args...)

		command.Dir = root

		command.Env = git.ScrubEnvironment(os.Environ())

		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %s: %v", args, output, err)
		}
	}

	toolPath := t.TempDir()

	for _, name := range []string{"go", "git"} {
		target, err := exec.LookPath(name)
		if err != nil {
			t.Fatal(err)
		}

		err = os.Symlink(target, filepath.Join(toolPath, name))
		if err != nil {
			t.Fatal(err)
		}
	}

	t.Setenv("PATH", toolPath)

	return root
}
