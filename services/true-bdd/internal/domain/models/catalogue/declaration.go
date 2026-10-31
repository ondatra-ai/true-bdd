package catalogue

// Declaration is one term a concept relies on, beside the concept as a
// finding names it ("then concept 'answer'").
type Declaration struct {
	Concept string
	Term    string
}

// quote wraps a term in double quotes, verbatim, as a finding quotes it.
func quote(text string) string {
	return `"` + text + `"`
}
