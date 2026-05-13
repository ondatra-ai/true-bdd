package shared

import "fmt"

// CLINotFoundError indicates the Claude CLI was not found.
type CLINotFoundError struct {
	BaseError

	Path string
}

// NewCLINotFoundError creates a new CLINotFoundError.
func NewCLINotFoundError(path, message string) *CLINotFoundError {
	// Match Python behavior: if path provided, format as "message: path"
	if path != "" {
		message = fmt.Sprintf("%s: %s", message, path)
	}

	return &CLINotFoundError{
		BaseError: BaseError{message: message},
		Path:      path,
	}
}

// Type returns the error type for CLINotFoundError.
func (e *CLINotFoundError) Type() string {
	return "cli_not_found_error"
}
