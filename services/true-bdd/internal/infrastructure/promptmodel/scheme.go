// Package promptmodel checks produced checklist data without changing the typed rendering input.
package promptmodel

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"
)

// ErrModelData distinguishes a Model mismatch from rendering and AI failures.
var ErrModelData = errors.New("checklist Model data mismatch")

// Scheme is one compiled host-owned fields-and-types contract.
type Scheme struct {
	name     string
	compiled *jsonschema.Schema
}

// Load reads a YAML JSON Schema. Only local definitions in this file are loaded.
func Load(name, path string) (*Scheme, error) {
	content, err := disk.Read(path)
	if err != nil {
		return nil, fmt.Errorf("checklist Model %s scheme %s: %w", name, path, err)
	}

	document, err := yamlValue(content)
	if err != nil {
		return nil, fmt.Errorf("checklist Model %s scheme %s: %w", name, path, err)
	}

	compiler := jsonschema.NewCompiler()
	compiler.UseLoader(jsonschema.SchemeURLLoader{})

	err = compiler.AddResource("model.json", document)
	if err != nil {
		return nil, fmt.Errorf("checklist Model %s scheme %s: %w", name, path, err)
	}

	compiled, err := compiler.Compile("model.json")
	if err != nil {
		return nil, fmt.Errorf("checklist Model %s scheme %s: %w", name, path, err)
	}

	return &Scheme{name: name, compiled: compiled}, nil
}

// Validate only reads data; serialization here never replaces the rendering input.
func (s *Scheme) Validate(data any) error {
	encoded, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("%w: Model %s: %w", ErrModelData, s.name, err)
	}

	value, err := yamlValue(encoded)
	if err != nil {
		return fmt.Errorf("%w: Model %s: %w", ErrModelData, s.name, err)
	}

	err = s.compiled.Validate(value)
	if err != nil {
		return fmt.Errorf("%w: Model %s: %w", ErrModelData, s.name, err)
	}

	return nil
}

func yamlValue(content []byte) (any, error) {
	var value any

	err := yaml.Unmarshal(content, &value)
	if err != nil {
		return nil, fmt.Errorf("decode YAML: %w", err)
	}

	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode JSON-compatible data: %w", err)
	}

	err = json.Unmarshal(encoded, &value)
	if err != nil {
		return nil, fmt.Errorf("decode JSON-compatible data: %w", err)
	}

	return value, nil
}
