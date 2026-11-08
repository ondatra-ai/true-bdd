package runner

import (
	"slices"
	"testing"
)

// TestCommandArgsNameTheRunDir: `{{CWD}}` in the scenario's command stands
// for the run dir, so a command can name a file of the tree by an absolute
// path; a run dir holding a space stays inside its one argument.
func TestCommandArgsNameTheRunDir(t *testing.T) {
	t.Parallel()

	cases := []struct {
		cmd, runDir string
		want        []string
	}{
		{
			"build tests --requirements {{CWD}}/docs/specs/requirements.yaml", "/runs/fixture",
			[]string{"build", "tests", "--requirements", "/runs/fixture/docs/specs/requirements.yaml"},
		},
		{"scen check {{CWD}}", "/runs/a fixture", []string{"scen", "check", "/runs/a fixture"}},
		{"build tests  --fix", "/runs/fixture", []string{"build", "tests", "--fix"}},
	}

	for _, tc := range cases {
		got := commandArgs(tc.cmd, tc.runDir)
		if !slices.Equal(got, tc.want) {
			t.Errorf("commandArgs(%q, %q) = %q, want %q", tc.cmd, tc.runDir, got, tc.want)
		}
	}
}
