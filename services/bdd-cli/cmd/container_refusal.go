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
	container, err := provide()
	if err != nil {
		command := strings.TrimPrefix(cmd.CommandPath(), cmd.Root().Name()+" ")

		return nil, runner.RefuseStartup(command, fmt.Errorf("initialize container: %w", err))
	}

	return container, nil
}
