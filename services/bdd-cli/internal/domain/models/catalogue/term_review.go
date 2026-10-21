package catalogue

import "fmt"

// TermReview is what the glossary says of the terms a run of steps
// relies on: each term once, in the order the steps declare it, and one
// finding per term the glossary does not define exactly once.
type TermReview struct {
	Terms    []string
	Findings []string
}

// ReviewTerms holds each declared term to the glossary: exactly one
// definition must name it, spelled as the catalogue spells it.
func ReviewTerms(declarations []Declaration, definitions []Definition) TermReview {
	defined := make(map[string]int, len(definitions))
	for _, definition := range definitions {
		defined[definition.Term]++
	}

	review := TermReview{Terms: make([]string, 0), Findings: make([]string, 0)}
	listed := make(map[string]bool)

	for _, declaration := range declarations {
		count := defined[declaration.Term]
		if count == 0 {
			review.Findings = append(review.Findings, fmt.Sprintf("%s declares %s, which the glossary does not define",
				quote(declaration.Template), quote(declaration.Term)))
		}

		if listed[declaration.Term] {
			continue
		}

		listed[declaration.Term] = true

		review.Terms = append(review.Terms, declaration.Term)

		if count > 1 {
			review.Findings = append(review.Findings, fmt.Sprintf("the glossary defines %s %d times",
				quote(declaration.Term), count))
		}
	}

	return review
}
