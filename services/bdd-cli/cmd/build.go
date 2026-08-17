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
//
// It receives the cobra command as well, because `build tests` needs a
// SECOND document — it renders its test files from the registry and the
// architectural spec together — and only the command knows whether the
// user passed the flag or left the host config to decide.
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
		Args:  argsWithUsage(cobra.NoArgs),
		// Every failure these commands RETURN is a failure of the run,
		// not of the invocation: an unresolvable document, a spec that
		// will not load, a walk that did not converge. Cobra's default
		// is to print the full help text after any error, which reads
		// as "you typed the flags wrong" and buries the line that
		// actually says what happened.
		//
		// Its switch is all-or-nothing though, so silencing usage here
		// silences it for a mistyped argument too — the one case where
		// the help text IS the answer. argsWithUsage carries it back on
		// that path only.
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			container, err := provide()
			if err != nil {
				return fmt.Errorf("initialize container: %w", err)
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
	cmd.Flags().Bool("fix", false, fixFlagDescription)

	return cmd
}

func newBuildTestsCmd(provide containerProvider) *cobra.Command {
	cmd := buildSpecCmd(
		"tests",
		"Walk the requirements registry and check every scenario has an executable test",
		`Walk every scenario in the configured scenario registry
(documents.scenarios_yaml, conventionally docs/scenarios.yaml) against the
build-tests checklist. The checklist asks whether every step of the
scenario resolves to a step definition in the suite that owns it — the
entry under architecture.testing.suites[] whose service: matches the
scenario's service:. With --fix, every failed (scenario, prompt) cell
drives a Claude-mediated test-authoring turn that Writes or Edits a step
definition under that suite's path:; the registry is never touched. The
CLI exits non-zero if any scenario is still uncovered after the walk.

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
				RegistryLoader:               container.RegistryLoader,
				ChecklistLoader:              container.ChecklistLoader,
				DocResolver:                  container.DocResolver,
				BuildTestsEvaluator:          container.BuildTestsEvaluator,
				BuildTestsFixPromptGenerator: container.BuildTestsFixPromptGenerator,
				BuildTestsFixApplier:         container.BuildTestsFixApplier,
				UserInputCollector:           container.UserInputCollector,
				TableRenderer:                container.TableRenderer,
				RunDir:                       container.RunDir,
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
}
