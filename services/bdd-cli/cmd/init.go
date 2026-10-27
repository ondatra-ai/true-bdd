package cmd

import (
	"log/slog"

 "github.com/ondatra-ai/true-bdd/pkg/logging"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/initialization"
	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "init",
		Short:        "Create missing configuration, checklists and prompt templates",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			logging.Install(logging.Stdout, "tmp/true-bdd.log.json", "")

 files, err := initialization.WriteMissing()
			for _, file := range files {
				action := "Left existing"
				if file.Created {
					action = "Created"
				}

				slog.Info(action, "path", file.Path)
			}

			return err
		},
	}
}
