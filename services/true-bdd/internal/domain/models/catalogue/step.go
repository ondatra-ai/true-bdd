package catalogue

import "strings"

// Prefixes that hand a step to a model, which no concept states. The
// Registry classifies its steps by them, as bddgo does.
const (
	// PrefixAct marks a step a model PERFORMS. Legal in given:/when: only.
	PrefixAct = "llm:"
	// PrefixRule marks a clause a model RULES ON. Legal in then: only.
	PrefixRule = "judge:"
)

// Step is one step as a closure check reads it: its text without its
// and:/but: keyword, trimmed, and the block it sits in.
type Step struct {
	Block Block
	Text  string
}

// NewStep is one statement of a block as a closure check reads it.
func NewStep(block Block, statement string) Step {
	return Step{Block: block, Text: strings.TrimSpace(statement)}
}

// ModelRun reports whether a model runs the step, which exempts it from
// every closure check.
func (s Step) ModelRun() bool {
	return strings.HasPrefix(s.Text, PrefixAct) || strings.HasPrefix(s.Text, PrefixRule)
}
