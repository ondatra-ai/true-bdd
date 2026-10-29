package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/bootstrap"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/commands"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/docs"
)

// NewBuildCommand builds the `build` cobra supergroup. The container is
// resolved lazily inside each subcommand's RunE.
func NewBuildCommand(provide containerProvider) *cobra.Command {
	buildCmd := &cobra.Command{
		Use:   "build",
		Short: "Build commands",
	}

	buildCmd.AddCommand(newBuildTestsCmd(provide))
	buildCmd.AddCommand(newBuildStepsCmd(provide))
	buildCmd.AddCommand(newBuildCodeCmd(provide))

	return buildCmd
}

// buildRunE is the run shape all `build` subcommands share, receiving the
// resolved container, spec path, and fix flag — plus the cobra command,
// since Registry commands need a second document and must check their flags.
type buildRunE func(
	ctx context.Context,
	cmd *cobra.Command,
	container *bootstrap.Container,
	specFile string,
	fix bool,
) error

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

// buildSpecCmd builds the cobra shell every `build` subcommand shares: a
// spec-path flag deferring to host config, plus --fix. The container is
// resolved at RunE time only (see containerProvider).
func buildSpecCmd(
	use, short, long, fixUsage string, flag specFlag, provide containerProvider, run buildRunE,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Long:  long,
		Args:  argsWithUsage(cobra.NoArgs),
		// A run failure (bad doc, bad spec, non-converging walk) shouldn't
		// dump help text like a mistyped flag would — so usage is silenced
		// globally and argsWithUsage restores it only for that one case.
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			container, err := containerFor(cmd, provide)
			if err != nil {
				return err
			}

			ctx, stop := signal.NotifyContext(context.Background(),
				os.Interrupt, syscall.SIGTERM)
			defer stop()

			specFile, err := resolveSpec(cmd, container, flag.name, flag.docKey, flag.label, "build "+use)
			if err != nil {
				return err
			}

			fix, _ := cmd.Flags().GetBool("fix")

			err = run(ctx, cmd, container, specFile, fix)

			stop()

			if err != nil {
				return fmt.Errorf("build %s: %w", use, err)
			}

			return nil
		},
	}

	cmd.Flags().String(flag.name, "", flag.usage)
	cmd.Flags().Bool("fix", false, fixUsage)

	return cmd
}

// addModelFlag gives a command that dispatches AI turns its run-only model
// tier overrides; build tests dispatches none and takes no --model.
func addModelFlag(cmd *cobra.Command) {
	cmd.Flags().StringArray("model", nil,
		"Override a configured model tier for this run: <tier>=<cli>:<model> (repeatable)")
}

func newBuildTestsCmd(provide containerProvider) *cobra.Command {
	cmd := buildSpecCmd(
		"tests",
		"Render or verify Registry test files",
		`Render tests deterministically from the Registry. Without --fix, refuse stale
files; with --fix, write them. No AI turn or test runner is started.`,
		"Write stale generated test files instead of refusing them",
		specFlag{
			name: "requirements",
			usage: "Path to the requirements registry YAML " +
				"(default: documents.scenarios_yaml from true-bdd/true-bdd.yaml)",
			docKey: docs.KeyScenariosYAML,
			label:  "scenario registry",
		},
		provide,
		func(
			ctx context.Context,
			cmd *cobra.Command,
			container *bootstrap.Container,
			requirementsFile string,
			fix bool,
		) error {
			// Resolved after the registry, deliberately: a host that
			// declared neither document must hear about the registry
			// first, since that is the one `build tests` is named for.
			architectureFile, err := resolveSpec(cmd, container, "architecture",
				docs.KeyArchitectureYAML, "architectural spec", "build tests")
			if err != nil {
				return err
			}

			return commands.RunBuildTests(ctx, commands.BuildTestsDeps{
				RegistryLoader: container.RegistryLoader,
				DocResolver:    container.DocResolver,
			}, requirementsFile, architectureFile, fix)
		},
	)

	cmd.Flags().String("architecture", "",
		"Path to the architectural spec naming the test suites "+
			"(default: documents.architecture_yaml from true-bdd/true-bdd.yaml)")

	return cmd
}

// resolveSpec reads a document-path flag, falling back to the host
// config when the user did not pass it. Flags().Changed is the only
// reliable signal: an unset flag means the config decides.
func resolveSpec(
	cmd *cobra.Command,
	container *bootstrap.Container,
	flagName, docKey, label, command string,
) (string, error) {
	value, _ := cmd.Flags().GetString(flagName)
	if cmd.Flags().Changed(flagName) {
		return value, nil
	}

	resolved, err := container.DocResolver.Resolve(docKey)
	if err != nil {
		return "", refuseUnresolvedDoc(command, label, err)
	}

	return resolved, nil
}

func newBuildCodeCmd(provide containerProvider) *cobra.Command {
	cmd := buildSpecCmd(
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
		fixFlagDescription,
		specFlag{
			name: "architecture",
			usage: "Path to the architecture.yaml file driving the test scope " +
				"(default: documents.architecture_yaml from true-bdd/true-bdd.yaml)",
			docKey: docs.KeyArchitectureYAML,
			label:  "architectural spec",
		},
		provide,
		func(
			ctx context.Context,
			_ *cobra.Command,
			container *bootstrap.Container,
			architectureFile string,
			fix bool,
		) error {
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
	addModelFlag(cmd)

	return cmd
}
