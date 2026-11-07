package main

import (
	"context"
	"errors"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/cli"
	"github.com/ondatra-ai/true-bdd/pkg/cli/spec"
)

// The engine waits for every CLI to exit by itself, so a signal means the
// recording was interrupted and must not become a cassette.
func TestWaitExitCodeRefusesASignalledCLI(t *testing.T) {
	t.Parallel()

	proc, err := spec.Start(context.Background(), []string{"sh", "-c", "kill -TERM $$"}, cli.Options{Output: cli.Pipe()})
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	_, err = waitExitCode(proc)
	if !errors.Is(err, errKilledWhileRecording) {
		t.Fatalf("waitExitCode error = %v, want %v", err, errKilledWhileRecording)
	}
}

func TestWaitExitCodeKeepsTheCLIsOwnCode(t *testing.T) {
	t.Parallel()

	proc, err := spec.Start(context.Background(), []string{"sh", "-c", "exit 3"}, cli.Options{Output: cli.Pipe()})
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	code, err := waitExitCode(proc)
	if err != nil || code != 3 {
		t.Fatalf("waitExitCode = %d, %v; want 3, nil", code, err)
	}
}
