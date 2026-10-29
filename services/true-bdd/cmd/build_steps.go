package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/bootstrap"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/commands"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/docs"
)

func newBuildStepsCmd(provide containerProvider) *cobra.Command {
	cmd := buildSpecCmd("steps", "Write missing step definitions and check Step coverage",
		"List missing Catalogue phrase definitions; --fix writes bodies and verifies coverage and the test list.",
		specFlag{
			name:   "requirements",
			usage:  "Path to the Registry (default: documents.scenarios_yaml)",
			docKey: docs.KeyScenariosYAML, label: "scenario registry",
		},
		provide, func(
			ctx context.Context, cmd *cobra.Command, container *bootstrap.Container,
			registryFile string, fix bool,
		) error {
			architectureFile, err := resolveSpec(cmd, container, "architecture",
				docs.KeyArchitectureYAML, "architectural spec", "build steps")
			if err != nil {
				return err
			}

			return commands.RunBuildSteps(ctx, commands.BuildStepsDeps{
				TmpDir:    container.RunDir.GetTmpOutPath(),
				Evaluator: container.BuildStepsEvaluator,
 FixGenerator: container.BuildStepsFixPromptGenerator,
 FixApplier: container.BuildStepsFixApplier,
				UserInputCollector: container.UserInputCollector, TableRenderer: container.TableRenderer,
				RegistryLoader:  container.RegistryLoader,
				ChecklistLoader: container.ChecklistLoader,
				DocResolver:     container.DocResolver,
			}, registryFile, architectureFile, fix)
		})
	cmd.Flags().String("architecture", "", "Path to the architectural spec (default: documents.architecture_yaml)")

	return cmd
}
