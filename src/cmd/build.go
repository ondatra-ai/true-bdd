package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"bdd-cli/src/internal/app/bootstrap"
	"bdd-cli/src/internal/app/commands"
	"bdd-cli/src/internal/pkg/console"
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

func newBuildTestsCmd(container *bootstrap.Container) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tests",
		Short: "Walk the requirements registry and check every scenario has an executable test",
		Long: `Walk every scenario in docs/requirements.yaml against the build-tests
checklist. The checklist asks whether each scenario id is referenced by an
executable test under tests/integration/, tests/e2e/, services/backend/, or
services/frontend/. With --fix, every failed (scenario, prompt) cell drives a
Claude-mediated test-authoring turn that Writes or Edits a test file under
the allowed roots; docs/requirements.yaml is never touched. The CLI exits
non-zero if any scenario is still uncovered after the walk.

Example:
  bdd-cli build tests
  bdd-cli build tests --fix
  bdd-cli build tests --requirements docs/requirements.yaml`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, stop := signal.NotifyContext(context.Background(),
				os.Interrupt, syscall.SIGTERM)
			defer stop()

			requirementsFile, _ := cmd.Flags().GetString("requirements")
			fix, _ := cmd.Flags().GetBool("fix")

			err := commands.RunBuildTests(ctx, commands.BuildTestsDeps{
				RegistryLoader:               container.RegistryLoader,
				ChecklistLoader:              container.ChecklistLoader,
				BuildTestsEvaluator:          container.BuildTestsEvaluator,
				BuildTestsFixPromptGenerator: container.BuildTestsFixPromptGenerator,
				BuildTestsFixApplier:         container.BuildTestsFixApplier,
				UserInputCollector:           container.UserInputCollector,
				TableRenderer:                container.TableRenderer,
				RunDir:                       container.RunDir,
			}, requirementsFile, fix)

			stop()

			if err != nil {
				return fmt.Errorf("build tests: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().String("requirements", defaultRequirementsFile,
		"Path to the requirements registry YAML")
	cmd.Flags().Bool("fix", false, fixFlagDescription)

	return cmd
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
