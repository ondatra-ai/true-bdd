package cmd

import (
	"os"
	"sync"

	"github.com/spf13/cobra"

	"github.com/ondatra-ai/true-bdd/src/internal/app/bootstrap"
)

// containerProvider lazily builds the bootstrap container on first use.
// The `version` and `remote` subcommands never call it, so they run
// without a container — and therefore without a valid host config,
// honestly reporting a bare folder (plan §3.1). The `us` and `build`
// commands resolve it inside their RunE, preserving their exact prior
// behavior (the container is built the moment the command runs).
type containerProvider func() (*bootstrap.Container, error)

// lazyContainer returns a provider that constructs the container at most
// once, memoizing the result (and any error).
func lazyContainer() containerProvider {
	var (
		once      sync.Once
		container *bootstrap.Container
		buildErr  error
	)

	return func() (*bootstrap.Container, error) {
		once.Do(func() {
			container, buildErr = bootstrap.NewContainer()
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
	rootCmd.AddCommand(newRemoteCmd())
	rootCmd.AddCommand(NewUSCommand(provide))
	rootCmd.AddCommand(NewBuildCommand(provide))

	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
