package gates_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestRuntimeKnowsNoProduct(t *testing.T) {
	t.Parallel()

	roots := []string{"pkg/testkit", "pkg/proxy/cli", "utilities/cliproxy"}
	for _, root := range roots {
		err := filepath.WalkDir(filepath.Join("../..", root), func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}

			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}

			inspectRuntimeFile(t, path)

			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

// Framework configuration and vendor rules stay; these literals encode the engine-specific findings in research #293.
func inspectRuntimeFile(t *testing.T, path string) {
	t.Helper()

	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}

	forbidden := []string{"templates", "services/calc", "true-bdd.log.json", "codex-homes", "codex-shell-home",
		"codex-go-build", "codex-tmp", "library-download", "us-refine", "us-apply", "scen-check", "build-steps", "build-code",
		`\d{4}-\d{2}-\d{2}-\d{2}-\d{2}-\d{2}-\d+`}

	ast.Inspect(file, func(node ast.Node) bool {
		literal, ok := node.(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			return true
		}

		value, decodeErr := strconv.Unquote(literal.Value)
		if decodeErr != nil {
			t.Fatal(decodeErr)
		}

		for _, word := range forbidden {
			if strings.Contains(value, word) {
				t.Errorf("%s holds engine-specific literal %q", path, value)
			}
		}

		return true
	})
}

func TestStepDefinitionsUseRootAPI(t *testing.T) {
	t.Parallel()

	for _, directory := range []string{"steps", "helpers"} {
		root := filepath.Join("../../tests/true-bdd", directory)

		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}

			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}

			inspectStepImports(t, path)

			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func inspectStepImports(t *testing.T, path string) {
	t.Helper()

	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}

	const module = "github.com/ondatra-ai/true-bdd/"

	for _, imported := range file.Imports {
		name, decodeErr := strconv.Unquote(imported.Path.Value)
		if decodeErr != nil {
			t.Fatal(decodeErr)
		}

		if strings.HasPrefix(name, module) && name != module+"pkg/testkit" && name != module+"pkg/proxy/cli" &&
			name != module+"tests/true-bdd/helpers" {
			t.Errorf("%s imports %s: Step definitions use the testkit root and CLI SDK", path, name)
		}
	}
}
