package product

import (
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
)

// ErrInvalidConcepts identifies a malformed concept declaration.
var ErrInvalidConcepts = errors.New("concepts: requires given, when or then lists with concept, means and terms; " +
	"names must be unique within each block")

// ValidateForm refuses the retired catalogue and malformed concepts before a
// command starts.
func ValidateForm(path string) error {
	doc, found, err := readDocument(path)
	if errors.Is(err, ErrCatalogueRetired) {
		return err
	}
	// Legacy unreadable products keep their command's original finding (#256,
	// engine-term-documents-003-product-not-yaml-stops-scen-check).
	if err != nil || !found {
		return nil //nolint:nilerr // The command retains responsibility for legacy read errors.
	}

	_, err = decodeConcepts(path, &doc.Concepts)

	return err
}

func decodeConcepts(path string, node *yaml.Node) ([]Concept, error) {
	if node.Kind == 0 {
		return nil, nil
	}

	if node.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("product document %s: %w", path, ErrInvalidConcepts)
	}

	entries := make([]Concept, 0)
	seen := make(map[catalogue.Block]bool)

	for index := 0; index < len(node.Content); index += 2 {
		block := catalogue.Block(node.Content[index].Value)

		list := node.Content[index+1]

		validBlock := block == catalogue.BlockGiven || block == catalogue.BlockWhen || block == catalogue.BlockThen
		if !validBlock || list.Kind != yaml.SequenceNode || seen[block] {
			return nil, fmt.Errorf("product document %s: block %s: %w", path, block, ErrInvalidConcepts)
		}

		seen[block] = true

		concepts, err := decodeConceptBlock(path, block, list)
		if err != nil {
			return nil, err
		}

		entries = append(entries, concepts...)
	}

	return entries, nil
}

func decodeConceptBlock(path string, block catalogue.Block, list *yaml.Node) ([]Concept, error) {
	entries := make([]Concept, 0, len(list.Content))
	seen := make(map[string]bool)

	for _, node := range list.Content {
		var entry Concept

		err := node.Decode(&entry)
		if err != nil {
			return nil, fmt.Errorf("product document %s: %s concept: %w", path, block, err)
		}

		if incompleteConcept(entry) || seen[entry.Name] {
			return nil, fmt.Errorf("product document %s: %s concept '%s': %w", path, block, entry.Name, ErrInvalidConcepts)
		}

		entry.Block = block
		seen[entry.Name] = true
		entries = append(entries, entry)
	}

	return entries, nil
}

func incompleteConcept(entry Concept) bool {
	return strings.TrimSpace(entry.Name) == "" || strings.TrimSpace(entry.Means) == "" || entry.Terms == nil
}
