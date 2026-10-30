package catalogue

// Specification is what the spec says about the steps one fix binds: the
// templates its steps match, the phrases they reach and the terms those declare,
// or, on a concept host (Declared false), the concepts its steps link to.
type Specification struct {
	Declared bool
	Steps    []PhrasedStep
	Phrases  []Entry
	Terms    []TermSpec
	Concepts []ConceptSpec
}

// PhrasedStep is one step beside the template of every phrase it
// matches; exactly one is the only closed answer.
type PhrasedStep struct {
	Step

	Templates []string
}

// TermSpec is one term a phrase declares, with what the glossary says it
// means and how the architecture maps it: every definition and mapping
// naming it, so a term defined twice or never is shown as it is.
type TermSpec struct {
	Term     string
	Means    []string
	Mappings []Mapping
}

// Specify reads the spec of a fix's steps: own are the Item's, sweep all
// it binds with them. A phrase is reached by a step matching it alone,
// listed once, in the order the steps reach it, own first.
func (c *Catalogue) Specify(own, sweep []Step, definitions []Definition, mappings []Mapping) Specification {
	spec := Specification{
		Declared: true,
		Steps:    make([]PhrasedStep, 0, len(own)),
		Phrases:  make([]Entry, 0),
		Terms:    make([]TermSpec, 0),
	}

	for _, step := range own {
		templates := make([]string, 0, 1)
		for _, entry := range c.Match(step.Text) {
			templates = append(templates, entry.Template)
		}

		spec.Steps = append(spec.Steps, PhrasedStep{Step: step, Templates: templates})
	}

	spec.Phrases = c.reached(append(append(make([]Step, 0, len(own)+len(sweep)), own...), sweep...))
	spec.Terms = declaredTerms(spec.Phrases, definitions, mappings)

	return spec
}

// reached lists the entry each step matches alone, once, in step order.
func (c *Catalogue) reached(steps []Step) []Entry {
	phrases := make([]Entry, 0)
	listed := make(map[string]bool)

	for _, step := range steps {
		if step.ModelRun() {
			continue
		}

		matches := c.Match(step.Text)
		if len(matches) != 1 || listed[matches[0].Template] {
			continue
		}

		listed[matches[0].Template] = true

		phrases = append(phrases, matches[0])
	}

	return phrases
}

// declaredTerms lists every term the phrases declare, once, in the order
// they declare it, beside its glossary definitions and its mappings.
func declaredTerms(phrases []Entry, definitions []Definition, mappings []Mapping) []TermSpec {
	terms := make([]string, 0)
	for _, phrase := range phrases {
		terms = append(terms, phrase.Terms...)
	}

	return termSpecs(terms, definitions, mappings)
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
