package cmd_test

import (
	"testing"

	"github.com/ondatra-ai/true-bdd/src/cmd"
)

// The default must stay a filesystem path relative to the host project
// root (the true-bdd/ config dir convention) — a module-path-style
// value here once slipped in during a rename and broke `build code`.
func TestBuildCodeArchitectureFlagDefault(t *testing.T) {
	t.Parallel()

	buildCmd := cmd.NewBuildCommand(nil)

	codeCmd, _, err := buildCmd.Find([]string{"code"})
	if err != nil {
		t.Fatalf("find `build code` subcommand: %v", err)
	}

	got, err := codeCmd.Flags().GetString("architecture")
	if err != nil {
		t.Fatalf("read --architecture flag: %v", err)
	}

	want := "true-bdd/architecture.yaml"
	if got != want {
		t.Fatalf("default --architecture = %q, want %q", got, want)
	}
}
