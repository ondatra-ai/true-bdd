package taskhandle

import (
	"errors"
	"fmt"
)

// The two terminal shapes. A halt is "something broke, a person decides"; a
// decline is "this run judged the work must not merge". They differ in what
// they write to ClickUp, which is why they are separate types and not a flag.
var (
	errHalted         = errors.New("halted")
	errDeclined       = errors.New("declined")
	errMandateRevoked = errors.New("the mandate was revoked")
)

type haltError struct {
	step  Step
	cause error
}

func (e *haltError) Error() string {
	return fmt.Sprintf("%s at step %s: %v", errHalted, e.step.Label(), e.cause)
}

func (e *haltError) Unwrap() error { return e.cause }

type declineError struct{ reason string }

func (e *declineError) Error() string {
	return fmt.Sprintf("%s: %s", errDeclined, e.reason)
}

func (e *declineError) Unwrap() error { return errDeclined }

// halt stops the run where it stands and waits for a person.
func halt(step Step, cause error) error {
	return &haltError{step: step, cause: cause}
}

// decline refuses to merge on the merits.
func decline(reason string) error {
	return &declineError{reason: reason}
}

// errNoStructuredOutput is a turn that answered without the schema's object —
// a refusal, a timeout mid-answer, or prose where JSON was required.
var errNoStructuredOutput = errors.New("no structured output")

// isHalt and isDecline read the terminal shape off an error.
func isHalt(err error) bool {
	var halted *haltError

	return errors.As(err, &halted)
}

func isDecline(err error) bool {
	var declined *declineError

	return errors.As(err, &declined)
}
