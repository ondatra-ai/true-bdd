package stepskeleton

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
)

type sourceFile struct {
	path      string
	source    []byte
	tree      *ast.File
	positions *token.FileSet
}

func readPackage(dir string) ([]sourceFile, error) {
	return readPackageMode(dir, 0)
}

func readPackageMode(dir string, mode parser.Mode) ([]sourceFile, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("read skeleton directory: %w", err)
	}

	files := make([]sourceFile, 0)

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}

		path := filepath.Join(dir, entry.Name())

		data, err := disk.Read(path)
		if err != nil {
			return nil, fmt.Errorf("read steps: %w", err)
		}

		positions := token.NewFileSet()

		tree, err := parser.ParseFile(positions, path, data, mode)
		if err != nil {
			return nil, fmt.Errorf("parse steps: %w", err)
		}

		files = append(files, sourceFile{path, data, tree, positions})
	}

	return files, nil
}

func ensureState(dir, packageName string, files []sourceFile) error {
	for _, file := range files {
		for _, declaration := range file.tree.Decls {
			general, ok := declaration.(*ast.GenDecl)
			if !ok {
				continue
			}

			for _, spec := range general.Specs {
				if typed, ok := spec.(*ast.TypeSpec); ok && typed.Name.Name == "State" {
					return nil
				}
			}
		}
	}

	return writeGo(unusedSkeletonFile(dir, "state.go"), []byte("package "+packageName+"\n\ntype State struct{}\n"))
}

func register(dir, packageName, function, pattern string, files []sourceFile) error {
	for _, file := range files {
		for _, declaration := range file.tree.Decls {
			fun, ok := declaration.(*ast.FuncDecl)
			if !ok || fun.Name.Name != registerFunction || fun.Recv != nil {
				continue
			}

			if fun.Body == nil || len(fun.Type.Params.List) != 1 || len(fun.Type.Params.List[0].Names) != 1 {
				return errRegisterShape
			}

			statement := fmt.Sprintf("\n%s.Step(%q, %s)\n", fun.Type.Params.List[0].Names[0].Name, pattern, function)
			offset := file.positions.Position(fun.Body.Rbrace).Offset
			source := append([]byte(nil), file.source[:offset]...)
			source = append(source, []byte(statement)...)
			source = append(source, file.source[offset:]...)

			return writeGo(file.path, source)
		}
	}

	source := fmt.Sprintf(`package %s
func Register(suite interface { Step(string, func(*State, []string) error) }) {
 suite.Step(%q, %s)
}
`, packageName, pattern, function)

	return writeGo(unusedSkeletonFile(dir, "register.go"), []byte(source))
}

// registerFunction is the steps package's registration entry point.
const registerFunction = "Register"

var errRegisterShape = errors.New("registration function must take one named suite parameter")

func unusedSkeletonFile(dir, name string) string {
	target := filepath.Join(dir, name)

	_, err := os.Stat(target)
	if err == nil {
		return filepath.Join(dir, "true_bdd_"+name)
	}

	return target
}
