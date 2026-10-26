package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

var errYesRequiresFix = errors.New("--yes requires --fix")

// argsWithUsage validates positional arguments and the --yes/--fix pair,
// attaching usage text to usage errors despite SilenceUsage.
func argsWithUsage(validate cobra.PositionalArgs) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		err := validate(cmd, args)
		if err == nil && cmd.Flags().Lookup("yes") != nil {
			yes, _ := cmd.Flags().GetBool("yes")

			fix, _ := cmd.Flags().GetBool("fix")
			if yes && !fix {
				err = errYesRequiresFix
			}
		}

		if err == nil {
			return nil
		}

		return fmt.Errorf("%w\n\n%s", err, cmd.UsageString())
	}
}
