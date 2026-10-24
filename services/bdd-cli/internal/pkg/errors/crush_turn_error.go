package errors

// CrushTurnOutcome identifies how a started crush turn failed.
type CrushTurnOutcome string

const (
	CrushTurnNonZeroExit CrushTurnOutcome = "non-zero exit"
	CrushTurnStalled     CrushTurnOutcome = "stalled"
	CrushTurnEmptyOutput CrushTurnOutcome = "empty output"
)

// CrushTurnError reports a failed turn that started, preserving its cause.
// A start failure or engine refusal must never carry this outcome.
type CrushTurnError struct {
	Outcome CrushTurnOutcome
	Cause   error
}

func (f *CrushTurnError) Error() string { return "crush turn " + string(f.Outcome) + ": " + f.Cause.Error() }
func (f *CrushTurnError) Unwrap() error { return f.Cause }
