package stepskeleton

import (
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"path"
	"strconv"
	"strings"
)

const testkitImport = "github.com/ondatra-ai/true-bdd/pkg/testkit"

var errStateStruct = errors.New("harness requires State to be a struct")

// EnsureScenarioRun embeds the harness's Scenario run while keeping definition-owned fields.
// Skeleton rewrites and the scenario-run Checklist item's F call it.
func EnsureScenarioRun(dir string) error {
	files, err := readPackage(dir)
	if err != nil {
		return err
	}

	file, state, err := scenarioRunState(files)
	if err != nil {
		return err
	}

	if state != nil {
		return embedScenarioRun(file, state)
	}

	source := fmt.Sprintf("package steps\n\nimport %q\n\ntype State struct {\n *testkit.ScenarioRun\n}\n", testkitImport)

	return writeGo(unusedSkeletonFile(dir, "state.go"), []byte(source))
}

func embedScenarioRun(file sourceFile, state *ast.StructType) error {
	qualifier, imported := scenarioRunQualifier(file.tree)

	if imported && hasScenarioRun(state, qualifier) {
		return nil
	}

	offset := file.positions.Position(state.Fields.Opening).Offset + 1

	source := string(file.source[:offset]) + "\n*" + qualifier + "ScenarioRun\n" + string(file.source[offset:])
	if !imported {
		source = addTestkitImport(file, source, qualifier)
	}

	return writeGo(file.path, []byte(source))
}

func scenarioRunQualifier(tree *ast.File) (string, bool) {
	for _, imported := range tree.Imports {
		path, err := strconv.Unquote(imported.Path.Value)
		if err != nil || path != testkitImport {
			continue
		}

		if imported.Name == nil {
			return "testkit.", true
		}

		if imported.Name.Name == "." {
			return "", true
		}

		return imported.Name.Name + ".", true
	}

	return unusedTestkitQualifier(tree), false
}

func scenarioRunType(expression ast.Expr, qualifier string) bool {
	if selector, ok := expression.(*ast.SelectorExpr); ok {
		name, named := selector.X.(*ast.Ident)

		return named && name.Name+"." == qualifier && selector.Sel.Name == "ScenarioRun"
	}

	name, ok := expression.(*ast.Ident)

	return ok && qualifier == "" && name.Name == "ScenarioRun"
}

func unusedTestkitQualifier(tree *ast.File) string {
	names := make(map[string]bool)

	for _, imported := range tree.Imports {
		importedPath, _ := strconv.Unquote(imported.Path.Value)

		name := path.Base(importedPath)
		if imported.Name != nil {
			name = imported.Name.Name
		}

		names[name] = true
	}

	name := "testkit"
	for suffix := 1; names[name]; suffix++ {
		name = "testkit" + strconv.Itoa(suffix)
	}

	return name + "."
}

func addTestkitImport(file sourceFile, source, qualifier string) string {
	declarationText := strconv.Quote(testkitImport)
	if qualifier != "testkit." {
		declarationText = strings.TrimSuffix(qualifier, ".") + " " + declarationText
	}

	for _, declaration := range file.tree.Decls {
		general, ok := declaration.(*ast.GenDecl)
		if !ok || general.Tok != token.IMPORT {
			continue
		}

		if general.Lparen.IsValid() {
			offset := file.positions.Position(general.Lparen).Offset + 1

			return source[:offset] + fmt.Sprintf("\n%s\n", declarationText) + source[offset:]
		}

		start := file.positions.Position(general.TokPos).Offset
		end := file.positions.Position(general.End()).Offset

		imports := fmt.Sprintf("import (\n%s\n%s\n)", declarationText, source[start+len("import"):end])

		return source[:start] + imports + source[end:]
	}

	offset := file.positions.Position(file.tree.Name.End()).Offset

	return source[:offset] + fmt.Sprintf("\n\nimport %s\n", declarationText) + source[offset:]
}

// InspectScenarioRun reads State's embedded harness field and the file the fix would edit.
// An absent State or directory returns the new State file without creating it.
func InspectScenarioRun(dir string) (string, bool, error) {
	files, err := readPackage(dir)
	if err != nil {
		return "", false, err
	}

	file, state, err := scenarioRunState(files)
	if err != nil {
		return file.path, false, err
	}

	if state == nil {
		return unusedSkeletonFile(dir, "state.go"), false, nil
	}

	qualifier, imported := scenarioRunQualifier(file.tree)

	return file.path, imported && hasScenarioRun(state, qualifier), nil
}

func scenarioRunState(files []sourceFile) (sourceFile, *ast.StructType, error) {
	for _, file := range files {
		for _, declaration := range file.tree.Decls {
			general, ok := declaration.(*ast.GenDecl)
			if !ok {
				continue
			}

			for _, spec := range general.Specs {
				typed, isType := spec.(*ast.TypeSpec)
				if !isType || typed.Name.Name != stateTypeName {
					continue
				}

				state, isStruct := typed.Type.(*ast.StructType)
				if !isStruct {
					return file, nil, errStateStruct
				}

				return file, state, nil
			}
		}
	}

	return sourceFile{}, nil, nil
}

func hasScenarioRun(state *ast.StructType, qualifier string) bool {
	for _, field := range state.Fields.List {
		pointer, ok := field.Type.(*ast.StarExpr)
		if ok && len(field.Names) == 0 && scenarioRunType(pointer.X, qualifier) {
			return true
		}
	}

	return false
}
