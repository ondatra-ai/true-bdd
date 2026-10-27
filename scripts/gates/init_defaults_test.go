package gates_test

import (
	"bytes"
	"fmt"
	"io/fs"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"gopkg.in/yaml.v3"
)

const initCopies = "../../services/true-bdd/internal/infrastructure/initialization/files"

func TestInitDefaultsMatchProjectFiles(t *testing.T) {
	t.Parallel()

	for _, tree := range []string{"true-bdd/checklists", "templates"} {
		t.Run(tree, func(t *testing.T) {
			t.Parallel()
			source := defaultTree(t, filepath.Join("../..", tree))

			copies := defaultTree(t, filepath.Join(initCopies, tree))
			for path, content := range source {
				copied, exists := copies[path]
				if !exists || !bytes.Equal(content, copied) {
					t.Errorf("init copy drift: %s/%s differs or is missing", tree, path)
				}

				delete(copies, path)
			}

			for path := range copies {
				t.Errorf("init copy drift: extra %s/%s", tree, path)
			}
		})
	}

	source := defaultConfig(t, "../../true-bdd/true-bdd.yaml")
	copies := defaultConfig(t, filepath.Join(initCopies, "true-bdd/true-bdd.yaml"))

	delete(source, "scripts")

	if _, exists := copies["scripts"]; exists {
		t.Error("init configuration contains scripts")
	}

	if !reflect.DeepEqual(source, copies) {
		t.Error("init configuration drift: must equal repository configuration less scripts")
	}
}

func defaultTree(t *testing.T, root string) map[string][]byte {
	t.Helper()

	files := make(map[string][]byte)

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("walk defaults: %w", walkErr)
		}

		if entry.IsDir() {
			return nil
		}

		relative, err := filepath.Rel(root, path)
		if err != nil {
			return fmt.Errorf("relative default: %w", err)
		}

		content, err := disk.Read(path)
		if err != nil {
			return fmt.Errorf("read default: %w", err)
		}

		files[relative] = content

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	return files
}

func defaultConfig(t *testing.T, path string) map[string]any {
	t.Helper()

	content, err := disk.Read(path)
	if err != nil {
		t.Fatal(err)
	}

	var config map[string]any

	err = yaml.Unmarshal(content, &config)
	if err != nil {
		t.Fatal(err)
	}

	return config
}
