package gates_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/scripts/gates"
)

func TestModuleGatesRejectBrokenNestedModulesAndIgnoreFixtures(t *testing.T) {
	for _, name := range []string{"Build", "Test"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()

			for _, module := range []string{".", "pkg", "services/true-bdd"} {
				moduleName := "example.com/module" + strings.ReplaceAll(filepath.Base(module), ".", "root")
				write(t, root, filepath.Join(module, "go.mod"), "module "+moduleName+"\n\ngo 1.25.0\n")
				write(t, root, filepath.Join(module, "sample.go"), "package sample\n")
			}

			write(t, root, "tests/true-bdd/fixtures/go.mod", "deliberately invalid sentinel\n")
			t.Chdir(root)

			var selected gates.Gate

			for _, gate := range gates.All {
				if gate.Name == name {
					selected = gate
				}
			}

			err := gates.Run([]gates.Gate{selected})
			if err != nil {
				t.Fatalf("valid modules: %v", err)
			}

			for _, module := range []string{"pkg", "services/true-bdd"} {
				path := filepath.Join(module, "broken_test.go")
				body := "package sample\nimport \"testing\"\n" +
					"func TestBroken(t *testing.T) { t.Fatal(\"nested module must fail\") }\n"

				if name == "Build" {
					path = filepath.Join(module, "broken.go")
					body = "package sample\nvar _ = undefinedSymbol\n"
				}

				write(t, root, path, body)

				err := gates.Run([]gates.Gate{selected})
				if err == nil {
					t.Errorf("%s passed broken %s", name, module)
				}

				err = os.Remove(path)
				if err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
