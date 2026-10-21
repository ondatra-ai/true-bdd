package catalogue

import (
	"cmp"
	"fmt"
	"strings"
)

// StepDefinition is one step definition a suite registers: its pattern as
// the suite compiles it, and the file and line it is written at.
type StepDefinition struct {
	Pattern string
	File    string
	Line    int
}

// Binder lists every step definition a step's text binds to.
type Binder func(text string) []StepDefinition

// label names the definition as a finding reads it.
func (d StepDefinition) label() string {
	return fmt.Sprintf("%s (%s:%d)", d.Pattern, d.File, d.Line)
}

// compareDefinitions orders definitions by where they are written.
func compareDefinitions(left, right StepDefinition) int {
	return cmp.Or(
		strings.Compare(left.File, right.File),
		cmp.Compare(left.Line, right.Line),
		strings.Compare(left.Pattern, right.Pattern),
	)
}

// labels joins the definitions' labels, as a finding lists them.
func labels(definitions []StepDefinition) string {
	named := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		named = append(named, definition.label())
	}

	return strings.Join(named, "; ")
}
