package errors

// CrushTurnError reports a failed turn that started, preserving its cause.
// A start failure or engine refusal must never carry this outcome.
type CrushTurnError struct {
	Outcome string
	Cause   error
}

func (f *CrushTurnError) Error() string { return "crush turn " + f.Outcome + ": " + f.Cause.Error() }
func (f *CrushTurnError) Unwrap() error { return f.Cause }
