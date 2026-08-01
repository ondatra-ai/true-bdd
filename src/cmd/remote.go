package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/ondatra-ai/true-bdd/src/internal/app/remote"
)

const defaultServerURL = "http://127.0.0.1:4517"

// resolveServerURL picks the harness server URL the remote connects OUT to.
// Precedence (plan: connect-cli-to-vercel-harness → Codex r1 #13):
//   - an explicit `--server` flag wins;
//   - else the set-once `TRUE_BDD_SERVER` env var, if present and non-empty;
//   - else the loopback default.
//
// `Flags().Changed("server")` is the only reliable signal that the user passed
// `--server`: Cobra installs the loopback value as the flag DEFAULT, so a flag
// left at the default is ambiguous between "user chose loopback" and "user did
// not pass the flag at all".
//
// A trailing slash is trimmed so the relay client's `baseURL + path`
// concatenation never produces a double slash — HTTPS URLs from `vercel env`
// or copy-paste often carry one (plan r1 #13: HTTPS URL normalization).
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

// normalizeServerURL trims trailing slashes. It deliberately does NOT reject
// malformed values: the remote surfaces a deterministic request-time error
// against the bad URL (and a test pins that the function returns the raw value
// unchanged except for the trailing slash).
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
