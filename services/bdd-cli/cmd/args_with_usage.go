package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// argsWithUsage wraps a positional-argument validator so its error
// carries the command's usage text.
//
// The `build` commands set SilenceUsage because everything they RETURN
// is a run failure, and a help dump after "your architectural spec is
// incomplete" reads as though the flags were typed wrong. But cobra's
// switch covers every error alike, including the one raised for a
// mistyped argument — and there the usage text is exactly the answer.
// Attaching it to that error restores it on that path alone.
func argsWithUsage(validate cobra.PositionalArgs) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		err := validate(cmd, args)
		if err == nil {
			return nil
		}

		return fmt.Errorf("%w\n\n%s", err, cmd.UsageString())
	}
}
