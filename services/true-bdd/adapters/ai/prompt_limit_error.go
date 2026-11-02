package ai

import "fmt"

// DefaultPromptLimit is the host's default system plus user byte allowance.
const DefaultPromptLimit = 150000

// PromptLimitError refuses a turn before any provider is started.
type PromptLimitError struct {
	Turn  string
	Size  int
	Limit int
}

func (e *PromptLimitError) Error() string {
	return fmt.Sprintf("prompt limit exceeded for %s: %d bytes, limit %d bytes", e.Turn, e.Size, e.Limit)
}
