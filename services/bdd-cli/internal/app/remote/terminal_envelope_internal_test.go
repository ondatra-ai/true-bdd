package remote

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"syscall"
	"testing"
)

// exitErr runs `sh -c "exit N"` and returns the real *exec.ExitError so
// the envelope logic is exercised against a genuine syscall.WaitStatus.
func exitErr(t *testing.T, code int) error {
	t.Helper()

	cmd := exec.CommandContext(context.Background(), "/bin/sh", "-c", "exit "+strconv.Itoa(code))

	err := cmd.Run()
	if err == nil && code != 0 {
		t.Fatalf("expected non-nil error for exit %d", code)
	}

	if err == nil {
		return nil
	}

	return fmt.Errorf("exit %d: %w", code, err)
}

// signaledErr starts a sleeper and SIGKILLs it, yielding a Signaled()
// wait status.
func signaledErr(t *testing.T) error {
	t.Helper()

	cmd := exec.CommandContext(context.Background(), "/bin/sh", "-c", "sleep 30")

	err := cmd.Start()
	if err != nil {
		t.Fatalf("start sleeper: %v", err)
	}

	err = cmd.Process.Signal(syscall.SIGKILL)
	if err != nil {
		t.Fatalf("kill sleeper: %v", err)
	}

	// Wait reaps the signaled child; the returned error carries the
	// Signaled() wait status the envelope inspects.
	return fmt.Errorf("signaled child: %w", cmd.Wait())
}

// result builds a present childResult for the envelope table.
func result(outcome string, finalizationOK bool) childResult {
	return childResult{present: true, outcome: outcome, finalizationOK: finalizationOK}
}

func TestSynthesizeEnvelopePrecedence(t *testing.T) {
	t.Parallel()

	signaled := signaledErr(t)
	nonZero := exitErr(t, 3)

	cases := []struct {
		name        string
		waitErr     error
		result      childResult
		wantOutcome string
		wantDetail  string
	}{
		{"signal beats everything", signaled, result(outcomeConverged, true), outcomeInterrupted, ""},
		{"signal with no result still interrupted", signaled, childResult{}, outcomeInterrupted, ""},
		{"no result on clean exit", nil, childResult{}, outcomeError, detailNoResult},
		{"no result on non-zero exit", nonZero, childResult{}, outcomeError, detailNoResult},
		{"clean exit yields engine outcome", nil, result(outcomeConverged, true), outcomeConverged, ""},
		{"contradiction on non-zero success", nonZero, result(outcomeConverged, true), outcomeError, detailContradiction},
		{"non-zero + not_fixed is legitimate", nonZero, result(outcomeNotFixed, true), outcomeNotFixed, ""},
		{"non-zero + max_attempts legitimate", nonZero, result(outcomeMaxAttempts, true), outcomeMaxAttempts, ""},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			env := synthesizeEnvelope(testCase.waitErr, testCase.result)
			if env.outcome != testCase.wantOutcome || env.detail != testCase.wantDetail {
				t.Fatalf("envelope = {%q,%q}, want {%q,%q}",
					env.outcome, env.detail, testCase.wantOutcome, testCase.wantDetail)
			}
		})
	}
}

// intVal safely dereferences an *int for assertions.
func intVal(t *testing.T, ptr *int) int {
	t.Helper()

	if ptr == nil {
		t.Fatal("expected a non-nil *int")
	}

	return *ptr
}

// TestEnvelopeFailedFinalizationRetained proves a converged story whose
// final write FAILED keeps its engine_outcome + finalization_ok=false while
// classified error(contradiction) — the facts are not erased (finding 7).
func TestEnvelopeFailedFinalizationRetained(t *testing.T) {
	t.Parallel()

	env := synthesizeEnvelope(exitErr(t, 3), result(outcomeConverged, false))
	if env.outcome != outcomeError || env.detail != detailContradiction {
		t.Fatalf("classification = {%q,%q}, want contradiction", env.outcome, env.detail)
	}

	if env.engineOutcome != outcomeConverged {
		t.Fatalf("engine_outcome erased: %q", env.engineOutcome)
	}

	if env.finalizationOK == nil || *env.finalizationOK {
		t.Fatalf("finalization_ok = %v, want false", env.finalizationOK)
	}

	if got := intVal(t, env.exitCode); got != 3 {
		t.Fatalf("exit_code = %d, want 3", got)
	}
}

// TestEnvelopeNonZeroNotFixedKeepsExitCode proves a legitimate non-zero
// not_fixed keeps its exit code and classification (finding 7).
func TestEnvelopeNonZeroNotFixedKeepsExitCode(t *testing.T) {
	t.Parallel()

	env := synthesizeEnvelope(exitErr(t, 3), result(outcomeNotFixed, true))
	if env.outcome != outcomeNotFixed {
		t.Fatalf("outcome = %q, want not_fixed", env.outcome)
	}

	if got := intVal(t, env.exitCode); got != 3 {
		t.Fatalf("exit_code = %d, want 3", got)
	}
}

// TestEnvelopeSignaledRecordsSignal proves a signaled exit is classified
// interrupted, records the signal name, and carries no exit code (finding 7).
func TestEnvelopeSignaledRecordsSignal(t *testing.T) {
	t.Parallel()

	env := synthesizeEnvelope(signaledErr(t), result(outcomeConverged, true))
	if env.outcome != outcomeInterrupted {
		t.Fatalf("outcome = %q, want interrupted", env.outcome)
	}

	if env.signal == "" {
		t.Fatal("signal name not recorded on a signaled exit")
	}

	if env.exitCode != nil {
		t.Fatalf("exit_code = %v, want nil on a signaled exit", env.exitCode)
	}
}

func TestLockFailureEnvelope(t *testing.T) {
	t.Parallel()

	locked := lockFailureEnvelope(errFolderLocked)
	if locked.outcome != outcomeError || locked.detail != detailFolderLocked {
		t.Fatalf("folder-locked envelope = {%q,%q}", locked.outcome, locked.detail)
	}

	other := lockFailureEnvelope(errUnexpectedStatus)
	if other.outcome != outcomeError || other.detail != detailSpawn {
		t.Fatalf("other lock failure envelope = {%q,%q}", other.outcome, other.detail)
	}
}
