package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// A mistyped argument is the one failure on these commands that IS an
// invocation error, so it must still reach the user with the usage text
// SilenceUsage otherwise suppresses.
func TestArgsWithUsageCarriesUsageOnRejection(t *testing.T) {
	t.Parallel()

	cmd := &cobra.Command{Use: "code", Short: "build code"}
	cmd.Flags().Bool("fix", false, "enable fix mode")

	err := argsWithUsage(cobra.NoArgs)(cmd, []string{"extra-arg"})
	if err == nil {
		t.Fatal("an unexpected positional argument must be rejected")
	}

	for _, want := range []string{"extra-arg", "Usage:", "--fix"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not carry %q", err, want)
		}
	}
}

// The wrapper must stay invisible on the path every real run takes.
func TestArgsWithUsagePassesValidInvocation(t *testing.T) {
	t.Parallel()

	cmd := &cobra.Command{Use: "code"}

	err := argsWithUsage(cobra.NoArgs)(cmd, nil)
	if err != nil {
		t.Fatalf("no positional arguments is the valid case: %v", err)
	}
}
