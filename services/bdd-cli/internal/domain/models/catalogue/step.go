package catalogue

import "strings"

// Prefixes that hand a step to a model, which no template phrases.
const (
	prefixAct  = "llm:"
	prefixRule = "judge:"
)

// Step is one step as a closure check reads it: its text without its
// and:/but: keyword, trimmed, and the block it sits in.
type Step struct {
	Block string
	Text  string
}

// ModelRun reports whether a model runs the step, which exempts it from
// every closure check.
func (s Step) ModelRun() bool {
	return strings.HasPrefix(s.Text, prefixAct) || strings.HasPrefix(s.Text, prefixRule)
}
