package enginecoverage

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

// packageLevel names the blocks outside every func declaration: a func
// literal assigned to a package-level var.
const packageLevel = "(package level)"

// declaredFunc is one top-level func declaration and the span it covers.
type declaredFunc struct {
	name       string
	start, end position
}

// declaredFuncs lists the file's func declarations in source order.
func declaredFuncs(path string) ([]declaredFunc, error) {
	src, err := disk.Read(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	files := token.NewFileSet()

	parsed, err := parser.ParseFile(files, path, src, parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	var funcs []declaredFunc

	for _, decl := range parsed.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}

		start, end := files.Position(funcDecl.Pos()), files.Position(funcDecl.End())
		funcs = append(funcs, declaredFunc{
			name:  funcName(funcDecl),
			start: position{start.Line, start.Column},
			end:   position{end.Line, end.Column},
		})
	}

	return funcs, nil
}

// enclosing is the index of the func declaration holding at, or -1.
func enclosing(funcs []declaredFunc, at position) int {
	for index, declared := range funcs {
		if !at.before(declared.start) && at.before(declared.end) {
			return index
		}
	}

	return -1
}

// funcName is how `go doc` would name the function: `Greet`, `Greeter.Hello`,
// `(*Greeter).Farewell`.
func funcName(funcDecl *ast.FuncDecl) string {
	if funcDecl.Recv == nil || len(funcDecl.Recv.List) == 0 {
		return funcDecl.Name.Name
	}

	receiver := funcDecl.Recv.List[0].Type
	if star, ok := receiver.(*ast.StarExpr); ok {
		return "(*" + typeName(star.X) + ")." + funcDecl.Name.Name
	}

	return typeName(receiver) + "." + funcDecl.Name.Name
}

// typeName strips a generic receiver's parameters: `Set[K]` is `Set`.
func typeName(expr ast.Expr) string {
	switch typed := expr.(type) {
	case *ast.Ident:
		return typed.Name
	case *ast.IndexExpr:
		return typeName(typed.X)
	case *ast.IndexListExpr:
		return typeName(typed.X)
	case *ast.ParenExpr:
		return typeName(typed.X)
	default:
		return fmt.Sprintf("%T", expr)
	}
}
