package stepskeleton

import (
	"errors"
	"fmt"
	"go/ast"
	"go/token"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
)

// ErrNotEngineSkeleton refuses to remove a definition the engine did not write alone in its file.
var ErrNotEngineSkeleton = errors.New("only an engine-written concept skeleton can be rewritten")

// RemoveConcept deletes a concept skeleton the engine wrote, its file and
// its registration, so a new pattern of the concept can take its place.
func RemoveConcept(dir string, definition Definition) error {
	files, err := readPackage(dir)
	if err != nil {
		return err
	}

	for _, file := range files {
		if file.path != definition.File {
			continue
		}

		for _, declaration := range file.tree.Decls {
			if !skeletonDeclaration(declaration, definition.Function) {
				return fmt.Errorf("%w: %s holds more than %s", ErrNotEngineSkeleton, file.path, definition.Function)
			}
		}
	}

	err = unregister(files, definition)
	if err != nil {
		return err
	}

	err = disk.Remove(definition.File)
	if err != nil {
		return fmt.Errorf("remove skeleton: %w", err)
	}

	return nil
}

// skeletonDeclaration is what an engine skeleton's file holds: its imports and its function.
func skeletonDeclaration(declaration ast.Decl, function string) bool {
	if fun, ok := declaration.(*ast.FuncDecl); ok {
		return fun.Recv == nil && fun.Name.Name == function
	}

	general, ok := declaration.(*ast.GenDecl)

	return ok && general.Tok == token.IMPORT
}

// unregister drops the literal registration of the definition's pattern and function.
func unregister(files []sourceFile, definition Definition) error {
	for _, file := range files {
		for _, fun := range packageFunctions([]sourceFile{file}) {
			if fun.Name.Name != registerFunction || fun.Body == nil {
				continue
			}

			for _, statement := range fun.Body.List {
				expression, ok := statement.(*ast.ExprStmt)
				if !ok {
					continue
				}

				pattern, function, isStep := stepCall(expression.X)
				if !isStep || pattern != definition.Pattern || function != definition.Function {
					continue
				}

				start := file.positions.Position(statement.Pos()).Offset
				end := file.positions.Position(statement.End()).Offset
				source := append(append([]byte(nil), file.source[:start]...), file.source[end:]...)

				return writeGo(file.path, source)
			}
		}
	}

	return fmt.Errorf("%w: no registration of %s", ErrSkeletonChanged, definition.Function)
}
