package catalogue

import "fmt"

// TermReview is what a term document says of the terms one concept relies
// on: each term once, in the order the concept declares it, and one finding
// per term the document does not name exactly once, naming the concept.
type TermReview struct {
	Terms    []string
	Findings []string
}

// termRules is how one term document words its findings: of a term it
// never names, and of one it names twice or more.
type termRules struct {
	unnamed  func(declaration Declaration) string
	repeated func(declaration Declaration, count int) string
}

// ReviewTerms holds each declared term to the glossary: exactly one
// definition must name it, spelled as the concept spells it.
func ReviewTerms(declarations []Declaration, definitions []Definition) TermReview {
	named := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		named = append(named, definition.Term)
	}

	return reviewDeclarations(declarations, named, termRules{
		unnamed: func(declaration Declaration) string {
			return fmt.Sprintf("%s declares %s, which the glossary does not define",
				declaration.Concept, quote(declaration.Term))
		},
		repeated: func(declaration Declaration, count int) string {
			return fmt.Sprintf("%s: the glossary defines %s %d times", declaration.Concept, quote(declaration.Term), count)
		},
	})
}

// reviewDeclarations holds each declared term to the terms a document
// names, once per term.
func reviewDeclarations(declarations []Declaration, named []string, rules termRules) TermReview {
	counts := make(map[string]int, len(named))
	for _, term := range named {
		counts[term]++
	}

	review := TermReview{Terms: make([]string, 0), Findings: make([]string, 0)}
	listed := make(map[string]bool)

	for _, declaration := range declarations {
		if listed[declaration.Term] {
			continue
		}

		listed[declaration.Term] = true

		review.Terms = append(review.Terms, declaration.Term)

		switch count := counts[declaration.Term]; {
		case count == 0:
			review.Findings = append(review.Findings, rules.unnamed(declaration))
		case count > 1:
			review.Findings = append(review.Findings, rules.repeated(declaration, count))
		}
	}

	return review
}
