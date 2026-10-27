package cmd_test

import (
	"io"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/cmd"
	"github.com/spf13/cobra"
)

func TestFixCommandsRejectRemovedApprovalFlagBeforeStartup(t *testing.T) {
	t.Parallel()

	for _, invocation := range []string{
 "build code", "build tests", "us refine example", "us apply example", "scen check",
 } {
 arguments := strings.Fields(invocation)
		t.Run(strings.Join(arguments, " "), func(t *testing.T) {
			t.Parallel()

			for _, fix := range []bool{false, true} {
				root := &cobra.Command{Use: "true-bdd", SilenceErrors: true, SilenceUsage: true}
				root.SetOut(io.Discard)
				root.SetErr(io.Discard)
				root.AddCommand(cmd.NewBuildCommand(nil), cmd.NewUSCommand(nil), cmd.NewScenCommand(nil))

				args := append([]string{}, arguments...)
				if fix {
					args = append(args, "--fix")
				}

				removedFlag := "--" + "yes"
				root.SetArgs(append(args, removedFlag))

				err := root.Execute()
				if err == nil || !strings.Contains(err.Error(), "unknown flag: "+removedFlag) {
					t.Fatalf("expected unknown flag before startup, got %v", err)
				}
			}
		})
	}
}
