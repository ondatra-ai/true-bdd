package cmd

import (
	"log"
	"os"

	"github.com/ondatra-ai/true-bdd/src/internal/app/bootstrap"
	"github.com/spf13/cobra"
)

func Execute() {
	container, err := bootstrap.NewContainer()
	if err != nil {
		log.Fatalf("Failed to initialize container: %v", err)
	}

	rootCmd := &cobra.Command{
		Use:   "true-bdd",
		Short: "TrueBDD CLI",
	}

	rootCmd.AddCommand(NewUSCommand(container))
	rootCmd.AddCommand(NewBuildCommand(container))

	err = rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
