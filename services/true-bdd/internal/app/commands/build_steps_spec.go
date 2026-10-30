package commands

import (
	"fmt"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/docs"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/product"
)

// productDocument is the configured product path, or "" when no document resolves.
func productDocument(resolver *docs.Resolver) string {
	path, err := resolver.Resolve(docs.KeyProduct)
	if err != nil {
		return ""
	}

	return path
}

// stepSpecification reads the glossary and word mapping shared by build
// steps and build code.
func stepSpecification(productFile, architectureFile string) ([]catalogue.Definition, []catalogue.Mapping, error) {
	definitions, err := product.LoadGlossary(productFile)
	if err != nil {
		return nil, nil, fmt.Errorf("read the glossary: %w", err)
	}

	mappings, err := architecture.LoadTermMappings(architectureFile)
	if err != nil {
		return nil, nil, fmt.Errorf("read the word mapping: %w", err)
	}

	return definitions, mappings, nil
}
