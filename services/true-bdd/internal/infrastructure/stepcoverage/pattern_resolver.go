package stepcoverage

import "go/ast"

// PatternResolver folds registration patterns using the package's string constants.
type PatternResolver struct {
	constants constTable
}

// NewPatternResolver indexes the parsed package's explicit constant values.
func NewPatternResolver(files []*ast.File) PatternResolver {
	return PatternResolver{constants: newConstTable(files)}
}

// Resolve reads a string literal, concatenation or constant reference.
// Expressions the scanner cannot fold remain unresolved.
func (r PatternResolver) Resolve(expr ast.Expr) (string, bool) {
	return r.constants.fold(expr, map[string]bool{})
}
