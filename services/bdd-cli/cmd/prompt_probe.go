package cmd

import (
	"github.com/spf13/cobra"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/promptprobe"
)

// newPromptProbeCmd builds the hidden `prompt-probe` subcommand (plan §4): a
// deterministic, non-Claude prompt driver protocol tests dispatch to exercise
// dialogs and the execution barrier — like `version`, it builds no container.
func newPromptProbeCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "prompt-probe",
		Short:  "Hidden deterministic prompt driver for protocol tests",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			promptprobe.Drive(cmd.InOrStdin())

			return nil
		},
	}
}
