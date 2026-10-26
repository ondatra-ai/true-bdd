package claude

import "errors"

// ErrResult reports an API error result, including rate and usage limits.
var ErrResult = errors.New("claude error result")

// TurnOutcome identifies a failed turn after the CLI started.
type TurnOutcome string

const (
	TurnNonZeroExit TurnOutcome = "non-zero exit"
	TurnErrorResult TurnOutcome = "error result"
	TurnEmptyOutput TurnOutcome = "empty output"
)

// StartedTurnError distinguishes failures after spawn from start failures.
type StartedTurnError struct {
	Outcome TurnOutcome
	Cause   error
}

func (e *StartedTurnError) Error() string {
	return "claude turn " + string(e.Outcome) + ": " + e.Cause.Error()
}
func (e *StartedTurnError) Unwrap() error { return e.Cause }
