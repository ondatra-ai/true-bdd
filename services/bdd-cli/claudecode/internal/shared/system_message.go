package shared

import (
	"encoding/json"
	"fmt"
)

// SystemMessage represents a system message.
type SystemMessage struct {
	MessageType string         `json:"type"`
	Subtype     string         `json:"subtype"`
	Data        map[string]any `json:"-"` // Preserve all original data
}

// Type returns the message type for SystemMessage.
func (m *SystemMessage) Type() string {
	return MessageTypeSystem
}

// MarshalJSON implements custom JSON marshaling for SystemMessage.
func (m *SystemMessage) MarshalJSON() ([]byte, error) {
	data := make(map[string]any)
	for k, v := range m.Data {
		data[k] = v
	}

	data["type"] = MessageTypeSystem
	data["subtype"] = m.Subtype

	result, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshal system message: %w", err)
	}

	return result, nil
}
