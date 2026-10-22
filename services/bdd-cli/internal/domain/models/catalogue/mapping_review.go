package catalogue

import "fmt"

// ReviewMappings holds each declared term to the architecture's word
// mapping: exactly one mapping must name it, spelled as the catalogue
// spells it. Each finding names a term once, whichever templates declare it.
func ReviewMappings(declarations []Declaration, mappings []Mapping) TermReview {
	named := make([]string, 0, len(mappings))
	for _, mapping := range mappings {
		named = append(named, mapping.Term)
	}

	return reviewDeclarations(declarations, named, termRules{
		unnamed: func(declaration Declaration) string {
			return quote(declaration.Term) + " has no mapping in the architecture"
		},
		perTemplate: false,
		repeated: func(term string, count int) string {
			return fmt.Sprintf("the architecture maps %s %d times", quote(term), count)
		},
	})
}
