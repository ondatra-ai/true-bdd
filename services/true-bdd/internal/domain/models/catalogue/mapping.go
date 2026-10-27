package catalogue

// Mapping is one entry of the architecture's word mapping: a product
// term as the catalogue spells it, what it is in the implementation, and
// how a step definition sets it up or observes it, never naming code.
type Mapping struct {
	Term       string `yaml:"term"`
	Technical  string `yaml:"technical"`
	ObservedBy string `yaml:"observed_by"`
}
