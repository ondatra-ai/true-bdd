package cmd_test

import (
	"testing"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/cmd"
)

// TestBuildPathFlagDefaultsAreEmpty checks both path flags default to
// empty, so an unset flag defers to host config (documents.architecture_yaml
// / documents.scenarios_yaml) at RunE time instead of a hardcoded path.
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
