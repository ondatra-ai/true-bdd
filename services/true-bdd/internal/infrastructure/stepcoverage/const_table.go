package stepcoverage

import (
	"go/ast"
	"go/token"
	"strconv"
)

// constTable holds a package's constants by name, so a pattern spliced
// from one folds to the string the suite would compile.
type constTable map[string]ast.Expr

// newConstTable indexes every `const` carrying an explicit value. A spec
// with none (iota, implicit repetition) is left out, so a reference to
// one fails to fold rather than folding to the wrong thing.
func newConstTable(files []*ast.File) constTable {
	table := constTable{}

	for _, file := range files {
		for _, decl := range file.Decls {
			generic, isGeneric := decl.(*ast.GenDecl)
			if !isGeneric || generic.Tok != token.CONST {
				continue
			}

			table.add(generic)
		}
	}

	return table
}

// add records one const declaration's named values.
func (t constTable) add(generic *ast.GenDecl) {
	for _, spec := range generic.Specs {
		value, isValue := spec.(*ast.ValueSpec)
		if !isValue || len(value.Values) != len(value.Names) {
			continue
		}

		for index, name := range value.Names {
			t[name.Name] = value.Values[index]
		}
	}
}

// fold resolves a constant expression to the string it denotes. seen
// guards the recursion: a const cycle is illegal Go but constructible,
// and must terminate rather than exhaust the stack.
func (t constTable) fold(expr ast.Expr, seen map[string]bool) (string, bool) {
	switch node := expr.(type) {
	case *ast.BasicLit:
		return unquote(node)

	case *ast.ParenExpr:
		return t.fold(node.X, seen)

	case *ast.BinaryExpr:
		return t.foldBinary(node, seen)

	case *ast.Ident:
		return t.foldIdent(node, seen)

	default:
		return "", false
	}
}

// foldBinary folds a concatenation. Any other operator is refused: none
// of them yields a string a suite could have registered.
func (t constTable) foldBinary(node *ast.BinaryExpr, seen map[string]bool) (string, bool) {
	if node.Op != token.ADD {
		return "", false
	}

	left, leftFolded := t.fold(node.X, seen)
	if !leftFolded {
		return "", false
	}

	right, rightFolded := t.fold(node.Y, seen)
	if !rightFolded {
		return "", false
	}

	return left + right, true
}

// foldIdent folds a reference to a package-level string const. A var is
// refused along with everything else the table does not hold: it can be
// assigned after init, so its source value is not the registered one.
func (t constTable) foldIdent(node *ast.Ident, seen map[string]bool) (string, bool) {
	value, known := t[node.Name]
	if !known || seen[node.Name] {
		return "", false
	}

	seen[node.Name] = true
	defer delete(seen, node.Name)

	return t.fold(value, seen)
}

// unquote reads a string literal's value; any other kind of literal is
// not a pattern.
func unquote(node *ast.BasicLit) (string, bool) {
	if node.Kind != token.STRING {
		return "", false
	}

	text, err := strconv.Unquote(node.Value)
	if err != nil {
		return "", false
	}

	return text, true
}
