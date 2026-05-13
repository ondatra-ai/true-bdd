package claudecode

import (
	"bdd-cli/src/claudecode/internal/shared"
)

// Message represents any message type in the Claude Code protocol.
type Message = shared.Message

// Message type constants.
const (
	MessageTypeUser      = shared.MessageTypeUser
	MessageTypeAssistant = shared.MessageTypeAssistant
	MessageTypeSystem    = shared.MessageTypeSystem
	MessageTypeResult    = shared.MessageTypeResult
)
