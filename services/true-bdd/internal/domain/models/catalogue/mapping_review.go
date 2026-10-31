package catalogue

import "fmt"

// ReviewMappings holds each declared term to the architecture's word
// mapping: exactly one mapping must name it, spelled as the concept spells it.
func ReviewMappings(declarations []Declaration, mappings []Mapping) TermReview {
	named := make([]string, 0, len(mappings))
	for _, mapping := range mappings {
		named = append(named, mapping.Term)
	}

	return reviewDeclarations(declarations, named, termRules{
		unnamed: func(declaration Declaration) string {
			return fmt.Sprintf("%s: %s has no mapping in the architecture", declaration.Concept, quote(declaration.Term))
		},
		repeated: func(declaration Declaration, count int) string {
			return fmt.Sprintf("%s: the architecture maps %s %d times", declaration.Concept, quote(declaration.Term), count)
		},
	})
}
