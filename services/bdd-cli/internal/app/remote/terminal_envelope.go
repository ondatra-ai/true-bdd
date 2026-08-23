package remote

import (
	"errors"
	"os/exec"
	"syscall"
)

const (
	outcomeError        = "error"
	outcomeInterrupted  = "interrupted"
	detailSpawn         = "spawn"
	detailNoResult      = "no_result"
	detailContradiction = "contradiction"
	detailFolderLocked  = "folder_locked"
	// detailTimeout marks a bounded scan-drain wait exceeded — a 504-class
	// dispatch failure, distinct from a spawn error or a false folder_locked
	// (plan §1.5).
	detailTimeout = "timeout"

	// Engine stop-reason outcomes the remote reasons about (mirrors the
	// runner's event vocabulary). Named so the contradiction check and the
	// tests share one source of truth.
	outcomeOK          = "ok"
	outcomeConverged   = "converged"
	outcomeUserExit    = "user_exit"
	outcomeNotFixed    = "not_fixed"
	outcomeMaxAttempts = "max_attempts"
)

// childResult is the complete engine result the child emitted on the event
// channel (plan §3.2 / finding 7), retained WHOLE so synthesizeEnvelope never
// erases it (a converged story whose write failed keeps engine_outcome=converged + finalization_ok=false).
type childResult struct {
	present        bool
	outcome        string
	finalizationOK bool
	detail         string
}

// terminalEnvelope is the remote-synthesized final classification for a run
// plus the FULL diagnostic envelope carried to the server/DB/API/UI (plan
// §3.2 / finding 7): outcome is the badge, detail set only for the error class.
type terminalEnvelope struct {
	outcome        string
	detail         string
	engineOutcome  string
	finalizationOK *bool
	exitCode       *int
	signal         string
}

// synthesizeEnvelope applies the plan §3.2 precedence (see
// TestSynthesizeEnvelopePrecedence): signal ⇒ interrupted; no result ⇒
// error(no_result); else the engine outcome, or error(contradiction) on a non-zero exit from a success-class result.
func synthesizeEnvelope(waitErr error, result childResult) terminalEnvelope {
	code, signaled, signalName := exitInfo(waitErr)

	env := attachFacts(terminalEnvelope{}, facts{
		code:       code,
		signaled:   signaled,
		signalName: signalName,
		result:     result,
	})

	switch {
	case signaled:
		env.outcome = outcomeInterrupted
	case !result.present:
		env.outcome = outcomeError
		env.detail = detailNoResult
	case code == 0:
		env.outcome = result.outcome
	case isSuccessClass(result.outcome):
		env.outcome = outcomeError
		env.detail = detailContradiction
	default:
		env.outcome = result.outcome
	}

	return env
}

// facts bundles the retained exit/signal/engine-result diagnostics.
type facts struct {
	code       int
	signaled   bool
	signalName string
	result     childResult
}

// attachFacts records the retained diagnostics on the envelope regardless
// of the final classification.
func attachFacts(env terminalEnvelope, f facts) terminalEnvelope {
	code, signaled, signalName, result := f.code, f.signaled, f.signalName, f.result
	if !signaled {
		exitCode := code
		env.exitCode = &exitCode
	}

	env.signal = signalName

	if result.present {
		env.engineOutcome = result.outcome
		finalizationOK := result.finalizationOK
		env.finalizationOK = &finalizationOK
	}

	return env
}

// exitInfo extracts the exit code, whether the child was killed by a
// signal, and (if so) the signal name from a cmd.Wait() error.
func exitInfo(waitErr error) (int, bool, string) {
	if waitErr == nil {
		return 0, false, ""
	}

	var exitErr *exec.ExitError
	if errors.As(waitErr, &exitErr) {
		status, ok := exitErr.Sys().(syscall.WaitStatus)
		if ok {
			if status.Signaled() {
				return status.ExitStatus(), true, status.Signal().String()
			}

			return status.ExitStatus(), false, ""
		}

		return exitErr.ExitCode(), false, ""
	}

	return -1, false, ""
}

// isSuccessClass reports whether the outcome demands a clean (zero)
// exit; a non-zero exit alongside one of these is a contradiction.
func isSuccessClass(outcome string) bool {
	switch outcome {
	case outcomeOK, outcomeConverged, outcomeUserExit:
		return true
	default:
		return false
	}
}
