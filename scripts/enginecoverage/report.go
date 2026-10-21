package enginecoverage

import (
	"cmp"
	"path"
	"path/filepath"
	"slices"
)

// Report is the coverage of every counted package, least covered first.
type Report struct {
	Packages []Package
}

// Package is one package's totals and the functions holding a block no run
// reached.
type Package struct {
	// Path is the package directory from the module root, "." for the root.
	Path string
	// Statements and Covered count statements, as `go tool cover` does.
	Statements, Covered int
	// Functions hold an unrun block, in file then source order.
	Functions []Function
}

// Function is one function holding at least one block no run reached.
type Function struct {
	// Name is how `go doc` names it, or packageLevel.
	Name string
	// File is the file from the module root.
	File string
	// Statements is every statement the function's blocks hold.
	Statements int
	// Uncovered is each run of adjacent blocks no run reached.
	Uncovered []Span
}

// Span is a run of adjacent blocks of one function that no run reached.
type Span struct {
	FirstLine, LastLine int
	Statements, Blocks  int
}

// Statements is the statement count across every package.
func (r Report) Statements() int {
	total := 0
	for _, pkg := range r.Packages {
		total += pkg.Statements
	}

	return total
}

// Covered is the reached statement count across every package.
func (r Report) Covered() int {
	total := 0
	for _, pkg := range r.Packages {
		total += pkg.Covered
	}

	return total
}

// UncoveredBlocks is how many blocks no run reached.
func (r Report) UncoveredBlocks() int {
	total := 0

	for _, pkg := range r.Packages {
		for _, fn := range pkg.Functions {
			for _, span := range fn.Uncovered {
				total += span.Blocks
			}
		}
	}

	return total
}

// UncoveredStatements is the statements of every block no run reached.
func (fn Function) UncoveredStatements() int {
	total := 0
	for _, span := range fn.Uncovered {
		total += span.Statements
	}

	return total
}

// newReport groups the merged profile's blocks by package and function.
func newReport(blocks []block, module, root string) (Report, error) {
	byPath := map[string]*Package{}

	for start := 0; start < len(blocks); {
		end := start + 1
		for end < len(blocks) && blocks[end].file == blocks[start].file {
			end++
		}

		err := addFile(byPath, blocks[start:end], module, root)
		if err != nil {
			return Report{}, err
		}

		start = end
	}

	report := Report{Packages: make([]Package, 0, len(byPath))}
	for _, pkg := range byPath {
		report.Packages = append(report.Packages, *pkg)
	}

	slices.SortFunc(report.Packages, leastCoveredFirst)

	return report, nil
}

// addFile tallies one file's blocks, all from the same source file, into its
// package.
func addFile(byPath map[string]*Package, blocks []block, module, root string) error {
	rel, err := relativeFile(module, blocks[0].file)
	if err != nil {
		return err
	}

	funcs, err := declaredFuncs(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return err
	}

	dir := path.Dir(rel)

	pkg, found := byPath[dir]
	if !found {
		pkg = &Package{Path: dir}
		byPath[dir] = pkg
	}

	walk := fileTally{pkg: pkg, file: rel, funcs: funcs, index: -1}
	for _, each := range blocks {
		walk.add(each)
	}

	walk.flush()

	return nil
}

// leastCoveredFirst orders by coverage ratio, cross-multiplied so no float
// rounding ties two packages that differ.
func leastCoveredFirst(left, right Package) int {
	return cmp.Or(
		cmp.Compare(left.Covered*right.Statements, right.Covered*left.Statements),
		cmp.Compare(left.Path, right.Path))
}
