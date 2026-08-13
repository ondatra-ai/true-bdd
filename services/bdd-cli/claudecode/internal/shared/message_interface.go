package shared

// Message type constants.
const (
	MessageTypeUser      = "user"
	MessageTypeAssistant = "assistant"
	MessageTypeSystem    = "system"
	MessageTypeResult    = "result"
)

// Message represents any message type in the Claude Code protocol.
type Message interface {
	Type() string
}
