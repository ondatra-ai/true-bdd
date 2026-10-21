package catalogue

import (
	"fmt"
	"regexp"
)

// Malformation is one catalogue entry the grammar refuses, and why.
type Malformation struct {
	Template string
	Reason   string
}

// Line renders the malformation as a check's context line reads it.
func (m Malformation) Line() string {
	return fmt.Sprintf("catalogue template %s is malformed: %s", quote(m.Template), m.Reason)
}

// compiledEntry is one entry beside the expression its steps must match.
type compiledEntry struct {
	entry   Entry
	pattern *regexp.Regexp
}

// Catalogue is the compiled catalogue: every entry the grammar accepts,
// in the order the product document lists them.
type Catalogue struct {
	entries []compiledEntry
}

// Compile compiles every entry against the product's roles. A refused
// entry is left out and reported; a template declared twice is reported once.
func Compile(entries []Entry, roles []string) (*Catalogue, []Malformation) {
	declared := make(map[string]int, len(entries))
	for _, entry := range entries {
		declared[entry.Template]++
	}

	compiled := &Catalogue{entries: make([]compiledEntry, 0, len(entries))}
	malformed := make([]Malformation, 0)
	reported := make(map[string]bool)

	for _, entry := range entries {
		reason := ""

		switch {
		case declared[entry.Template] > 1:
			reason = "another entry declares the same template"
		case !validBlock(entry.Block):
			reason = fmt.Sprintf("its block %s is not given, when or then", quote(entry.Block))
		}

		pattern, err := compileTemplate(entry.Template, roles)
		if reason == "" && err != nil {
			reason = err.Error()
		}

		if reason == "" {
			compiled.entries = append(compiled.entries, compiledEntry{entry: entry, pattern: pattern})

			continue
		}

		if !reported[entry.Template] {
			reported[entry.Template] = true

			malformed = append(malformed, Malformation{Template: entry.Template, Reason: reason})
		}
	}

	return compiled, malformed
}

// Match lists every entry whose template the whole text matches, in
// catalogue order. Exactly one is the only closed answer.
func (c *Catalogue) Match(text string) []Entry {
	matches := make([]Entry, 0, 1)

	for _, candidate := range c.entries {
		if candidate.pattern.MatchString(text) {
			matches = append(matches, candidate.entry)
		}
	}

	return matches
}
