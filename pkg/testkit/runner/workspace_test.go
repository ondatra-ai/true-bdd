package runner_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/runner"
)

func TestFixtureCommandsIgnoreParentWorkspace(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{".git", "true-bdd", "templates", "fixture/input"} {
		err := os.MkdirAll(filepath.Join(root, dir), 0o700)
		if err != nil {
			t.Fatal(err)
		}
	}

	write := func(name, body string) {
		t.Helper()

		err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o700)
		if err != nil {
			t.Fatal(err)
		}
	}
	write("go.work", "go 1.25.0\nuse .\n")
	write("fixture/prep.sh", "go env GOWORK > prep-workspace\n")
	write("fixture/teardown.sh", "go env GOWORK > teardown-workspace\n")
	write("engine", "#!/bin/sh\ngo env GOWORK\n")
	t.Chdir(root)
	t.Setenv("GOWORK", filepath.Join(root, "go.work"))

	fixture, err := runner.LoadFixture(filepath.Join(root, "fixture"))
	if err != nil {
		t.Fatal(err)
	}

	result, err := runner.Execute(t.Context(), fixture, filepath.Join(root, "engine"),
		filepath.Join(root, "runs"), time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	runner.Teardown(result.TmpDir, fixture.TeardownCmds)

	if strings.TrimSpace(result.Stdout) != "off" {
		t.Errorf("engine inherited workspace: %s", result.Stdout)
	}

	for _, name := range []string{"prep-workspace", "teardown-workspace"} {
		data, err := os.ReadFile(filepath.Join(result.TmpDir, name))
		if err != nil || strings.TrimSpace(string(data)) != "off" {
			t.Errorf("%s = %s, error %v; want off", name, data, err)
		}
	}
}
