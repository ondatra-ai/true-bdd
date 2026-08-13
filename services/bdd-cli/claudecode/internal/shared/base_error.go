package shared

import "fmt"

// BaseError provides common error functionality.
type BaseError struct {
	message string
	cause   error
}

func (e *BaseError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %v", e.message, e.cause)
	}

	return e.message
}

func (e *BaseError) Unwrap() error {
	return e.cause
}

// Type returns the error type for BaseError.
func (e *BaseError) Type() string {
	return "base_error"
}
