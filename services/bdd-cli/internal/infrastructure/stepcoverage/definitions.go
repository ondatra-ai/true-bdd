package stepcoverage

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

// stepMethod is the method a suite registers a definition through, and
// stepCallArgs its arity — a `.Step(` of any other arity is a different
// method, ignored rather than refused.
const (
	stepMethod   = "Step"
	stepCallArgs = 2
)

// ErrPatternNotConstant signals a pattern this scanner cannot fold to
// the string the suite compiles. Refused, never skipped: a skip reports
// every step it binds as unbound, and a fix turn pays for that gap.
var ErrPatternNotConstant = errors.New("step pattern is not a constant expression")

// ErrPatternInvalid signals a step pattern that does not compile. It
// fails the whole answer rather than shrinking the table, mirroring the
// suite, where one bad pattern is reported by every scenario.
var ErrPatternInvalid = errors.New("step pattern is not a valid regexp")

// definition is one registered step pattern and where it was written.
type definition struct {
	pattern *regexp.Regexp
	source  string
	where   string
}

// loadDefinitions reads every step definition a suite's steps/ package
// registers. An absent or empty directory is zero definitions rather
// than an error: a suite that has written none has every step unbound.
func loadDefinitions(dir string) ([]definition, error) {
	files, fset, err := parsePackage(dir)
	if err != nil || len(files) == 0 {
		return nil, err
	}

	table := newConstTable(files)

	var defs []definition

	for _, file := range files {
		found, defErr := definitionsIn(fset, file, table)
		if defErr != nil {
			return nil, defErr
		}

		defs = append(defs, found...)
	}

	return defs, nil
}

// parsePackage parses the package's non-test .go files, in name order.
// _test.go is skipped: the suite binary links the package without its
// test files, so a Step call there registers nothing.
func parsePackage(dir string) ([]*ast.File, *token.FileSet, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil, nil
		}

		return nil, nil, fmt.Errorf("read %s: %w", dir, err)
	}

	fset := token.NewFileSet()

	var files []*ast.File

	for _, entry := range entries {
		if !isDefinitionFile(entry) {
			continue
		}

		file, parseErr := parseOne(fset, filepath.Join(dir, entry.Name()))
		if parseErr != nil {
			return nil, nil, parseErr
		}

		files = append(files, file)
	}

	return files, fset, nil
}

// isDefinitionFile reports whether a directory entry is one of the
// package's compiled source files.
func isDefinitionFile(entry fs.DirEntry) bool {
	name := entry.Name()

	return !entry.IsDir() &&
		strings.HasSuffix(name, ".go") &&
		!strings.HasSuffix(name, "_test.go")
}

// parseOne parses one file. An unparseable file is an error, not a file
// to skip: the definitions it registers would otherwise vanish and every
// step they bind would read as a gap.
func parseOne(fset *token.FileSet, path string) (*ast.File, error) {
	source, err := disk.Read(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	file, err := parser.ParseFile(fset, path, source, 0)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	return file, nil
}

// definitionsIn collects one file's registrations, stopping at the first
// pattern it cannot fold.
func definitionsIn(fset *token.FileSet, file *ast.File, table constTable) ([]definition, error) {
	var (
		defs []definition
		err  error
	)

	ast.Inspect(file, func(node ast.Node) bool {
		if err != nil {
			return false
		}

		expr, found := stepPatternArg(node)
		if !found {
			return true
		}

		def, defErr := newDefinition(fset, expr, table)
		if defErr != nil {
			err = defErr

			return false
		}

		defs = append(defs, def)

		return true
	})

	return defs, err
}

// stepPatternArg reports the pattern argument of a `<recv>.Step(p, fn)`
// call, matching on shape rather than on the receiver's name — a host
// names its suite variable whatever it likes.
func stepPatternArg(node ast.Node) (ast.Expr, bool) {
	call, isCall := node.(*ast.CallExpr)
	if !isCall || len(call.Args) != stepCallArgs {
		return nil, false
	}

	selector, isSelector := call.Fun.(*ast.SelectorExpr)
	if !isSelector || selector.Sel.Name != stepMethod {
		return nil, false
	}

	_, isIdent := selector.X.(*ast.Ident)
	if !isIdent {
		return nil, false
	}

	return call.Args[0], true
}

// newDefinition folds one pattern argument and compiles it.
func newDefinition(fset *token.FileSet, expr ast.Expr, table constTable) (definition, error) {
	where := fset.Position(expr.Pos()).String()

	source, folded := table.fold(expr, map[string]bool{})
	if !folded {
		return definition{}, fmt.Errorf("%s: %w: %s",
			where, ErrPatternNotConstant, render(fset, expr))
	}

	compiled, err := regexp.Compile(source)
	if err != nil {
		return definition{}, fmt.Errorf("%s: %w: %q (%w)",
			where, ErrPatternInvalid, source, err)
	}

	return definition{pattern: compiled, source: source, where: where}, nil
}

// render prints an expression as it was written, for the refusal to name.
func render(fset *token.FileSet, expr ast.Expr) string {
	var buf bytes.Buffer

	err := format.Node(&buf, fset, expr)
	if err != nil {
		return "<unprintable expression>"
	}

	return buf.String()
}
