package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"bdd-cli/src/internal/app/bootstrap"
	"bdd-cli/src/internal/app/commands"
	"github.com/spf13/cobra"
)

const defaultRequirementsFile = "docs/requirements.yaml"

func NewUSCommand(container *bootstrap.Container) *cobra.Command {
	usCmd := &cobra.Command{
		Use:   "us",
		Short: "User story commands",
	}

	usCmd.AddCommand(newUSCreateCmd(container))
	usCmd.AddCommand(newUSRefineCmd(container))
	usCmd.AddCommand(newUSApplyCmd(container))

	return usCmd
}

// newUSChecklistCmd builds a subcommand that drives a per-story checklist.
func newUSChecklistCmd(
	container *bootstrap.Container,
	use string,
	short string,
	long string,
	config commands.CommandConfig,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Long:  long,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(context.Background(),
				os.Interrupt, syscall.SIGTERM)
			defer stop()

			fix, _ := cmd.Flags().GetBool("fix")

			err := container.USValidationCmd.Execute(ctx, args[0], fix, config)

			stop()

			if err != nil {
				return fmt.Errorf("%s command failed: %w", config.CommandName, err)
			}

			return nil
		},
	}

	cmd.Flags().Bool("fix", false,
		"Enable interactive fix mode to resolve failed checks")

	return cmd
}

func newUSCreateCmd(container *bootstrap.Container) *cobra.Command {
	return newUSChecklistCmd(
		container,
		"create [story-number]",
		"Create and validate a user story",
		`Extract a story from its epic and validate it against the us-create
checklist. The story is saved to docs/stories/ upon passing all checks.

Example:
  bdd-cli us create 4.1
  bdd-cli us create 4.1 --fix`,
		commands.CommandConfig{
			CommandName:   "us create",
			ChecklistName: "us-create",
			LoadFromEpic:  true,
		},
	)
}

func newUSRefineCmd(container *bootstrap.Container) *cobra.Command {
	return newUSChecklistCmd(
		container,
		"refine [story-number]",
		"Refine a user story",
		`Load a story from docs/stories/ and validate it against the us-refine
checklist. The story file is updated in place upon passing all checks.

Example:
  bdd-cli us refine 4.1
  bdd-cli us refine 4.1 --fix`,
		commands.CommandConfig{
			CommandName:   "us refine",
			ChecklistName: "us-refine",
			LoadFromEpic:  false,
		},
	)
}

func newUSApplyCmd(container *bootstrap.Container) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apply [story-number]",
		Short: "Apply scenarios from a refined user story into the registry",
		Long: `Walk every acceptance criterion in docs/stories/<story-number>-*.yaml and
validate each one against the us-apply checklist. With --fix, every failed
(AC, prompt) cell drives a Claude-mediated edit on a scratch copy of
docs/requirements.yaml. The canonical registry file is replaced atomically
only when every AC passes every prompt; otherwise it is left untouched.

Stories that still use the deprecated scenarios.test_scenarios[] format are
rejected — convert them to acceptance_criteria with embedded steps first.

Example:
  bdd-cli us apply 4.1
  bdd-cli us apply 4.1 --fix`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(context.Background(),
				os.Interrupt, syscall.SIGTERM)
			defer stop()

			fix, _ := cmd.Flags().GetBool("fix")

			err := container.USValidationCmd.ExecuteStoryScenarioChecklist(
				ctx,
				args[0],
				defaultRequirementsFile,
				"us-apply",
				fix,
				container.StoryScenarioParser,
				container.ApplyEvaluator,
				container.ApplyFixPromptGenerator,
				container.ApplyFixApplier,
			)

			stop()

			if err != nil {
				return fmt.Errorf("us apply command failed: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().Bool("fix", false,
		"Enable interactive fix mode to merge scenarios into the registry")

	return cmd
}
