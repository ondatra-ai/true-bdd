package shared

// MessageParseError represents message structure parsing failures.
type MessageParseError struct {
	BaseError

	Data any
}

// NewMessageParseError creates a new MessageParseError.
func NewMessageParseError(message string, data any) *MessageParseError {
	return &MessageParseError{
		BaseError: BaseError{message: message},
		Data:      data,
	}
}

// Type returns the error type for MessageParseError.
func (e *MessageParseError) Type() string {
	return "message_parse_error"
}
