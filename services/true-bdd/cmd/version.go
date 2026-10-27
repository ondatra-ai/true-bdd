package cmd

import (
	"github.com/spf13/cobra"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/buildinfo"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/console"
)

// newVersionCmd builds the `version` subcommand, which never touches the
// bootstrap container (see containerProvider) — it prints the build identity.
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the true-bdd build version and commit",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			info := buildinfo.Current()
			console.Printf("true-bdd version %s commit %s\n", info.Version, info.Commit)

			return nil
		},
	}
}
