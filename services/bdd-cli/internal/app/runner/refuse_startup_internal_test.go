package runner

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/logging"
)

// errLoadItems is the error `us apply 95.4` actually produces, copied
// verbatim (wrapper prefixes, [category:CODE] tag, trailing sentinel
// and all) so a paraphrase can't quietly drift from the real shape.
var errLoadItems = errors.New(
	"failed to load items: failed to parse story scenarios: " +
		"[infrastructure:STORY_FILE_NOT_FOUND] no story file found for story 95.4 " +
		"in docs/product/stories (expected format: 95.4-<slug>.yaml): story file not found",
)

// refuseStartup must return the cause untouched: callers match on
// sentinels via errors.Is, and cobra prints exactly this text — a
// helper that re-wrapped or flattened it would break both silently.
func TestRefuseStartupReturnsTheCauseUnchanged(t *testing.T) {
	stdout := captureRunnerStdout(t, func() {
		got := refuseStartup("us apply", errLoadItems)

		if !errors.Is(got, errLoadItems) {
			t.Errorf("returned error does not wrap the cause: %v", got)
		}

		if got.Error() != errLoadItems.Error() {
			t.Errorf("error text\n got: %q\nwant: %q", got.Error(), errLoadItems.Error())
		}
	})

	// Only stdout is assertable here — see refuseStartup's doc for why.
	if !strings.Contains(stdout, `msg="Refusing to start"`) ||
		!strings.Contains(stdout, "no story file found for story 95.4") {
		t.Errorf("stdout missing the refusal line, got: %q", stdout)
	}
}

// Proves the Reported-marker contract from helpers.go: an
// already-diagnosed error must not be announced a second time.
func TestRefuseStartupStaysSilentForAnAlreadyReportedError(t *testing.T) {
	cause := errors.New("discover calc/e2e: executable file not found in $PATH")

	stdout := captureRunnerStdout(t, func() {
		got := refuseStartup("build code", Reported(cause))

		// Suppressing the report must not suppress the error: the exit
		// code and every errors.Is match upstream still depend on it.
		if !errors.Is(got, cause) {
			t.Errorf("marker swallowed the cause: %v", got)
		}

		if got.Error() != cause.Error() {
			t.Errorf("marker changed the message\n got: %q\nwant: %q", got.Error(), cause.Error())
		}
	})

	if stdout != "" {
		t.Errorf("expected no second report for an already-reported error, got: %q", stdout)
	}
}

// captureRunnerStdout points the process's slog at a buffer — the seam
// pkg/logging exists to provide, and why this never touches os.Stdout.
func captureRunnerStdout(t *testing.T, body func()) string {
	t.Helper()

	var captured bytes.Buffer

	original := slog.Default()

	slog.SetDefault(slog.New(logging.Handler(&captured, "")))

	// Restore even when body calls t.Fatalf, which unwinds via
	// runtime.Goexit: that skips statements but still runs defers.
	defer slog.SetDefault(original)

	body()

	return captured.String()
}
