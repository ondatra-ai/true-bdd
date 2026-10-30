package catalogue

// ConceptSpec is one concept a fix's steps link to through the step
// definitions they bind: its closed meaning, the words it relies on, and
// its bindings for the services of those steps.
type ConceptSpec struct {
	Concept  string
	Block    Block
	Means    string
	Terms    []string
	Bindings []ConceptUse
}

// ConceptUse is one service's binding of a concept: the Public interface
// references a definition uses and what it does through them, or its intent.
type ConceptUse struct {
	Service    string
	References []string
	Does       string
	Intent     string
}

// SpecifyConcepts reads a concept host's spec of a fix's steps: the
// concepts they link to, each word those concepts declare, once, in the
// order they declare it, beside its glossary definitions and mappings.
func SpecifyConcepts(concepts []ConceptSpec, definitions []Definition, mappings []Mapping) Specification {
	terms := make([]string, 0)
	for _, concept := range concepts {
		terms = append(terms, concept.Terms...)
	}

	return Specification{
		Steps:    make([]PhrasedStep, 0),
		Phrases:  make([]Entry, 0),
		Terms:    termSpecs(terms, definitions, mappings),
		Concepts: concepts,
	}
}

// Key names a concept within its block.
func (c ConceptSpec) Key() string { return string(c.Block) + ":" + c.Concept }
