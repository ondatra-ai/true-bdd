package cmd

import (
	"github.com/spf13/cobra"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/console"
)

// Version is the true-bdd build version string, surfaced by `true-bdd version`.
const Version = "0.1.0"

// newVersionCmd builds the `version` subcommand, which never touches the
// bootstrap container (see containerProvider) — it prints the version line.
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the true-bdd version",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			console.Printf("true-bdd version %s\n", Version)

			return nil
		},
	}
}
