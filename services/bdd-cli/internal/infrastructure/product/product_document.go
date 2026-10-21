package product

import (
	"errors"
	"fmt"
	"io/fs"

	"gopkg.in/yaml.v3"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

// productDocument is the slice of the product document the checks decode.
type productDocument struct {
	Roles []struct {
		Name string `yaml:"name"`
	} `yaml:"roles"`
	Catalogue yaml.Node `yaml:"catalogue"`
	Glossary  yaml.Node `yaml:"glossary"`
}

// readDocument parses the product document at path; found is false for
// an empty path or a missing file, which a host without one has.
func readDocument(path string) (*productDocument, bool, error) {
	if path == "" {
		return nil, false, nil
	}

	data, err := disk.Read(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}

	if err != nil {
		return nil, false, fmt.Errorf("read product document %s: %w", path, err)
	}

	var doc productDocument

	err = yaml.Unmarshal(data, &doc)
	if err != nil {
		return nil, false, fmt.Errorf("parse product document %s: %w", path, err)
	}

	return &doc, true, nil
}

// absent reports whether a key was left out or set to null.
func absent(node *yaml.Node) bool {
	return node.Kind == 0 || node.Tag == "!!null"
}
