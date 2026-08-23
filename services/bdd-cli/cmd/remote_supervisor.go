package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/remote"
)

// newRemoteSupervisorCmd builds the hidden `remote-supervisor` subcommand:
// the resident gated group-leader launcher the remote spawns per run. Flag
// parsing is DISABLED so the real command's own flags (e.g. `--fix`) pass through untouched.
func newRemoteSupervisorCmd() *cobra.Command {
	return &cobra.Command{
		Use:                remote.SupervisorSubcommand,
		Short:              "Hidden resident gated group-leader launcher (internal)",
		Hidden:             true,
		DisableFlagParsing: true,
		RunE: func(_ *cobra.Command, args []string) error {
			os.Exit(remote.RunSupervisor(args))

			return nil
		},
	}
}
