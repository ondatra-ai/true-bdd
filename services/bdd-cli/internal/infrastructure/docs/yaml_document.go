package docs

import (
	"errors"
	"fmt"
	"io/fs"

	"gopkg.in/yaml.v3"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/disk"
)

// ReadYAML decodes the document at path into doc, naming it by label
// ("product document") in an error. found is false for an empty path or a
// missing file, which a host without that document has.
func ReadYAML(path, label string, doc any) (bool, error) {
	if path == "" {
		return false, nil
	}

	data, err := disk.Read(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("read %s %s: %w", label, path, err)
	}

	err = yaml.Unmarshal(data, doc)
	if err != nil {
		return false, fmt.Errorf("parse %s %s: %w", label, path, err)
	}

	return true, nil
}

// Absent reports whether a key of a decoded document was left out or set
// to null.
func Absent(node *yaml.Node) bool {
	return node.Kind == 0 || node.Tag == "!!null"
}
