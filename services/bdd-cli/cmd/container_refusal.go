package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/bootstrap"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/runner"
)

// containerFor builds the container a command runs on, refusing to start
// when it cannot: cobra alone prints the error to stderr, which no step reads.
func containerFor(cmd *cobra.Command, provide containerProvider) (*bootstrap.Container, error) {
	overrides, _ := cmd.Flags().GetStringArray("model")

	container, err := provide(overrides...)
	if err != nil {
		return nil, runner.RefuseStartup(commandName(cmd), fmt.Errorf("initialize container: %w", err))
	}

	yes, _ := cmd.Flags().GetBool("yes")
	if container.UserInputCollector != nil {
		container.UserInputCollector.SetAssumeYes(yes)
	}

	return container, nil
}

// commandName is the command as a person types it after the binary's own
// name: `us refine`, `scen check`.
func commandName(cmd *cobra.Command) string {
	return strings.TrimPrefix(cmd.CommandPath(), cmd.Root().Name()+" ")
}
