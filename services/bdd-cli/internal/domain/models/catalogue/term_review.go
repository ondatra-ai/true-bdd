package catalogue

import "fmt"

// TermReview is what a term document says of the terms a run of steps
// relies on: each term once, in the order the steps declare it, and one
// finding per term the document does not name exactly once.
type TermReview struct {
	Terms    []string
	Findings []string
}

// termRules is how one term document words its findings: of a term it
// never names, per declaring template or once, and of one named twice or more.
type termRules struct {
	unnamed     func(declaration Declaration) string
	perTemplate bool
	repeated    func(term string, count int) string
}

// ReviewTerms holds each declared term to the glossary: exactly one
// definition must name it, spelled as the catalogue spells it.
func ReviewTerms(declarations []Declaration, definitions []Definition) TermReview {
	named := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		named = append(named, definition.Term)
	}

	return reviewDeclarations(declarations, named, termRules{
		unnamed: func(declaration Declaration) string {
			return fmt.Sprintf("%s declares %s, which the glossary does not define",
				quote(declaration.Template), quote(declaration.Term))
		},
		perTemplate: true,
		repeated: func(term string, count int) string {
			return fmt.Sprintf("the glossary defines %s %d times", quote(term), count)
		},
	})
}

// reviewDeclarations holds each declared term to the terms a document
// names, once per term, reporting a term it never names per its rules.
func reviewDeclarations(declarations []Declaration, named []string, rules termRules) TermReview {
	counts := make(map[string]int, len(named))
	for _, term := range named {
		counts[term]++
	}

	review := TermReview{Terms: make([]string, 0), Findings: make([]string, 0)}
	listed := make(map[string]bool)

	for _, declaration := range declarations {
		count := counts[declaration.Term]
		first := !listed[declaration.Term]

		if count == 0 && (first || rules.perTemplate) {
			review.Findings = append(review.Findings, rules.unnamed(declaration))
		}

		if !first {
			continue
		}

		listed[declaration.Term] = true

		review.Terms = append(review.Terms, declaration.Term)

		if count > 1 {
			review.Findings = append(review.Findings, rules.repeated(declaration.Term, count))
		}
	}

	return review
}
