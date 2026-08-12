package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/ondatra-ai/true-bdd/src/internal/app/commands"
	"github.com/ondatra-ai/true-bdd/src/internal/infrastructure/docs"
)

// NewBuildCommand builds the `build` cobra supergroup. The container is
// resolved lazily inside each subcommand's RunE.
func NewBuildCommand(provide containerProvider) *cobra.Command {
	buildCmd := &cobra.Command{
		Use:   "build",
		Short: "Build commands",
	}

	buildCmd.AddCommand(newBuildTestsCmd(provide))
	buildCmd.AddCommand(newBuildCodeCmd(provide))

	return buildCmd
}

func newBuildTestsCmd(provide containerProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tests",
		Short: "Walk the requirements registry and check every scenario has an executable test",
		Long: `Walk every scenario in the configured scenario registry
(documents.scenarios_yaml, conventionally docs/scenarios.yaml) against the
build-tests checklist. The checklist asks whether each scenario id is
referenced by an executable test under tests/integration/, tests/e2e/,
services/backend/, or services/frontend/. With --fix, every failed
(scenario, prompt) cell drives a Claude-mediated test-authoring turn that
Writes or Edits a test file under the allowed roots; the registry is never
touched. The CLI exits non-zero if any scenario is still uncovered after
the walk.

Example:
  true-bdd build tests
  true-bdd build tests --fix
  true-bdd build tests --requirements docs/scenarios.yaml`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			container, err := provide()
			if err != nil {
				return fmt.Errorf("initialize container: %w", err)
			}

			ctx, stop := signal.NotifyContext(context.Background(),
				os.Interrupt, syscall.SIGTERM)
			defer stop()

			requirementsFile, _ := cmd.Flags().GetString("requirements")
			// Flags().Changed is the only reliable signal the user passed
			// the flag; an unset flag means the config decides.
			if !cmd.Flags().Changed("requirements") {
				requirementsFile, err = container.DocResolver.Resolve(docs.KeyScenariosYAML)
				if err != nil {
					return fmt.Errorf("resolve scenario registry: %w", err)
				}
			}

			fix, _ := cmd.Flags().GetBool("fix")

			err = commands.RunBuildTests(ctx, commands.BuildTestsDeps{
				RegistryLoader:               container.RegistryLoader,
				ChecklistLoader:              container.ChecklistLoader,
				DocResolver:                  container.DocResolver,
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

	cmd.Flags().String("requirements", "",
		"Path to the requirements registry YAML "+
			"(default: documents.scenarios_yaml from true-bdd/true-bdd.yaml)")
	cmd.Flags().Bool("fix", false, fixFlagDescription)

	return cmd
}

func newBuildCodeCmd(provide containerProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "code",
		Short: "Discover failing tests via architecture.yaml and (optionally) drive Claude to fix the production code",
		Long: `Walk every (service, layer) pair declared in the configured
architectural spec (documents.architecture_yaml, conventionally
docs/architecture/architecture.yaml), discover currently-failing tests
through their framework runner, and walk each failure against the
build-code checklist. With --fix, every failed cell drives a
Claude-mediated turn that Writes or Edits production source under services/* so
the failing test passes; test files and the scenario registry are never
touched. The CLI exits non-zero if any test is still failing after the walk.

Example:
  true-bdd build code
  true-bdd build code --fix
  true-bdd build code --architecture docs/architecture/architecture.yaml`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			container, err := provide()
			if err != nil {
				return fmt.Errorf("initialize container: %w", err)
			}

			ctx, stop := signal.NotifyContext(context.Background(),
				os.Interrupt, syscall.SIGTERM)
			defer stop()

			architectureFile, _ := cmd.Flags().GetString("architecture")
			// Flags().Changed is the only reliable signal the user passed
			// the flag; an unset flag means the config decides.
			if !cmd.Flags().Changed("architecture") {
				architectureFile, err = container.DocResolver.Resolve(docs.KeyArchitectureYAML)
				if err != nil {
					return fmt.Errorf("resolve architectural spec: %w", err)
				}
			}

			fix, _ := cmd.Flags().GetBool("fix")

			err = commands.RunBuildCode(ctx, commands.BuildCodeDeps{
				ArchitectureLoader:          container.ArchitectureLoader,
				TestRunnerDispatcher:        container.TestRunnerDispatcher,
				ChecklistLoader:             container.ChecklistLoader,
				DocResolver:                 container.DocResolver,
				BuildCodeEvaluator:          container.BuildCodeEvaluator,
				BuildCodeFixPromptGenerator: container.BuildCodeFixPromptGenerator,
				BuildCodeFixApplier:         container.BuildCodeFixApplier,
				UserInputCollector:          container.UserInputCollector,
				TableRenderer:               container.TableRenderer,
				RunDir:                      container.RunDir,
			}, architectureFile, fix)

			stop()

			if err != nil {
				return fmt.Errorf("build code: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().String("architecture", "",
		"Path to the architecture.yaml file driving the test scope "+
			"(default: documents.architecture_yaml from true-bdd/true-bdd.yaml)")
	cmd.Flags().Bool("fix", false, fixFlagDescription)

	return cmd
}
