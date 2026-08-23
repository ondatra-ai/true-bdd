package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/remote"
)

const defaultServerURL = "http://127.0.0.1:4517"

// resolveServerURL resolves the harness server URL: --server flag, then
// TRUE_BDD_SERVER env, then loopback. Flags().Changed is required: Cobra
// installs the default INTO the flag, so a flag left at default can't otherwise be told from "not passed".
func resolveServerURL(cmd *cobra.Command, flagValue string) string {
	var raw string
	if cmd.Flags().Changed("server") {
		raw = flagValue
	} else if env := os.Getenv("TRUE_BDD_SERVER"); env != "" {
		raw = env
	} else {
		raw = flagValue
	}

	return normalizeServerURL(raw)
}

// normalizeServerURL trims trailing slashes. It deliberately does not
// reject malformed values — the remote surfaces a deterministic
// request-time error against the bad URL instead.
func normalizeServerURL(raw string) string {
	return strings.TrimRight(raw, "/")
}

// newRemoteCmd builds the `remote` subcommand — the host-folder agent
// that connects OUT to the harness server (plan §3.1). It constructs no
// bootstrap container, so it runs honestly in a bare folder.
func newRemoteCmd() *cobra.Command {
	var serverURL string

	cmd := &cobra.Command{
		Use:   "remote",
		Short: "Run as a host-folder agent connected to the harness server",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, stop := signal.NotifyContext(context.Background(),
				os.Interrupt, syscall.SIGTERM)
			defer stop()

			resolved := resolveServerURL(cmd, serverURL)
			err := remote.Run(ctx, remote.Options{ServerURL: resolved, Version: Version})

			stop()

			if err != nil {
				return fmt.Errorf("remote: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&serverURL, "server", defaultServerURL,
		"Harness server URL the remote connects OUT to (env: TRUE_BDD_SERVER)")

	return cmd
}
