package cmd

import (
	"os"
	"sync"

	"github.com/spf13/cobra"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/bootstrap"
)

// containerProvider lazily builds the bootstrap container on first use.
// `version` never calls it, so it runs in a bare folder; `us`, `build` and
// `scen` resolve it in RunE.
type containerProvider func(...string) (*bootstrap.Container, error)

// lazyContainer returns a provider that constructs the container at most
// once, memoizing the result (and any error).
func lazyContainer() containerProvider {
	var (
		once      sync.Once
		container *bootstrap.Container
		buildErr  error
	)

	return func(overrides ...string) (*bootstrap.Container, error) {
		once.Do(func() {
			container, buildErr = bootstrap.NewContainer(overrides...)
		})

		return container, buildErr
	}
}

// Execute builds the CLI command tree and runs it.
func Execute() {
	provide := lazyContainer()

	rootCmd := &cobra.Command{
		Use:   "true-bdd",
		Short: "TrueBDD CLI",
	}

	rootCmd.AddCommand(newVersionCmd())

	rootCmd.AddCommand(newClaudeGuardCmd())
	rootCmd.AddCommand(NewUSCommand(provide))
	rootCmd.AddCommand(NewBuildCommand(provide))
	rootCmd.AddCommand(NewScenCommand(provide))

	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
