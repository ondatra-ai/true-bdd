package testkit_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/testkit"
)

func TestRunCommandSetsGoworkOffAndStripsClaudecode(t *testing.T) {
	t.Setenv("GOWORK", "/inherited/go.work")
	t.Setenv("CLAUDECODE", "parent-session")
	t.Setenv("TRUE_BDD_COMMAND_MARKER", "inherited")
	directory := t.TempDir()

	result, err := testkit.RunCommand([]string{"sh", "-c",
		`printf '%s\n%s\n%s\n%s\n' "$GOWORK" "${CLAUDECODE+present}" "$TRUE_BDD_COMMAND_MARKER" "$PWD"`},
		directory, time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	wantDirectory, err := filepath.EvalSymlinks(directory)
	if err != nil {
		t.Fatal(err)
	}

	if result.Code != 0 || result.Stdout != "off\n\ninherited\n"+wantDirectory+"\n" || result.Stderr != "" {
		t.Fatalf("command result: %+v", result)
	}
}
