package shared

// Content block type constants.
const (
	ContentBlockTypeText       = "text"
	ContentBlockTypeThinking   = "thinking"
	ContentBlockTypeToolUse    = "tool_use"
	ContentBlockTypeToolResult = "tool_result"
)

// ContentBlock represents any content block within a message.
type ContentBlock interface {
	BlockType() string
}
