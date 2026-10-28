package architecture

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	"gopkg.in/yaml.v3"
)

// ErrInterfaceDocument identifies a document that cannot state a Public interface.
var ErrInterfaceDocument = errors.New("invalid Public interface document")

// LoadInterfaces loads service declarations and their documents, relative to the Host project root.
// It does not require a testing block, so document commands can use it too.
func LoadInterfaces(path, root string) ([]Service, error) {
	data, err := disk.Read(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read architecture file %s: %w", path, err)
	}

	var raw struct {
		Architecture struct {
			Services []Service `yaml:"services"`
		} `yaml:"architecture"`
	}

	err = yaml.Unmarshal(data, &raw)
	if err != nil {
		return nil, fmt.Errorf("failed to parse architecture file %s: %w", path, err)
	}

	for _, service := range raw.Architecture.Services {
		err = loadInterfaceDocument(service.PublicInterface, root)
		if err != nil {
			return nil, fmt.Errorf("service %s: %w", service.Name, err)
		}
	}

	return raw.Architecture.Services, nil
}

func loadInterfaceDocument(declaration *PublicInterface, root string) error {
	if declaration == nil || declaration.Document == "" {
		return nil
	}

	if declaration.Kind != kindCLI && declaration.Kind != kindHTTP {
		return nil
	}

	path := declaration.Document

	clean := filepath.Clean(path)
	if filepath.IsAbs(path) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("%s: %w: document must be relative to the Host project root", path, ErrInterfaceDocument)
	}

	data, err := disk.Read(filepath.Join(root, path))
	if err != nil {
		return fmt.Errorf("read Public interface document %s: %w", path, err)
	}

	err = parseInterfaceDocument(declaration, data, filepath.Join(root, path), root)
	if err != nil {
		return fmt.Errorf("parse Public interface document %s: %w", path, err)
	}

	return nil
}

func parseInterfaceDocument(declaration *PublicInterface, data []byte, path, root string) error {
	err := validateInterfaceYAML(data)
	if err != nil {
		return err
	}

	if declaration.Kind == kindHTTP {
		return parseHTTP(declaration, data, path, root)
	}

	return parseCLI(declaration, data)
}

func validateInterfaceYAML(data []byte) error {
	decoder := yaml.NewDecoder(bytes.NewReader(data))

	var document yaml.Node

	err := decoder.Decode(&document)
	if err != nil {
		return fmt.Errorf("YAML/JSON: %w", err)
	}

	if len(document.Content) == 0 || document.Content[0].Kind != yaml.MappingNode {
		return fmt.Errorf("%w: expected a YAML/JSON mapping", ErrInterfaceDocument)
	}

	var trailing yaml.Node

	err = decoder.Decode(&trailing)
	if errors.Is(err, io.EOF) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("YAML/JSON: %w", err)
	}

	return fmt.Errorf("%w: expected exactly one document", ErrInterfaceDocument)
}
