// Package catalogue is the product document's step-phrase catalogue: the
// shapes a Scenario's steps may take, each with what it means. A template
// compiles to one regular expression, and a step must match exactly one.
// The terms an entry declares are the glossary words its meaning relies
// on, and each must be defined exactly once.
package catalogue

// Block is the block a step sits in, as the Registry and a catalogue entry
// spell it.
type Block string

// The blocks a step sits in.
const (
	BlockGiven Block = "given"
	BlockWhen  Block = "when"
	BlockThen  Block = "then"
)

// Entry is one phrase of the catalogue. Template is its identity; Block is
// the one block a step of that shape may sit in.
type Entry struct {
	Template string   `yaml:"template"`
	Block    Block    `yaml:"block"`
	Means    string   `yaml:"means"`
	Terms    []string `yaml:"terms"`
}

// validBlock reports whether a block is one a step can sit in.
func validBlock(block Block) bool {
	switch block {
	case BlockGiven, BlockWhen, BlockThen:
		return true
	}

	return false
}
