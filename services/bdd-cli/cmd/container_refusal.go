package cmd

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/bootstrap"
)

// containerFor builds the container a command runs on, reporting a failure
// like runner.refuseStartup: cobra alone prints it to stderr, which no step reads.
func containerFor(cmd *cobra.Command, provide containerProvider) (*bootstrap.Container, error) {
	container, err := provide()
	if err != nil {
		slog.Error("Refusing to start",
			"command", strings.TrimPrefix(cmd.CommandPath(), cmd.Root().Name()+" "),
			"error", err,
		)

		return nil, fmt.Errorf("initialize container: %w", err)
	}

	return container, nil
}
