// Package verification selects complete evidence units referenced by step definitions.
package verification

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
)

type sourceDeclaration struct {
	file  string
	code  string
	node  ast.Node
	names []string
}

// Sources follows local declarations conservatively by identifier, including helpers and State.
func Sources(suite string, functions []string) (string, error) {
	declarations, err := sourceDeclarations(suite)
	if err != nil {
		return "", err
	}

	wanted := make(map[string]bool)
	for _, name := range append(slices.Clone(functions), "State") {
		wanted[name] = true
	}

	selected := make(map[int]bool)

	for changed := true; changed; {
		changed = false

		for index, declaration := range declarations {
			if selected[index] || !declaration.used(wanted) {
				continue
			}

			selected[index], changed = true, true

			for name := range Identifiers(declaration.node) {
				wanted[name] = true
			}
		}
	}

	var result strings.Builder

	for index, declaration := range declarations {
		if selected[index] {
			fmt.Fprintf(&result, "\n--- %s ---\n%s\n", declaration.file, declaration.code)
		}
	}

	return result.String(), nil
}

// Identifiers names referenced syntax, excluding comments and string literals.
func Identifiers(node ast.Node) map[string]bool {
	names := make(map[string]bool)

	ast.Inspect(node, func(child ast.Node) bool {
		if identifier, ok := child.(*ast.Ident); ok {
			names[identifier.Name] = true
		}

		return true
	})

	return names
}

func (declaration sourceDeclaration) used(names map[string]bool) bool {
	for _, name := range declaration.names {
		if names[name] {
			return true
		}
	}

	return false
}

func sourceDeclarations(suite string) ([]sourceDeclaration, error) {
	paths, err := filepath.Glob(filepath.Join(suite, "*.go"))
	if err != nil {
		return nil, fmt.Errorf("list definition source: %w", err)
	}

	helpers, err := helperSourcePaths(filepath.Join(filepath.Dir(suite), "helpers"))
	if err != nil {
		return nil, err
	}

	paths = append(paths, helpers...)

	slices.Sort(paths)

	declarations := make([]sourceDeclaration, 0)

	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}

		found, readErr := readDeclarations(path)
		if readErr != nil {
			return nil, readErr
		}

		declarations = append(declarations, found...)
	}

	return declarations, nil
}

func readDeclarations(path string) ([]sourceDeclaration, error) {
	content, err := disk.Read(path)
	if err != nil {
		return nil, fmt.Errorf("read definition source: %w", err)
	}

	positions := token.NewFileSet()

	tree, err := parser.ParseFile(positions, path, content, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse definition source: %w", err)
	}

	declarations := make([]sourceDeclaration, 0, len(tree.Decls))
	for _, node := range tree.Decls {
		names, doc := declarationNames(node)
		if len(names) == 0 {
			continue
		}

		start := node.Pos()
		if doc != nil {
			start = doc.Pos()
		}

		code := string(content[positions.Position(start).Offset:positions.Position(node.End()).Offset])
		declarations = append(declarations, sourceDeclaration{path, code, node, names})
	}

	return append(declarations, registrationDeclarations(path, content, positions, tree)...), nil
}

func declarationNames(node ast.Decl) ([]string, *ast.CommentGroup) {
	switch declaration := node.(type) {
	case *ast.FuncDecl:
		return []string{declaration.Name.Name}, declaration.Doc
	case *ast.GenDecl:
		names := make([]string, 0)
		for _, spec := range declaration.Specs {
			names = append(names, specificationNames(spec)...)
		}

		return names, declaration.Doc
	}

	return nil, nil
}

func specificationNames(spec ast.Spec) []string {
	switch spec := spec.(type) {
	case *ast.TypeSpec:
		return []string{spec.Name.Name}
	case *ast.ValueSpec:
		names := make([]string, 0, len(spec.Names))
		for _, name := range spec.Names {
			if name.Name != "_" {
				names = append(names, name.Name)
			}
		}

		return names
	case *ast.ImportSpec:
		name := filepath.Base(strings.Trim(spec.Path.Value, `"`))
		if spec.Name != nil {
			name = spec.Name.Name
		}

		return []string{name}
	}

	return nil
}

func registrationDeclarations(
	path string, content []byte, positions *token.FileSet, tree *ast.File,
) []sourceDeclaration {
	declarations := make([]sourceDeclaration, 0)

	ast.Inspect(tree, func(node ast.Node) bool {
		call, isCall := node.(*ast.CallExpr)
		if !isCall || len(call.Args) != 2 {
			return true
		}

		selector, ok := call.Fun.(*ast.SelectorExpr)

		function, named := call.Args[1].(*ast.Ident)
		if !ok || selector.Sel.Name != "Step" || !named {
			return true
		}

		code := string(content[positions.Position(call.Pos()).Offset:positions.Position(call.End()).Offset])
		declarations = append(declarations, sourceDeclaration{path, code, call, []string{function.Name}})

		return true
	})

	return declarations
}

func helperSourcePaths(helpers string) ([]string, error) {
	paths := make([]string, 0)

	err := filepath.WalkDir(helpers, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if path == helpers && errors.Is(walkErr, fs.ErrNotExist) {
				return nil
			}

			return walkErr
		}

		if entry.Type().IsRegular() && strings.HasSuffix(path, ".go") {
			paths = append(paths, path)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list helper source: %w", err)
	}

	return paths, nil
}
