package shared

import "fmt"

// ProcessError represents subprocess execution failures.
type ProcessError struct {
	BaseError

	ExitCode int
	Stderr   string
}

// NewProcessError creates a new ProcessError.
func NewProcessError(message string, exitCode int, stderr string) *ProcessError {
	return &ProcessError{
		BaseError: BaseError{message: message},
		ExitCode:  exitCode,
		Stderr:    stderr,
	}
}

// Type returns the error type for ProcessError.
func (e *ProcessError) Type() string {
	return "process_error"
}

func (e *ProcessError) Error() string {
	message := e.message
	if e.ExitCode != 0 {
		message = fmt.Sprintf("%s (exit code: %d)", message, e.ExitCode)
	}

	if e.Stderr != "" {
		message = fmt.Sprintf("%s\nError output: %s", message, e.Stderr)
	}

	return message
}
