package runner

import (
	"errors"
	"io"
	"os"
	"strings"
	"testing"
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
	if !strings.Contains(stdout, "Cannot start: "+errLoadItems.Error()) {
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

// captureRunnerStdout runs body with os.Stdout redirected to a pipe,
// returning what it wrote. It drains concurrently — deferring risks
// deadlock past the pipe's ~64KB buffer — and stays non-parallel since it swaps a process global.
func captureRunnerStdout(t *testing.T, body func()) string {
	t.Helper()

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("open pipe: %v", err)
	}

	captured := make(chan string, 1)

	go func() {
		content, readErr := io.ReadAll(reader)
		if readErr != nil {
			captured <- ""

			return
		}

		captured <- string(content)
	}()

	original := os.Stdout
	os.Stdout = writer

	// Restore, close and drain even when body calls t.Fatalf — that
	// unwinds via runtime.Goexit, which skips ordinary statements but
	// still runs deferred functions.
	defer func() {
		os.Stdout = original
		_ = writer.Close()
		_ = reader.Close()
	}()

	body()

	os.Stdout = original

	err = writer.Close()
	if err != nil {
		t.Fatalf("close pipe writer: %v", err)
	}

	return <-captured
}
