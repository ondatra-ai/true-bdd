package runner

import (
	"errors"
	"io"
	"os"
	"strings"
	"testing"
)

// errLoadItems is the error `us apply 95.4` actually produces, copied
// verbatim from a run rather than paraphrased. The wrapper prefixes, the
// [category:CODE] tag and the trailing sentinel are all part of the
// shape refuseStartup has to print, so a stand-in without them would let
// the real text drift while this test stayed green.
var errLoadItems = errors.New(
	"failed to load items: failed to parse story scenarios: " +
		"[infrastructure:STORY_FILE_NOT_FOUND] no story file found for story 95.4 " +
		"in docs/product/stories (expected format: 95.4-<slug>.yaml): story file not found",
)

// The refusal must return the caller's error untouched. Every call site
// wraps before calling, and callers upstream match on the sentinels
// underneath — so a helper that re-wrapped, replaced or flattened the
// error would silently break `errors.Is` and change the text cobra
// prints, when the only thing it exists to change is the reporting.
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

	// The console line is the only channel a BDD fixture can assert:
	// stdout_regex matches stdout alone, and cobra prints the returned
	// error to stderr under a usage dump, which neither the harness nor
	// the judge ever reads.
	if !strings.Contains(stdout, "Cannot start: "+errLoadItems.Error()) {
		t.Errorf("stdout missing the refusal line, got: %q", stdout)
	}
}

// An error already diagnosed by the code that produced it must not be
// announced a second time. `build code` prints `Cannot run <svc>/<layer>:`
// from inside LoadItems and then returns; without this suppression the
// same failure appears twice, the second time headlined "Refusing to
// start" after the run had demonstrably started.
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

// captureRunnerStdout runs body with os.Stdout redirected to a pipe and
// returns what it wrote.
//
// The read side runs concurrently on purpose. A pipe holds ~64KB before
// it blocks, and these refusals are joined error chains that only grow —
// draining after body() returned would deadlock inside console.Println
// the first time one crossed the buffer, and the test would hang until
// the package timeout rather than fail.
//
// Deliberately not parallel: it swaps a process global, and Go holds
// t.Parallel() tests until the sequential ones finish, so keeping this
// sequential is what makes the swap safe.
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
