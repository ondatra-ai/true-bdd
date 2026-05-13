package shared

import "fmt"

const maxLineDisplayLength = 100

// JSONDecodeError represents JSON parsing failures.
type JSONDecodeError struct {
	BaseError

	Line          string
	Position      int
	OriginalError error
}

// NewJSONDecodeError creates a new JSONDecodeError.
func NewJSONDecodeError(line string, position int, cause error) *JSONDecodeError {
	// Match Python behavior: truncate line to maxLineDisplayLength chars and add ...
	truncatedLine := line
	if len(line) > maxLineDisplayLength {
		truncatedLine = line[:maxLineDisplayLength]
	}

	message := fmt.Sprintf("Failed to decode JSON: %s...", truncatedLine)

	return &JSONDecodeError{
		BaseError:     BaseError{message: message}, // Don't include cause in message
		Line:          line,
		Position:      position,
		OriginalError: cause, // Store separately like Python
	}
}

// Type returns the error type for JSONDecodeError.
func (e *JSONDecodeError) Type() string {
	return "json_decode_error"
}

func (e *JSONDecodeError) Unwrap() error {
	return e.OriginalError
}
