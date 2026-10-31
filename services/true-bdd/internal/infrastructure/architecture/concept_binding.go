package architecture

import "github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"

// ConceptBinding states a concept's use through a service's Public interface.
type ConceptBinding struct {
	Concept    string               `yaml:"concept"`
	Block      catalogue.Block      `yaml:"block"`
	Service    string               `yaml:"service"`
	References []InterfaceReference `yaml:"references,omitempty"`
	Does       string               `yaml:"does,omitempty"`
	Intent     string               `yaml:"intent,omitempty"`
}
