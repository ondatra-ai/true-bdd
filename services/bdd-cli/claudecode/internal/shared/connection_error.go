package shared

// ConnectionError represents connection-related failures.
type ConnectionError struct {
	BaseError
}

// NewConnectionError creates a new ConnectionError.
func NewConnectionError(message string, cause error) *ConnectionError {
	return &ConnectionError{
		BaseError: BaseError{message: message, cause: cause},
	}
}

// Type returns the error type for ConnectionError.
func (e *ConnectionError) Type() string {
	return "connection_error"
}
