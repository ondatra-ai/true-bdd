package gates_test

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	gitcli "github.com/ondatra-ai/true-bdd/pkg/cli/git"
)

// The Test gate checks Git and Go subprocesses even in build-tagged files (#206).
func TestRawGitCallersScrubRepositoryEnvironment(t *testing.T) {
	root := gitOutput(t, ".", "rev-parse", "--show-toplevel")
	t.Chdir(root)

	paths, err := gitcli.ListedFiles()
	if err != nil {
		t.Fatal(err)
	}

	for _, path := range paths {
		if !strings.HasSuffix(path, ".go") || strings.HasPrefix(path, "tests/bdd-cli/fixtures/") ||
			strings.Contains(path, "/testdata/") {
			continue
		}

		checkGitSource(t, path)
	}
}

func checkGitSource(t *testing.T, path string) {
	t.Helper()

	tree, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if errors.Is(err, fs.ErrNotExist) {
		return
	}

	if err != nil {
		t.Fatal(err)
	}

	for _, declaration := range tree.Decls {
		function, matched := declaration.(*ast.FuncDecl)
		if !matched || function.Body == nil {
			continue
		}

		ast.Inspect(function.Body, func(node ast.Node) bool {
			call, matched := node.(*ast.CallExpr)
			if !matched || !rawGitCommand(call, tree) {
				return true
			}

			variable := commandVariable(function.Body, call)

			environment := commandEnvironment(function.Body, variable)
			if !containsScrubber(environment, tree) {
				t.Errorf("%s: %s spawns git or go without git.ScrubEnvironment", path, function.Name.Name)
			}

			return true
		})
	}
}

func rawGitCommand(call *ast.CallExpr, tree *ast.File) bool {
	selector, matched := call.Fun.(*ast.SelectorExpr)
	if !matched || importPath(selector.X, tree) != "os/exec" {
		return false
	}

	argument := 0

	switch selector.Sel.Name {
	case "CommandContext":
		argument = 1
	case "Command":
	default:
		return false
	}

	if len(call.Args) <= argument {
		return false
	}

	binary := filepath.Base(commandProgram(call.Args[argument], tree))

	return binary == gitcli.Bin || binary == "go"
}

func commandProgram(expression ast.Expr, tree *ast.File) string {
	switch expression := expression.(type) {
	case *ast.BasicLit:
		if expression.Kind == token.STRING {
			value, _ := strconv.Unquote(expression.Value)

			return value
		}
	case *ast.Ident:
		if expression.Obj != nil {
			return commandProgram(declaredValue(expression.Obj.Decl), tree)
		}
	case *ast.CallExpr:
		return lookedUpProgram(expression, tree)
	default:
	}

	return ""
}

func lookedUpProgram(call *ast.CallExpr, tree *ast.File) string {
	if len(call.Args) != 1 {
		return ""
	}

	selector, matched := call.Fun.(*ast.SelectorExpr)
	if matched && selector.Sel.Name == "LookPath" && importPath(selector.X, tree) == "os/exec" {
		return commandProgram(call.Args[0], tree)
	}

	return ""
}

func declaredValue(declaration any) ast.Expr {
	switch declaration := declaration.(type) {
	case *ast.AssignStmt:
		if len(declaration.Rhs) == 1 {
			return declaration.Rhs[0]
		}
	case *ast.ValueSpec:
		if len(declaration.Values) == 1 {
			return declaration.Values[0]
		}
	default:
	}

	return nil
}

func commandVariable(body *ast.BlockStmt, call *ast.CallExpr) string {
	variable := ""

	ast.Inspect(body, func(node ast.Node) bool {
		assignment, matched := node.(*ast.AssignStmt)
		if !matched || len(assignment.Rhs) != 1 || len(assignment.Lhs) != 1 {
			return true
		}

		if assignment.Rhs[0] == call {
			identifier, matched := assignment.Lhs[0].(*ast.Ident)
			if matched {
				variable = identifier.Name
			}
		}

		return true
	})

	return variable
}

func commandEnvironment(body *ast.BlockStmt, variable string) ast.Expr {
	var environment ast.Expr

	ast.Inspect(body, func(node ast.Node) bool {
		assignment, matched := node.(*ast.AssignStmt)
		if !matched || len(assignment.Lhs) != 1 || len(assignment.Rhs) != 1 {
			return true
		}

		field, matched := assignment.Lhs[0].(*ast.SelectorExpr)
		if !matched || field.Sel.Name != "Env" {
			return true
		}

		receiver, matched := field.X.(*ast.Ident)
		if matched && receiver.Name == variable {
			environment = assignment.Rhs[0]
		}

		return true
	})

	return environment
}

func containsScrubber(expression ast.Expr, tree *ast.File) bool {
	if expression == nil {
		return false
	}

	if identifier, matched := expression.(*ast.Ident); matched && identifier.Obj != nil {
		return containsScrubber(declaredValue(identifier.Obj.Decl), tree)
	}

	scrubbed := false

	ast.Inspect(expression, func(node ast.Node) bool {
		call, matched := node.(*ast.CallExpr)
		if !matched {
			return true
		}

		selector, matched := call.Fun.(*ast.SelectorExpr)
		if matched && selector.Sel.Name == "ScrubEnvironment" &&
			isGitScrubber(importPath(selector.X, tree)) {
			scrubbed = true
		}

		return true
	})

	return scrubbed
}

func importPath(expression ast.Expr, tree *ast.File) string {
	identifier, matched := expression.(*ast.Ident)
	if !matched {
		return ""
	}

	for _, spec := range tree.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}

		name := filepath.Base(path)
		if spec.Name != nil {
			name = spec.Name.Name
		}

		if name == identifier.Name {
			return path
		}
	}

	return ""
}

func isGitScrubber(path string) bool {
	return path == "github.com/ondatra-ai/true-bdd/pkg/cli/git" ||
		path == "github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/cli/git"
}
