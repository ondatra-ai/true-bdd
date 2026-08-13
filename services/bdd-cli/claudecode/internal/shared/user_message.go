package shared

import (
	"encoding/json"
	"fmt"
)

// UserMessage represents a message from the user.
type UserMessage struct {
	MessageType string      `json:"type"`
	Content     interface{} `json:"content"` // string or []ContentBlock
}

// Type returns the message type for UserMessage.
func (m *UserMessage) Type() string {
	return MessageTypeUser
}

// MarshalJSON implements custom JSON marshaling for UserMessage.
func (m *UserMessage) MarshalJSON() ([]byte, error) {
	type userMessage UserMessage

	temp := struct {
		*userMessage

		Type string `json:"type"`
	}{
		userMessage: (*userMessage)(m),
		Type:        MessageTypeUser,
	}

	data, err := json.Marshal(temp)
	if err != nil {
		return nil, fmt.Errorf("marshal user message: %w", err)
	}

	return data, nil
}
