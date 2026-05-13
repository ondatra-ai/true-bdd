package shared

// TextBlock represents text content.
type TextBlock struct {
	MessageType string `json:"type"`
	Text        string `json:"text"`
}

// BlockType returns the content block type for TextBlock.
func (b *TextBlock) BlockType() string {
	return ContentBlockTypeText
}

// ThinkingBlock represents thinking content with signature.
type ThinkingBlock struct {
	MessageType string `json:"type"`
	Thinking    string `json:"thinking"`
	Signature   string `json:"signature"`
}

// BlockType returns the content block type for ThinkingBlock.
func (b *ThinkingBlock) BlockType() string {
	return ContentBlockTypeThinking
}

// ToolUseBlock represents a tool use request.
type ToolUseBlock struct {
	MessageType string         `json:"type"`
	ToolUseID   string         `json:"tool_use_id"`
	Name        string         `json:"name"`
	Input       map[string]any `json:"input"`
}

// BlockType returns the content block type for ToolUseBlock.
func (b *ToolUseBlock) BlockType() string {
	return ContentBlockTypeToolUse
}

// ToolResultBlock represents the result of a tool use.
type ToolResultBlock struct {
	MessageType string      `json:"type"`
	ToolUseID   string      `json:"tool_use_id"`
	Content     interface{} `json:"content"` // string or structured data
	IsError     *bool       `json:"is_error,omitempty"`
}

// BlockType returns the content block type for ToolResultBlock.
func (b *ToolResultBlock) BlockType() string {
	return ContentBlockTypeToolResult
}
