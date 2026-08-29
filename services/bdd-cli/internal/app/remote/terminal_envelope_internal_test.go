package remote

import (
	"syscall"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/cli"
)

// exitResult is a child that exited with code. The genuine syscall.WaitStatus
// this used to spawn a process for is now classified in pkg/shell, whose own
// tests signal a real process; here the classification is the input.
func exitResult(code int) cli.Result {
	return cli.Result{Code: code}
}

// signaledResult is a child SIGKILLed rather than returned from.
func signaledResult() cli.Result {
	return cli.Result{Code: -1, Signal: syscall.SIGKILL}
}

// result builds a present childResult for the envelope table.
func result(outcome string, finalizationOK bool) childResult {
	return childResult{present: true, outcome: outcome, finalizationOK: finalizationOK}
}

func TestSynthesizeEnvelopePrecedence(t *testing.T) {
	t.Parallel()

	signaled := signaledResult()
	nonZero := exitResult(3)

	cases := []struct {
		name        string
		exit        cli.Result
		result      childResult
		wantOutcome string
		wantDetail  string
	}{
		{"signal beats everything", signaled, result(outcomeConverged, true), outcomeInterrupted, ""},
		{"signal with no result still interrupted", signaled, childResult{}, outcomeInterrupted, ""},
		{"no result on clean exit", cli.Result{}, childResult{}, outcomeError, detailNoResult},
		{"no result on non-zero exit", nonZero, childResult{}, outcomeError, detailNoResult},
		{"clean exit yields engine outcome", cli.Result{}, result(outcomeConverged, true), outcomeConverged, ""},
		{"contradiction on non-zero success", nonZero, result(outcomeConverged, true), outcomeError, detailContradiction},
		{"non-zero + not_fixed is legitimate", nonZero, result(outcomeNotFixed, true), outcomeNotFixed, ""},
		{"non-zero + max_attempts legitimate", nonZero, result(outcomeMaxAttempts, true), outcomeMaxAttempts, ""},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			env := synthesizeEnvelope(testCase.exit, testCase.result)
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

	env := synthesizeEnvelope(exitResult(3), result(outcomeConverged, false))
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

	env := synthesizeEnvelope(exitResult(3), result(outcomeNotFixed, true))
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

	env := synthesizeEnvelope(signaledResult(), result(outcomeConverged, true))
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
