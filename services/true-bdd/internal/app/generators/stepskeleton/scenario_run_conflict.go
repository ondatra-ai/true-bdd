package stepskeleton

import (
	"errors"
	"fmt"
	"go/ast"
)

// ErrScenarioRunConflict means the harness field name belongs to the definitions.
var ErrScenarioRunConflict = errors.New("definition-owned State field ScenarioRun conflicts with the harness field")

func scenarioRunConflict(file sourceFile, state *ast.StructType) error {
	qualifier, imported := scenarioRunQualifier(file.tree)
	for _, field := range state.Fields.List {
		if imported && isScenarioRun(field, qualifier) {
			continue
		}

		if fieldNamedScenarioRun(field) {
			return fmt.Errorf("%s: %w", file.path, ErrScenarioRunConflict)
		}
	}

	return nil
}

func fieldNamedScenarioRun(field *ast.Field) bool {
	for _, name := range field.Names {
		if name.Name == scenarioRunTypeName {
			return true
		}
	}

	if len(field.Names) != 0 {
		return false
	}

	return embeddedScenarioRunName(field.Type)
}

func embeddedScenarioRunName(expression ast.Expr) bool {
	switch typed := expression.(type) {
	case *ast.StarExpr:
		return embeddedScenarioRunName(typed.X)
	case *ast.IndexExpr:
		return embeddedScenarioRunName(typed.X)
	case *ast.IndexListExpr:
		return embeddedScenarioRunName(typed.X)
	case *ast.SelectorExpr:
		return typed.Sel.Name == scenarioRunTypeName
	case *ast.Ident:
		return typed.Name == scenarioRunTypeName
	default:
		return false
	}
}
