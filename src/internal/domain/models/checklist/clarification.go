package checklist

// ClarifyQuestion represents a question the AI needs answered before generating a fix.
type ClarifyQuestion struct {
	ID       string   `yaml:"id"`       // Unique identifier (e.g., "q1", "q2")
	Question string   `yaml:"question"` // The question text
	Context  string   `yaml:"context"`  // Why this question matters
	Options  []string `yaml:"options"`  // Suggested answers (user can provide custom)
}

// GenerateResult represents the output of fix prompt generation.
// Either FixPrompt OR Questions will be populated, never both.
type GenerateResult struct {
	FixPrompt string            // Non-empty if fix was generated successfully
	Questions []ClarifyQuestion // Non-empty if clarification is needed
}

// HasQuestions returns true if this result contains questions.
func (r *GenerateResult) HasQuestions() bool {
	return len(r.Questions) > 0
}

// HasFixPrompt returns true if this result contains a fix prompt.
func (r *GenerateResult) HasFixPrompt() bool {
	return r.FixPrompt != ""
}
