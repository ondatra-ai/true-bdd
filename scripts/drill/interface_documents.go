package drill

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"gopkg.in/yaml.v3"
)

const architectureFile = "docs/architecture/architecture.yaml"

func interfaceDocuments(data []byte) (map[string]bool, error) {
	var document struct {
		Architecture struct {
			Services []struct {
				PublicInterface struct {
					Document string `yaml:"document"`
				} `yaml:"public_interface"`
			} `yaml:"services"`
		} `yaml:"architecture"`
	}

	err := yaml.Unmarshal(data, &document)
	if err != nil {
		return nil, fmt.Errorf("read architecture Public interfaces: %w", err)
	}

	documents := make(map[string]bool)

	for _, service := range document.Architecture.Services {
		path := service.PublicInterface.Document
		if path == "" {
			continue
		}

		clean := filepath.Clean(path)
		if filepath.IsAbs(path) || clean == "." || clean == ".." ||
			strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("%w: Public interface document %s must name a file inside the Host project", errRefused, path)
		}

		documents[filepath.ToSlash(clean)] = true
	}

	return documents, nil
}

func readInterfaceDocuments(tree string) (map[string]bool, error) {
	data, err := disk.Read(filepath.Join(tree, architectureFile))
	if err != nil {
		return nil, err
	}

	return interfaceDocuments(data)
}

func archiveInterfaceDocuments(archive []byte) (map[string]bool, error) {
	reader := tar.NewReader(bytes.NewReader(archive))
	for {
		header, err := reader.Next()
		if err != nil {
			return nil, fmt.Errorf("reading architecture from base archive: %w", err)
		}

		if header.Name != architectureFile {
			continue
		}

		data, err := io.ReadAll(reader)
		if err != nil {
			return nil, fmt.Errorf("reading archived architecture: %w", err)
		}

		return interfaceDocuments(data)
	}
}
