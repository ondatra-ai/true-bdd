package cmd

import (
	"errors"
	"io"
	"os"
	"strings"
	"testing"
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

	// stdout, not the returned error, is what the BDD fixture checks —
	// cobra's default error printing goes to stderr, which the fixture
	// never reads.
	if !strings.Contains(stdout, "Cannot start: resolve scenario registry") {
		t.Errorf("stdout missing the refusal line, got: %q", stdout)
	}

	if !strings.Contains(stdout, "documents.scenarios_yaml") {
		t.Errorf("stdout refusal does not name the config key, got: %q", stdout)
	}
}

// captureStdout redirects os.Stdout to a pipe and returns what it wrote.
// Never call t.Parallel() around it: swapping a process global races any
// parallel test that also touches stdout while this one holds it swapped.
func captureStdout(t *testing.T, body func()) string {
	t.Helper()

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("open pipe: %v", err)
	}

	original := os.Stdout
	os.Stdout = writer

	defer func() { os.Stdout = original }()

	body()

	err = writer.Close()
	if err != nil {
		t.Fatalf("close pipe writer: %v", err)
	}

	captured, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read captured stdout: %v", err)
	}

	return string(captured)
}
