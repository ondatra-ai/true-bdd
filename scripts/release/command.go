package release

import (
	"fmt"
	"io"

	"github.com/ondatra-ai/true-bdd/pkg/cli/git"
	"github.com/spf13/cobra"
)

// Command is the repository's artifact build and rehearsal interface.
func Command() *cobra.Command {
	command := &cobra.Command{Use: "release", SilenceUsage: true, SilenceErrors: true}
	command.SetOut(io.Discard)
	command.SetErr(io.Discard)
	command.AddCommand(buildCommand(), rehearseCommand())

	return command
}

func buildCommand() *cobra.Command {
	var version, out string

	command := &cobra.Command{Use: "build --version <v> --out <dir>", Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if version == "" || out == "" {
				return fmt.Errorf("%w: --version and --out are required", errRefused)
			}

			root, err := git.TopLevel()
			if err != nil {
				return fmt.Errorf("locating repository: %w", err)
			}

			return Build(command.Context(), root, version, out)
		}}
	command.Flags().StringVar(&version, "version", "", "archive version (stamping waits for #220)")
	command.Flags().StringVar(&out, "out", "", "artifact directory")

	return command
}

func rehearseCommand() *cobra.Command {
	var (
		artifacts string
		opt       Options
	)

	command := &cobra.Command{Use: "rehearse --artifacts <dir> [--live]", Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if artifacts == "" {
				return fmt.Errorf("%w: --artifacts is required", errRefused)
			}

			root, err := git.TopLevel()
			if err != nil {
				return fmt.Errorf("locating repository: %w", err)
			}

			_, err = Rehearse(command.Context(), root, artifacts, opt)

			return err
		}}
	command.Flags().StringVar(&artifacts, "artifacts", "", "directory containing archives, checksums.txt and release.json")
	command.Flags().BoolVar(&opt.Live, "live", false, "repair hosts with real AI CLIs (costs money)")
	command.Flags().StringVar(&opt.HarnessRevision, "harness-revision", "",
		"require harness from proxy at this revision instead of exported source")

	return command
}
