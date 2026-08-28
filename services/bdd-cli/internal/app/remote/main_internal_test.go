package remote

import (
	"context"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/console"
	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

// Environment keys the re-exec helper child reads: the process-supervision
// tests spawn THIS test binary as an engineered child (no Claude), driving
// flock-inheritance, SIGINT-EOF, TERM→KILL escalation, and nested-descendant-cleanup paths deterministically.
const (
	testChildModeEnv     = "TRUE_BDD_REMOTE_TEST_CHILD_MODE"
	testGrandchildPidEnv = "TRUE_BDD_REMOTE_TEST_GC_PID_FILE"
)

// Helper child modes.
const (
	modeBlockStdin      = "block-stdin"
	modeIgnoreSignals   = "ignore-signals"
	modeSpawnGrandchild = "spawn-grandchild"
)

// TestMain intercepts a re-exec into helper-child mode before running the
// package's tests.
func TestMain(m *testing.M) {
	mode := os.Getenv(testChildModeEnv)
	if mode != "" {
		runTestChild(mode)

		return
	}

	os.Exit(m.Run())
}

// runTestChild runs the engineered child behavior for a mode and exits.
func runTestChild(mode string) {
	switch mode {
	case modeBlockStdin:
		blockStdinChild()
	case modeIgnoreSignals:
		ignoreSignalsChild()
	case modeSpawnGrandchild:
		spawnGrandchildChild()
	default:
		os.Exit(2)
	}

	os.Exit(0)
}

// testChildReady is the stdout marker a helper child writes AFTER its
// signal handlers are installed. The parent waits for it before signalling,
// closing the startup race where a SIGINT could arrive before signal.Ignore.
const testChildReady = "READY"

// emitReady tells the parent the child's signal handlers are in place.
func emitReady() {
	_, _ = io.WriteString(console.Out(), testChildReady+"\n")
}

// blockStdinChild ignores SIGINT and blocks reading stdin until EOF, then
// exits cleanly — modelling the collector's "stdin close ⇒ Exit" path.
func blockStdinChild() {
	signal.Ignore(syscall.SIGINT)
	emitReady()

	_, _ = io.Copy(io.Discard, console.In())
}

// ignoreSignalsChild ignores SIGINT and SIGTERM and blocks (on a timer, so
// the runtime's deadlock detector stays quiet) until SIGKILL — the
// forced-escalation target; it never reads stdin, so a stdin close cannot end it either.
func ignoreSignalsChild() {
	signal.Ignore(syscall.SIGINT, syscall.SIGTERM)
	emitReady()

	for {
		time.Sleep(time.Hour)
	}
}

// spawnGrandchildChild starts a long sleeper in the same process group,
// records its pid, then blocks on stdin — the nested-descendant target.
func spawnGrandchildChild() {
	grandchild := exec.CommandContext(context.Background(), "sleep", "300")

	err := grandchild.Start()
	if err != nil {
		os.Exit(3)
	}

	pidFile := os.Getenv(testGrandchildPidEnv)
	_ = disk.Write(pidFile, []byte(strconv.Itoa(grandchild.Process.Pid)), disk.Shared)

	signal.Ignore(syscall.SIGINT)
	emitReady()

	_, _ = io.Copy(io.Discard, console.In())
}
