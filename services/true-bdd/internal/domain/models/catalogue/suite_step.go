package catalogue

// SuiteStep is one deterministic step of a suite as the pattern check
// reads it: the Item whose grouping holds it, or the definition binding it.
type SuiteStep struct {
	Block Block
	Text  string
	Item  string
	Bound string
}
