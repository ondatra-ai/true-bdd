package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// argsWithUsage attaches usage text to positional argument errors despite SilenceUsage.
func argsWithUsage(validate cobra.PositionalArgs) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		err := validate(cmd, args)
		if err == nil {
			return nil
		}

		return fmt.Errorf("%w\n\n%s", err, cmd.UsageString())
	}
}
