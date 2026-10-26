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

// NewScenCommand builds the `scen` cobra supergroup. The container is
// resolved lazily inside each subcommand's RunE.
func NewScenCommand(provide containerProvider) *cobra.Command {
	scenCmd := &cobra.Command{
		Use:   "scen",
		Short: "Scenario registry commands",
	}

	scenCmd.AddCommand(newScenCheckCmd(provide))

	return scenCmd
}

func newScenCheckCmd(provide containerProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check [scenario-id...]",
		Short: "Walk registry scenarios against the scen-check checklist",
		Long: `Walk entries in the configured scenario registry
(documents.scenarios_yaml, conventionally docs/scenarios.yaml) against
the scen-check checklist, one cell per (scenario, prompt). Each prompt
reads one scenario's own fields — description, service, path, lineage
and steps — and rules on that scenario alone; no prompt is shown the
registry file, so none can ask a cross-registry question. A Q: prompt
costs one AI turn per scenario; a D: prompt is answered by one of the
engine's built-in checks with no turn, before any Q: prompt, and a
scenario failing one skips its Q: prompts.

With no argument every scenario is walked; ids narrow the walk to those
scenarios, in ascending id order whatever order they are typed. An id
naming no entry, or given twice, is refused before the first turn.

The CLI exits 1 when any scenario fails a check, so scen check can gate
a change. --fix walks the same checklist, every shipped prompt of which
carries a fix template, and is refused at startup while a prompt it
would walk carries none. A failed D: check's fix defines what it found
undefined in the product document, or maps it in the architecture
document, never in the scenario; a failed Q: prompt's fix rewrites the
scenario's own registry entry, or the meaning of a catalogue phrase.

Example:
  true-bdd scen check
  true-bdd scen check scen-check-001-claimed-scenario-passes scen-check-003-unknown-id-refused
  true-bdd scen check --requirements docs/scenarios.yaml`,
		Args: argsWithUsage(cobra.ArbitraryArgs),
		// A run failure shouldn't dump help text like a mistyped flag
		// would — argsWithUsage restores usage for that one case.
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runScenCheck(cmd, args, provide)
		},
	}

	cmd.Flags().String("requirements", "",
		"Path to the requirements registry YAML "+
			"(default: documents.scenarios_yaml from true-bdd/true-bdd.yaml)")
	cmd.Flags().Bool("fix", false, fixFlagDescription)
	cmd.Flags().Bool("yes", false, yesFlagDescription)
	cmd.Flags().StringArray("model", nil,
		"Override a configured model tier for this run: <tier>=<cli>:<model> (repeatable)")

	return cmd
}

// runScenCheck is newScenCheckCmd's RunE body, lifted out so the cobra
// shell above stays a declaration.
func runScenCheck(cmd *cobra.Command, args []string, provide containerProvider) error {
	container, err := containerFor(cmd, provide)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(),
		os.Interrupt, syscall.SIGTERM)
	defer stop()

	requirementsFile, err := resolveSpec(cmd, container, "requirements",
		docs.KeyScenariosYAML, "scenario registry", "scen check")
	if err != nil {
		return err
	}

	fix, _ := cmd.Flags().GetBool("fix")

	err = runScenCheckWithContainer(ctx, container, requirementsFile, args, fix)

	stop()

	if err != nil {
		return fmt.Errorf("scen check: %w", err)
	}

	return nil
}

// runScenCheckWithContainer projects the container onto ScenCheckDeps, so
// the command depends on only what it uses.
func runScenCheckWithContainer(
	ctx context.Context,
	container *bootstrap.Container,
	requirementsFile string,
	ids []string,
	fix bool,
) error {
	return commands.RunScenCheck(ctx, commands.ScenCheckDeps{
		RegistryLoader:              container.RegistryLoader,
		ChecklistLoader:             container.ChecklistLoader,
		DocResolver:                 container.DocResolver,
		ScenCheckEvaluator:          container.ScenCheckEvaluator,
		ScenCheckFixPromptGenerator: container.ScenCheckFixPromptGenerator,
		ScenCheckFixApplier:         container.ScenCheckFixApplier,
		UserInputCollector:          container.UserInputCollector,
		TableRenderer:               container.TableRenderer,
		RunDir:                      container.RunDir,
	}, requirementsFile, ids, fix)
}
