package catalogue

import "fmt"

// ReviewMappings holds each declared term to the architecture's word
// mapping: exactly one mapping must name it, spelled as the catalogue
// spells it. Each finding names a term once, whichever templates declare it.
func ReviewMappings(declarations []Declaration, mappings []Mapping) TermReview {
	mapped := make(map[string]int, len(mappings))
	for _, mapping := range mappings {
		mapped[mapping.Term]++
	}

	review := TermReview{Terms: make([]string, 0), Findings: make([]string, 0)}
	listed := make(map[string]bool)

	for _, declaration := range declarations {
		if listed[declaration.Term] {
			continue
		}

		listed[declaration.Term] = true

		review.Terms = append(review.Terms, declaration.Term)

		switch count := mapped[declaration.Term]; {
		case count == 0:
			review.Findings = append(review.Findings,
				quote(declaration.Term)+" has no mapping in the architecture")
		case count > 1:
			review.Findings = append(review.Findings,
				fmt.Sprintf("the architecture maps %s %d times", quote(declaration.Term), count))
		}
	}

	return review
}
