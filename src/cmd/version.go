package cmd

import (
	"github.com/spf13/cobra"

	"github.com/ondatra-ai/true-bdd/src/internal/infrastructure/events"
	"github.com/ondatra-ai/true-bdd/src/internal/pkg/console"
)

// Version is the true-bdd build version string. Surfaced by `true-bdd
// version` and echoed by the harness "Test connection" control (P8).
const Version = "0.1.0"

// newVersionCmd builds the `version` subcommand — a real, non-Claude,
// non-interactive command that constructs no bootstrap container. It
// prints the version line and, when the event channel is enabled (under
// `true-bdd remote`), emits an `ok` result event (plan §3.1/§3.2).
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the true-bdd version",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			console.Printf("true-bdd version %s\n", Version)
			events.NewEmitter().EmitResult("ok", true, "")

			return nil
		},
	}
}
