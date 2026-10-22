package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/bootstrap"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/commands"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/docs"
)

const fixFlagDescription = "Enable interactive fix mode to resolve failed checks"

// storyRunE is the run shape every `us` subcommand uses after sourcing
// its lazily-built container, Story id arg, and fix flag.
type storyRunE func(ctx context.Context, container *bootstrap.Container, storyNumber string, fix bool) error

// NewUSCommand builds the `us` cobra supergroup. The container provider
// is resolved lazily inside each subcommand's RunE.
func NewUSCommand(provide containerProvider) *cobra.Command {
	usCmd := &cobra.Command{
		Use:   "us",
		Short: "User story commands",
	}

	usCmd.AddCommand(newUSRefineCmd(provide))
	usCmd.AddCommand(newUSApplyCmd(provide))

	return usCmd
}

// buildStoryCmd builds the cobra shell shared by every `us` subcommand that
// takes a Story id and an optional --fix flag. The container is
// resolved at RunE time only (see containerProvider), never at construction.
func buildStoryCmd(use, short, long string, provide containerProvider, run storyRunE) *cobra.Command {
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Long:  long,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			container, err := containerFor(cmd, provide)
			if err != nil {
				return err
			}

			ctx, stop := signal.NotifyContext(context.Background(),
				os.Interrupt, syscall.SIGTERM)
			defer stop()

			fix, _ := cmd.Flags().GetBool("fix")

			err = run(ctx, container, args[0], fix)

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

func newUSRefineCmd(provide containerProvider) *cobra.Command {
	return buildStoryCmd(
		"refine <story-id>",
		"Refine a user story",
		`Load the Story whose id is <story-id> from the one <story-id>-*.yaml file in
paths.stories_dir (conventionally docs/product/stories/) that declares that id,
and validate it against the us-refine checklist. The Story file is updated in
place upon passing all checks.

Example:
  true-bdd us refine scen-check
  true-bdd us refine scen-check --fix`,
		provide,
		func(ctx context.Context, container *bootstrap.Container, storyNumber string, fix bool) error {
			return commands.RunRefine(ctx, commands.RefineDeps{
				StoryCommonDeps: storyCommonFromContainer(container),
				StoryLoader:     container.StoryLoader,
			}, storyNumber, fix)
		},
	)
}

// storyCommonFromContainer projects the bootstrap container into the
// fields us refine shares with the story runner. Keeps the cobra
// handler tiny.
func storyCommonFromContainer(container *bootstrap.Container) commands.StoryCommonDeps {
	return commands.StoryCommonDeps{
		ChecklistLoader:    container.ChecklistLoader,
		DocResolver:        container.DocResolver,
		Evaluator:          container.Evaluator,
		FixGenerator:       container.FixGenerator,
		FixApplier:         container.FixApplier,
		UserInputCollector: container.UserInputCollector,
		TableRenderer:      container.TableRenderer,
		RunDir:             container.RunDir,
		StoriesDir:         container.StoriesDir,
	}
}

func newUSApplyCmd(provide containerProvider) *cobra.Command {
	return buildStoryCmd(
		"apply <story-id>",
		"Apply scenarios from a refined user story into the registry",
		`Walk every acceptance criterion of the Story whose id is <story-id> (the one
<story-id>-*.yaml file in paths.stories_dir that declares that id) and
validate each one against the us-apply checklist. With --fix, every failed
(AC, prompt) cell drives a Claude-mediated edit on a scratch copy of the
scenario registry configured at documents.scenarios_yaml (conventionally
docs/scenarios.yaml). The canonical registry file is replaced atomically
only when every AC passes every prompt; otherwise it is left untouched.

Stories that still use the deprecated scenarios.test_scenarios[] format are
rejected — convert them to acceptance_criteria with embedded steps first.

Example:
  true-bdd us apply scen-check
  true-bdd us apply scen-check --fix`,
		provide,
		func(ctx context.Context, container *bootstrap.Container, storyNumber string, fix bool) error {
			requirementsFile, err := container.DocResolver.Resolve(docs.KeyScenariosYAML)
			if err != nil {
				return refuseUnresolvedDoc("us apply", "scenario registry", err)
			}

			return commands.RunApply(ctx, commands.ApplyDeps{
				StoryScenarioParser:     container.StoryScenarioParser,
				ChecklistLoader:         container.ChecklistLoader,
				DocResolver:             container.DocResolver,
				ApplyEvaluator:          container.ApplyEvaluator,
				ApplyFixPromptGenerator: container.ApplyFixPromptGenerator,
				ApplyFixApplier:         container.ApplyFixApplier,
				UserInputCollector:      container.UserInputCollector,
				TableRenderer:           container.TableRenderer,
				RunDir:                  container.RunDir,
			}, storyNumber, requirementsFile, fix)
		},
	)
}
