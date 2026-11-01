package product

import "github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"

// Concept declares one closed meaning within a Given, When or Then block.
type Concept struct {
	Name  string          `yaml:"concept"`
	Means string          `yaml:"means"`
	Terms []string        `yaml:"terms"`
	Block catalogue.Block `yaml:"-"`
}
