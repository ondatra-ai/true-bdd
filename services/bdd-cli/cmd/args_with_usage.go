package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// argsWithUsage wraps a positional-argument validator so its error carries
// the command's usage text — restoring it on the one path SilenceUsage
// otherwise kills too: a mistyped argument, where the usage IS the answer.
func argsWithUsage(validate cobra.PositionalArgs) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		err := validate(cmd, args)
		if err == nil {
			return nil
		}

		return fmt.Errorf("%w\n\n%s", err, cmd.UsageString())
	}
}
