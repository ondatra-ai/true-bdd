package cmd_test

import (
	"testing"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/cmd"
)

// Both path flags must register with an EMPTY default: an unset flag is
// the signal that the config (documents.architecture_yaml /
// documents.scenarios_yaml) decides the path at RunE time. A literal
// path here would silently bypass the host config — that hardcode is
// exactly what this test guards against re-entering.
func TestBuildPathFlagDefaultsAreEmpty(t *testing.T) {
	t.Parallel()

	cases := []struct {
		subcommand string
		flag       string
	}{
		{subcommand: "code", flag: "architecture"},
		{subcommand: "tests", flag: "requirements"},
	}

	for _, testCase := range cases {
		t.Run(testCase.subcommand+" --"+testCase.flag, func(t *testing.T) {
			t.Parallel()

			buildCmd := cmd.NewBuildCommand(nil)

			subCmd, _, err := buildCmd.Find([]string{testCase.subcommand})
			if err != nil {
				t.Fatalf("find `build %s` subcommand: %v", testCase.subcommand, err)
			}

			got, err := subCmd.Flags().GetString(testCase.flag)
			if err != nil {
				t.Fatalf("read --%s flag: %v", testCase.flag, err)
			}

			if got != "" {
				t.Fatalf("default --%s = %q, want empty (config-resolved)", testCase.flag, got)
			}
		})
	}
}
