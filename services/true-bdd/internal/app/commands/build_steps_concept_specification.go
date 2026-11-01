package commands

import (
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/product"
)

// conceptSpecification is what every concept Item's context reads from the Specification.
type conceptSpecification struct {
	concepts    []product.Concept
	definitions []catalogue.Definition
	mappings    []catalogue.Mapping
	bindings    []architecture.ConceptBinding
}

// meaning is the concept's own entry and its binding for the Item's service.
func (spec conceptSpecification) meaning(item *BuildStepsItem) conceptItemContext {
	context := conceptItemContext{}

	for _, concept := range spec.concepts {
		if concept.Name == item.Concept.Name && concept.Block == item.Concept.Block {
			item.Concept.Means = concept.Means
			context.Concept = conceptMeaning{concept.Name, concept.Block, concept.Means, concept.Terms}
		}
	}

	for _, binding := range spec.bindings {
		if binding.Concept == item.Concept.Name && binding.Block == item.Concept.Block && binding.Service == item.Service {
			context.Binding = binding
		}
	}

	return context
}
