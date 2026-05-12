package claudecode

import (
	"bdd-cli/src/claudecode/internal/shared"
)

// ContentBlock represents any content block within a message.
type ContentBlock = shared.ContentBlock

// Content block type constants.
const (
	ContentBlockTypeText       = shared.ContentBlockTypeText
	ContentBlockTypeThinking   = shared.ContentBlockTypeThinking
	ContentBlockTypeToolUse    = shared.ContentBlockTypeToolUse
	ContentBlockTypeToolResult = shared.ContentBlockTypeToolResult
)
