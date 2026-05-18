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
)

const (
	defaultRequirementsFile = "docs/requirements.yaml"
	fixFlagDescription      = "Enable interactive fix mode to resolve failed checks"
)

// runWithFix is the run function shape every `us` subcommand uses
// after sourcing its fix flag and a story-number arg.
type runWithFix func(ctx context.Context, storyNumber string, fix bool) error

// NewUSCommand builds the `us` cobra supergroup.
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

// buildStoryCmd builds the cobra shell shared by every `us`
// subcommand that takes a story number and an optional --fix flag.
func buildStoryCmd(use, short, long string, run runWithFix) *cobra.Command {
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

			err := run(ctx, args[0], fix)

			stop()

			if err != nil {
				return fmt.Errorf("%s command failed: %w", use, err)
			}

			return nil
		},
	}

	cmd.Flags().Bool("fix", false, fixFlagDescription)

	return cmd
}

func newUSCreateCmd(container *bootstrap.Container) *cobra.Command {
	return buildStoryCmd(
		"create [story-number]",
		"Create and validate a user story",
		`Extract a story from its epic and validate it against the us-create
checklist. The story is saved to docs/stories/ upon passing all checks.

Example:
  bdd-cli us create 4.1
  bdd-cli us create 4.1 --fix`,
		func(ctx context.Context, storyNumber string, fix bool) error {
			return commands.RunCreate(ctx, commands.CreateDeps{
				StoryCommonDeps: storyCommonFromContainer(container),
				EpicLoader:      container.EpicLoader,
			}, storyNumber, fix)
		},
	)
}

func newUSRefineCmd(container *bootstrap.Container) *cobra.Command {
	return buildStoryCmd(
		"refine [story-number]",
		"Refine a user story",
		`Load a story from docs/stories/ and validate it against the us-refine
checklist. The story file is updated in place upon passing all checks.

Example:
  bdd-cli us refine 4.1
  bdd-cli us refine 4.1 --fix`,
		func(ctx context.Context, storyNumber string, fix bool) error {
			return commands.RunRefine(ctx, commands.RefineDeps{
				StoryCommonDeps: storyCommonFromContainer(container),
				StoryLoader:     container.StoryLoader,
			}, storyNumber, fix)
		},
	)
}

// storyCommonFromContainer projects the bootstrap container into the
// fields shared by us create and us refine. Keeps the per-command
// cobra handlers tiny.
func storyCommonFromContainer(container *bootstrap.Container) commands.StoryCommonDeps {
	return commands.StoryCommonDeps{
		ChecklistLoader:    container.ChecklistLoader,
		Evaluator:          container.Evaluator,
		FixGenerator:       container.FixGenerator,
		FixApplier:         container.FixApplier,
		UserInputCollector: container.UserInputCollector,
		TableRenderer:      container.TableRenderer,
		RunDir:             container.RunDir,
		StoriesDir:         container.StoriesDir,
	}
}

func newUSApplyCmd(container *bootstrap.Container) *cobra.Command {
	return buildStoryCmd(
		"apply [story-number]",
		"Apply scenarios from a refined user story into the registry",
		`Walk every acceptance criterion in docs/stories/<story-number>-*.yaml and
validate each one against the us-apply checklist. With --fix, every failed
(AC, prompt) cell drives a Claude-mediated edit on a scratch copy of
docs/requirements.yaml. The canonical registry file is replaced atomically
only when every AC passes every prompt; otherwise it is left untouched.

Stories that still use the deprecated scenarios.test_scenarios[] format are
rejected — convert them to acceptance_criteria with embedded steps first.

Example:
  bdd-cli us apply 4.1
  bdd-cli us apply 4.1 --fix`,
		func(ctx context.Context, storyNumber string, fix bool) error {
			return commands.RunApply(ctx, commands.ApplyDeps{
				StoryScenarioParser:     container.StoryScenarioParser,
				ChecklistLoader:         container.ChecklistLoader,
				ApplyEvaluator:          container.ApplyEvaluator,
				ApplyFixPromptGenerator: container.ApplyFixPromptGenerator,
				ApplyFixApplier:         container.ApplyFixApplier,
				UserInputCollector:      container.UserInputCollector,
				TableRenderer:           container.TableRenderer,
				RunDir:                  container.RunDir,
			}, storyNumber, defaultRequirementsFile, fix)
		},
	)
}
