package cmd

import (
	"bdd-cli/src/internal/app/bootstrap"
	"bdd-cli/src/internal/pkg/console"
	"github.com/spf13/cobra"
)

func NewBuildCommand(container *bootstrap.Container) *cobra.Command {
	buildCmd := &cobra.Command{
		Use:   "build",
		Short: "Build commands",
	}

	buildCmd.AddCommand(newBuildTestsCmd(container))
	buildCmd.AddCommand(newBuildCodeCmd(container))

	return buildCmd
}

func newBuildTestsCmd(_ *bootstrap.Container) *cobra.Command {
	return &cobra.Command{
		Use:   "tests",
		Short: "Build tests (not yet implemented)",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			console.Println("build tests: not yet implemented")

			return nil
		},
	}
}

func newBuildCodeCmd(_ *bootstrap.Container) *cobra.Command {
	return &cobra.Command{
		Use:   "code",
		Short: "Build code (not yet implemented)",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			console.Println("build code: not yet implemented")

			return nil
		},
	}
}
