package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/bootstrap"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/commands"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/docs"
)

func newBuildStepsCmd(provide containerProvider) *cobra.Command {
	cmd := buildSpecCmd("steps", "List missing step definitions and check Step coverage",
		"List missing Catalogue phrase definitions and check Step coverage. --fix is not implemented yet.",
		specFlag{
			name:   "requirements",
			usage:  "Path to the Registry (default: documents.scenarios_yaml)",
			docKey: docs.KeyScenariosYAML, label: "scenario registry",
		},
		provide, func(
			ctx context.Context, cmd *cobra.Command, container *bootstrap.Container,
			registryFile string, fix bool,
		) error {
			if fix {
				return commands.RunBuildSteps(ctx, commands.BuildStepsDeps{}, "", "", true)
			}

			architectureFile, err := resolveSpec(cmd, container, "architecture",
				docs.KeyArchitectureYAML, "architectural spec", "build steps")
			if err != nil {
				return err
			}

			return commands.RunBuildSteps(ctx, commands.BuildStepsDeps{
				TmpDir:          container.RunDir.GetTmpOutPath(),
				RegistryLoader:  container.RegistryLoader,
				ChecklistLoader: container.ChecklistLoader,
				DocResolver:     container.DocResolver,
			}, registryFile, architectureFile, false)
		})
	cmd.Flags().String("architecture", "", "Path to the architectural spec (default: documents.architecture_yaml)")

	return cmd
}
