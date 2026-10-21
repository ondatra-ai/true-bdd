// Package catalogue is the product document's step-phrase catalogue: the
// shapes a Scenario's steps may take, each with what it means. A template
// compiles to one regular expression, and a step must match exactly one.
package catalogue

// The blocks a step sits in, as the Registry and a catalogue entry spell them.
const (
	BlockGiven = "given"
	BlockWhen  = "when"
	BlockThen  = "then"
)

// Entry is one phrase of the catalogue. Template is its identity; Block is
// the one block a step of that shape may sit in.
type Entry struct {
	Template string   `yaml:"template"`
	Block    string   `yaml:"block"`
	Means    string   `yaml:"means"`
	Terms    []string `yaml:"terms"`
}

// validBlock reports whether a block is one a step can sit in.
func validBlock(block string) bool {
	switch block {
	case BlockGiven, BlockWhen, BlockThen:
		return true
	}

	return false
}
