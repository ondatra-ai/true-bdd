package stepskeleton

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// ErrPending identifies a body that still contains the engine's placeholder.
var ErrPending = errors.New("step not written yet")

// Shape retains everything except imports, State, and the selected function bodies.
func Shape(dir string, functions ...string) (string, error) {
	files, err := readPackage(dir)
	if err != nil {
		return "", err
	}

	var shape bytes.Buffer

	for _, file := range files {
		for _, declaration := range file.tree.Decls {
			declaration = shapeDeclaration(declaration)
			if declaration == nil {
				continue
			}

			if fun, ok := declaration.(*ast.FuncDecl); ok && slices.Contains(functions, fun.Name.Name) && fun.Recv == nil {
				fun.Body = nil
			}

			err := format.Node(&shape, token.NewFileSet(), declaration)
			if err != nil {
				return "", fmt.Errorf("format skeleton shape: %w", err)
			}
		}
	}

	return shape.String(), nil
}

// Written requires the selected function's body, without the pending sentinel.
func (d Definition) Written() error {
	files, err := readPackage(filepath.Dir(d.File))
	if err != nil {
		return err
	}

	for _, file := range files {
		for _, declaration := range file.tree.Decls {
			fun, ok := declaration.(*ast.FuncDecl)
			if !ok || fun.Name.Name != d.Function || fun.Recv != nil {
				continue
			}

			if pendingBody(fun.Body) {
				return ErrPending
			}

			return nil
		}
	}

	return ErrPending
}

func shapeDeclaration(declaration ast.Decl) ast.Decl {
	general, ok := declaration.(*ast.GenDecl)
	if !ok {
		return declaration
	}

	if general.Tok == token.IMPORT {
		return nil
	}

	specs := make([]ast.Spec, 0, len(general.Specs))
	for _, spec := range general.Specs {
		if typed, ok := spec.(*ast.TypeSpec); ok && typed.Name.Name == "State" {
			continue
		}

		specs = append(specs, spec)
	}

	if len(specs) == 0 {
		return nil
	}

	general.Specs = specs

	return general
}

func pendingBody(body *ast.BlockStmt) bool {
	if body == nil || len(body.List) == 0 {
		return true
	}

	pending := false

	ast.Inspect(body, func(node ast.Node) bool {
		literal, ok := node.(*ast.BasicLit)
		if ok && literal.Kind == token.STRING {
			value, _ := strconv.Unquote(literal.Value)
			pending = pending || strings.Contains(value, ErrPending.Error())
		}

		return true
	})

	return pending
}
