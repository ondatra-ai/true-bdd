package errors

// ClaudeTurnOutcome identifies a failed, started build-code writer.
type ClaudeTurnOutcome string

const (
	ClaudeTurnNonZeroExit ClaudeTurnOutcome = "non-zero exit"
	ClaudeTurnErrorResult ClaudeTurnOutcome = "error result"
	ClaudeTurnEmptyOutput ClaudeTurnOutcome = "empty output"
)

// ClaudeTurnError preserves a started writer failure for the engine-owned rerun.
type ClaudeTurnError struct {
	Outcome ClaudeTurnOutcome
	Cause   error
}

func (e *ClaudeTurnError) Error() string {
	return "claude turn " + string(e.Outcome) + ": " + e.Cause.Error()
}
func (e *ClaudeTurnError) Unwrap() error { return e.Cause }
