package triage

// The verdict validator and the prompt builder are unexported; the tests reach
// them through this seam, which the compiler drops from any non-test build.

// ValidateForTest is the check on an answer that the schema cannot express.
func ValidateForTest(verdict Verdict, filed bool) error { return verdict.validate(filed) }

// PromptForTest is the whole turn a subject renders to.
func PromptForTest(subject Subject) string { return subject.prompt() }

// SchemaForTest is the shape the turn is held to.
func SchemaForTest() string { return verdictSchema }
