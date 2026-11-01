package catalogue

// ConceptKey names a concept within its block, block:concept: the one key
// a concept's binding, its words and its Item are judged and looked up by.
func ConceptKey(block Block, concept string) string {
	return string(block) + ":" + concept
}
