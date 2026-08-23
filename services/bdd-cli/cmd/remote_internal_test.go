package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

const testEnvURL = "https://true-bdd-app.vercel.app"

// newRemoteCmdForTest builds an isolated `remote` cobra.Command whose --server
// flag mirrors the production default, so resolveServerURL can be exercised
// without spawning the agent. The command is NOT attached to rootCmd.
func newRemoteCmdForTest(t *testing.T) *cobra.Command {
	t.Helper()

	cmd := &cobra.Command{Use: "remote"}
	cmd.Flags().StringVar(new(string), "server", defaultServerURL, "")

	return cmd
}

// TestResolveServerURL covers every plan-mandated case (Codex r1 #5): unset
// env, env-only, explicit flag override, empty env, and HTTPS normalization.
// Subtests stay sequential since t.Setenv cannot run under t.Parallel.
func TestResolveServerURL(t *testing.T) {
	t.Run("unset env returns the loopback default", func(t *testing.T) {
		// Explicitly clear so a developer's exported TRUE_BDD_SERVER cannot
		// leak into the test (also t.Setenv-compatible).
		t.Setenv("TRUE_BDD_SERVER", "")

		cmd := newRemoteCmdForTest(t)
		flagValue, _ := cmd.Flags().GetString("server")

		got := resolveServerURL(cmd, flagValue)
		if got != defaultServerURL {
			t.Fatalf("unset env: got %q, want %q", got, defaultServerURL)
		}
	})

	t.Run("env-only is used when --server is at default", func(t *testing.T) {
		t.Setenv("TRUE_BDD_SERVER", testEnvURL)
		cmd := newRemoteCmdForTest(t)
		flagValue, _ := cmd.Flags().GetString("server")

		got := resolveServerURL(cmd, flagValue)
		if got != testEnvURL {
			t.Fatalf("env-only: got %q, want %q", got, testEnvURL)
		}
	})

	t.Run("explicit --server overrides env", func(t *testing.T) {
		t.Setenv("TRUE_BDD_SERVER", "https://env-only.example")

		const explicit = "https://flag-wins.example"

		cmd := newRemoteCmdForTest(t)

		err := cmd.Flags().Set("server", explicit)
		if err != nil {
			t.Fatalf("set --server: %v", err)
		}

		flagValue, _ := cmd.Flags().GetString("server")

		got := resolveServerURL(cmd, flagValue)
		if got != explicit {
			t.Fatalf("flag-overrides-env: got %q, want %q", got, explicit)
		}
	})

	t.Run("empty env falls back to the loopback default", func(t *testing.T) {
		t.Setenv("TRUE_BDD_SERVER", "")
		cmd := newRemoteCmdForTest(t)
		flagValue, _ := cmd.Flags().GetString("server")

		got := resolveServerURL(cmd, flagValue)
		if got != defaultServerURL {
			t.Fatalf("empty env: got %q, want %q", got, defaultServerURL)
		}
	})

	t.Run("invalid env is returned as-is (request-time error)", func(t *testing.T) {
		const bogus = "not-a-valid-url"
		t.Setenv("TRUE_BDD_SERVER", bogus)

		cmd := newRemoteCmdForTest(t)
		flagValue, _ := cmd.Flags().GetString("server")

		got := resolveServerURL(cmd, flagValue)
		if got != bogus {
			t.Fatalf("invalid env: got %q, want %q (validation deferred to the HTTP client)", got, bogus)
		}
	})
}

// TestNormalizeServerURL pins the trailing-slash trim (plan r1 #13: HTTPS URL
// normalization) so the relay client's `baseURL + path` concatenation never
// produces a double slash.
func TestNormalizeServerURL(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"https://true-bdd-app.vercel.app/": "https://true-bdd-app.vercel.app",
		"https://x.example/":               "https://x.example",
		"http://127.0.0.1:4517/":           defaultServerURL,
		"http://127.0.0.1:4517":            defaultServerURL,
		"https://y.example/a/b/":           "https://y.example/a/b",
	}

	for raw, want := range cases {
		t.Run(raw, func(t *testing.T) {
			t.Parallel()

			got := normalizeServerURL(raw)
			if got != want {
				t.Fatalf("normalize(%q) = %q, want %q", raw, got, want)
			}
		})
	}
}
