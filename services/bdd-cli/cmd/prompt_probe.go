package cmd

import (
	"github.com/spf13/cobra"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/promptprobe"
)

// newPromptProbeCmd builds the hidden `prompt-probe` subcommand (plan §4): a
// deterministic, non-Claude prompt driver that emits choice → clarify →
// freetext prompts through the real answer collector and reads answers from
// stdin. Protocol tests dispatch it to exercise the prompt dialogs, the
// execution barrier (a blocked child holding the folder flock), and the answer
// path without spawning `claude`. Like `version`, it builds no bootstrap
// container, so it runs honestly in any folder.
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
