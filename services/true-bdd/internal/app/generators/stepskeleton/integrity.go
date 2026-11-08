package stepskeleton

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/stepcoverage"
)

// ErrSkeletonChanged refuses a changed registration or function signature.
var ErrSkeletonChanged = errors.New("preserve the engine's registration and function signature")

// Intact checks the selected function's signature and its registration.
func (d Definition) Intact(dir string) error {
	files, err := readPackage(dir)
	if err != nil {
		return err
	}

	signature, registered := false, false
	resolver := patternResolver(files)

	for _, fun := range packageFunctions(files) {
		if fun.Name.Name == d.Function {
			signature, err = intactSignature(fun.Type)
			if err != nil {
				return err
			}
		}

		if fun.Name.Name == registerFunction {
			registered = registered || d.registered(fun, resolver)
		}
	}

	if !signature || !registered {
		return ErrSkeletonChanged
	}

	return nil
}

func (d Definition) registered(fun *ast.FuncDecl, resolver stepcoverage.PatternResolver) bool {
	found := false

	ast.Inspect(fun.Body, func(node ast.Node) bool {
		literal, function, isStep := stepCall(node, resolver)
		found = found || (isStep && literal == d.Pattern && function == d.Function)

		return true
	})

	return found
}

// stepCall reads a constant `suite.Step(pattern, function)` registration.
func stepCall(node ast.Node, resolver stepcoverage.PatternResolver) (string, string, bool) {
	call, isCall := node.(*ast.CallExpr)
	if !isCall || len(call.Args) != 2 {
		return "", "", false
	}

	selector, isSelector := call.Fun.(*ast.SelectorExpr)
	if !isSelector || selector.Sel.Name != "Step" {
		return "", "", false
	}

	_, isReceiver := selector.X.(*ast.Ident)
	if !isReceiver {
		return "", "", false
	}

	function, isFunction := call.Args[1].(*ast.Ident)
	if !isFunction {
		return "", "", false
	}

	literal, resolved := resolver.Resolve(call.Args[0])
	if !resolved {
		return "", "", false
	}

	return literal, function.Name, true
}

func patternResolver(files []sourceFile) stepcoverage.PatternResolver {
	trees := make([]*ast.File, 0, len(files))
	for _, file := range files {
		trees = append(trees, file.tree)
	}

	return stepcoverage.NewPatternResolver(trees)
}

func intactSignature(signature *ast.FuncType) (bool, error) {
	var source bytes.Buffer

	err := format.Node(&source, token.NewFileSet(), signature)
	if err != nil {
		return false, fmt.Errorf("read skeleton signature: %w", err)
	}

	return source.String() == "func(state *State, args []string) error", nil
}

func packageFunctions(files []sourceFile) []*ast.FuncDecl {
	functions := make([]*ast.FuncDecl, 0)

	for _, file := range files {
		for _, declaration := range file.tree.Decls {
			fun, ok := declaration.(*ast.FuncDecl)
			if ok && fun.Recv == nil {
				functions = append(functions, fun)
			}
		}
	}

	return functions
}
