package catalogue

// Specification is what the spec says about the steps one fix binds on a
// concept host: the concepts its steps link to, and every term those
// concepts declare beside its glossary definitions and mappings.
type Specification struct {
	Terms    []TermSpec
	Concepts []ConceptSpec
}

// TermSpec is one term a concept declares, with what the glossary says it
// means and how the architecture maps it: every definition and mapping
// naming it, so a term defined twice or never is shown as it is.
type TermSpec struct {
	Term     string
	Means    []string
	Mappings []Mapping
}

// termSpecs lists each term once, in order, beside every glossary
// definition and mapping naming it.
func termSpecs(terms []string, definitions []Definition, mappings []Mapping) []TermSpec {
	listed := make(map[string]bool)
	found := make([]TermSpec, 0)

	for _, term := range terms {
		if listed[term] {
			continue
		}

		listed[term] = true

		word := TermSpec{Term: term, Means: make([]string, 0, 1), Mappings: make([]Mapping, 0, 1)}

		for _, definition := range definitions {
			if definition.Term == term {
				word.Means = append(word.Means, definition.Means)
			}
		}

		for _, mapping := range mappings {
			if mapping.Term == term {
				word.Mappings = append(word.Mappings, mapping)
			}
		}

		found = append(found, word)
	}

	return found
}
