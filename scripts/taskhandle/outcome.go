package taskhandle

// Outcome is how a run ended. Five verdicts, and the command exits 0 for all
// of them: a halt is something the tool successfully decided, and task-loop
// must keep working the queue whichever one it gets.
type Outcome string

const (
	OutcomeDone          Outcome = "DONE"
	OutcomeFailed        Outcome = "FAILED"
	OutcomeHalted        Outcome = "halted"
	OutcomeAwaitingMerge Outcome = "awaiting merge"
	OutcomeNotStarted    Outcome = "not started"
)
