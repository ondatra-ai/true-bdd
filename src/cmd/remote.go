package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/ondatra-ai/true-bdd/src/internal/app/remote"
)

const defaultServerURL = "http://127.0.0.1:4517"

// newRemoteCmd builds the `remote` subcommand — the host-folder agent
// that connects OUT to the harness server (plan §3.1). It constructs no
// bootstrap container, so it runs honestly in a bare folder.
func newRemoteCmd() *cobra.Command {
	var serverURL string

	cmd := &cobra.Command{
		Use:   "remote",
		Short: "Run as a host-folder agent connected to the harness server",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			ctx, stop := signal.NotifyContext(context.Background(),
				os.Interrupt, syscall.SIGTERM)
			defer stop()

			err := remote.Run(ctx, remote.Options{ServerURL: serverURL, Version: Version})

			stop()

			if err != nil {
				return fmt.Errorf("remote: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&serverURL, "server", defaultServerURL,
		"Harness server URL the remote connects OUT to")

	return cmd
}
