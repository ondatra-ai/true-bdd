package catalogue

// Definition is one entry of the product glossary: a term, spelled
// exactly as the catalogue entries declare it, and what it means.
type Definition struct {
	Term  string `yaml:"term"`
	Means string `yaml:"means"`
}
