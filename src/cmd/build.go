package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/ondatra-ai/true-bdd/src/internal/app/bootstrap"
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

// buildRunE is the run shape both `build` subcommands use after sourcing
// their lazily-built container, resolved spec path, and fix flag.
type buildRunE func(ctx context.Context, container *bootstrap.Container, specFile string, fix bool) error

// specFlag describes the spec-path override flag a `build` subcommand
// exposes, and the `documents.*` key that decides the path when the
// flag is not passed.
type specFlag struct {
	name   string
	usage  string
	docKey string
	// label names the document in the resolve error, e.g.
	// "scenario registry".
	label string
}

// buildSpecCmd builds the cobra shell shared by every `build`
// subcommand: a spec-path flag whose empty default defers to the host
// config, plus --fix. The container is resolved from the provider at
// RunE time (never at construction), so building the command tree
// touches no host config.
func buildSpecCmd(use, short, long string, flag specFlag, provide containerProvider, run buildRunE) *cobra.Command {
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Long:  long,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			container, err := provide()
			if err != nil {
				return fmt.Errorf("initialize container: %w", err)
			}

			ctx, stop := signal.NotifyContext(context.Background(),
				os.Interrupt, syscall.SIGTERM)
			defer stop()

			specFile, _ := cmd.Flags().GetString(flag.name)
			// Flags().Changed is the only reliable signal the user passed
			// the flag; an unset flag means the config decides.
			if !cmd.Flags().Changed(flag.name) {
				specFile, err = container.DocResolver.Resolve(flag.docKey)
				if err != nil {
					return fmt.Errorf("resolve %s: %w", flag.label, err)
				}
			}

			fix, _ := cmd.Flags().GetBool("fix")

			err = run(ctx, container, specFile, fix)

			stop()

			if err != nil {
				return fmt.Errorf("build %s: %w", use, err)
			}

			return nil
		},
	}

	cmd.Flags().String(flag.name, "", flag.usage)
	cmd.Flags().Bool("fix", false, fixFlagDescription)

	return cmd
}

func newBuildTestsCmd(provide containerProvider) *cobra.Command {
	return buildSpecCmd(
		"tests",
		"Walk the requirements registry and check every scenario has an executable test",
		`Walk every scenario in the configured scenario registry
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
		specFlag{
			name: "requirements",
			usage: "Path to the requirements registry YAML " +
				"(default: documents.scenarios_yaml from true-bdd/true-bdd.yaml)",
			docKey: docs.KeyScenariosYAML,
			label:  "scenario registry",
		},
		provide,
		func(ctx context.Context, container *bootstrap.Container, requirementsFile string, fix bool) error {
			return commands.RunBuildTests(ctx, commands.BuildTestsDeps{
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
		},
	)
}

func newBuildCodeCmd(provide containerProvider) *cobra.Command {
	return buildSpecCmd(
		"code",
		"Discover failing tests via architecture.yaml and (optionally) drive Claude to fix the production code",
		`Walk every (service, layer) pair declared in the configured
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
		specFlag{
			name: "architecture",
			usage: "Path to the architecture.yaml file driving the test scope " +
				"(default: documents.architecture_yaml from true-bdd/true-bdd.yaml)",
			docKey: docs.KeyArchitectureYAML,
			label:  "architectural spec",
		},
		provide,
		func(ctx context.Context, container *bootstrap.Container, architectureFile string, fix bool) error {
			return commands.RunBuildCode(ctx, commands.BuildCodeDeps{
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
		},
	)
}
