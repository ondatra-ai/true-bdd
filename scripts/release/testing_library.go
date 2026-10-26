package release

import (
	"fmt"
	"path/filepath"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"gopkg.in/yaml.v3"
)

func declareTestingLibrary(host, module, version string) error {
	path := filepath.Join(host, "docs/architecture/architecture.yaml")

	data, err := disk.Read(path)
	if err != nil {
		return fmt.Errorf("read host architecture: %w", err)
	}

	var document yaml.Node

	err = yaml.Unmarshal(data, &document)
	if err != nil {
		return fmt.Errorf("decode host architecture: %w", err)
	}

	libraries := mappingValue(mappingValue(mappingValue(document.Content[0], "architecture"), "testing"), "libraries")
	if libraries == nil || libraries.Kind != yaml.SequenceNode {
		return fmt.Errorf("%w: host testing.libraries missing", errRefused)
	}

	for _, library := range libraries.Content {
		name, pin := mappingValue(library, "module"), mappingValue(library, "version")
		if name != nil && pin != nil && name.Value == module {
			pin.Value = version

			return writeHostArchitecture(path, &document)
		}
	}

	var library yaml.Node

	err = library.Encode(map[string]string{"module": module, "version": version})
	if err != nil {
		return fmt.Errorf("encode testing library: %w", err)
	}

	libraries.Content = append(libraries.Content, &library)

	return writeHostArchitecture(path, &document)
}

func mappingValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}

	for index := 0; index+1 < len(node.Content); index += 2 {
		if node.Content[index].Value == key {
			return node.Content[index+1]
		}
	}

	return nil
}

func writeHostArchitecture(path string, document *yaml.Node) error {
	data, err := yaml.Marshal(document)
	if err == nil {
		err = disk.Write(path, data, disk.Private)
	}

	if err != nil {
		return fmt.Errorf("write host architecture: %w", err)
	}

	return nil
}
