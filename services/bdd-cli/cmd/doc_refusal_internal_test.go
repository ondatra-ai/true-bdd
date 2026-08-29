package cmd

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/logging"
)

// errResolve mimics ErrDocPathNotConfigured's real text — the config-key
// substring the build-tests-unconfigured-registry BDD fixture asserts on
// stdout, so changing it here breaks that fixture, not this test.
var errResolve = errors.New(
	`document path not configured: "scenarios_yaml" (config key documents.scenarios_yaml)`,
)

// TestRefuseUnresolvedDocPreservesTheErrorContract checks refuseUnresolvedDoc
// keeps the same wrapping and text the call sites had before it existed, so
// errors.Is still matches the resolver's sentinel errors.
func TestRefuseUnresolvedDocPreservesTheErrorContract(t *testing.T) {
	stdout := captureStdout(t, func() {
		got := refuseUnresolvedDoc("build tests", "scenario registry", errResolve)

		if !errors.Is(got, errResolve) {
			t.Fatalf("returned error does not wrap the cause: %v", got)
		}

		want := "resolve scenario registry: " + errResolve.Error()
		if got.Error() != want {
			t.Fatalf("error text\n got: %q\nwant: %q", got.Error(), want)
		}
	})

	// The log record, not the returned error, is what the BDD fixture checks:
	// the engine binds slog to stdout, and cobra's own error printing goes to
	// stderr, which the fixture never reads.
	if !strings.Contains(stdout, `msg="Refusing to start: document unresolvable"`) {
		t.Errorf("stdout missing the refusal line, got: %q", stdout)
	}

	if !strings.Contains(stdout, "documents.scenarios_yaml") {
		t.Errorf("stdout refusal does not name the config key, got: %q", stdout)
	}
}

// captureStdout points the process's slog at a buffer and returns what the
// refusal wrote. Never call t.Parallel() around it: swapping a process global
// races any parallel test that also logs while this one holds it swapped.
func captureStdout(t *testing.T, body func()) string {
	t.Helper()

	var captured bytes.Buffer

	original := slog.Default()

	slog.SetDefault(slog.New(logging.Handler(&captured, "")))

	defer slog.SetDefault(original)

	body()

	return captured.String()
}
