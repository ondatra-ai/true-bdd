package claudecode

import (
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/claudecode/internal/shared"
)

// TextBlock represents text content.
type TextBlock = shared.TextBlock

// ThinkingBlock represents thinking content with signature.
type ThinkingBlock = shared.ThinkingBlock

// ToolUseBlock represents a tool use request.
type ToolUseBlock = shared.ToolUseBlock

// ToolResultBlock represents the result of a tool use.
type ToolResultBlock = shared.ToolResultBlock
