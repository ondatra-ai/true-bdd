package errors

// CodexTurnOutcome identifies a started codex writer's failure.
type CodexTurnOutcome string

const (
	CodexTurnNonZeroExit CodexTurnOutcome = "non-zero exit"
	CodexTurnEmptyOutput CodexTurnOutcome = "empty output"
)

// CodexTurnError excludes start failures and engine refusals before spawning.
type CodexTurnError struct {
	Outcome CodexTurnOutcome
	Cause   error
}

func (f *CodexTurnError) Error() string {
	return "codex turn " + string(f.Outcome) + ": " + f.Cause.Error()
}
func (f *CodexTurnError) Unwrap() error { return f.Cause }
