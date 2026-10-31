// Package catalogue holds the product's step vocabulary as data: the blocks
// a step sits in, the steps a check reads, the concepts a fix links them to
// with their glossary words and mappings, and the step definition patterns
// an AI proposes for a concept, which the engine checks.
package catalogue

// Block is the block a step sits in, as the Registry and a concept spell it.
type Block string

// The blocks a step sits in.
const (
	BlockGiven Block = "given"
	BlockWhen  Block = "when"
	BlockThen  Block = "then"
)
