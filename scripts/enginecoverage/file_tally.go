package enginecoverage

// fileTally walks one file's blocks in source order. A function's blocks are
// adjacent in that order, its func literals' included, so a change of
// enclosing declaration closes the function before it.
type fileTally struct {
	pkg   *Package
	file  string
	funcs []declaredFunc
	// index is the declaration current belongs to, -1 for package level.
	index   int
	current *Function
	// open is whether the previous block of current was unrun, so the next
	// unrun one extends its span instead of starting another.
	open bool
}

func (w *fileTally) add(each block) {
	w.pkg.Statements += each.statements
	if each.count > 0 {
		w.pkg.Covered += each.statements
	}

	index := enclosing(w.funcs, each.start)
	if w.current == nil || index != w.index {
		w.flush()
		w.start(index)
	}

	w.current.Statements += each.statements

	switch {
	case each.count > 0:
		w.open = false
	case w.open:
		last := &w.current.Uncovered[len(w.current.Uncovered)-1]
		last.LastLine = each.end.line
		last.Statements += each.statements
		last.Blocks++
	default:
		w.current.Uncovered = append(w.current.Uncovered, Span{
			FirstLine: each.start.line, LastLine: each.end.line,
			Statements: each.statements, Blocks: 1,
		})
		w.open = true
	}
}

// start opens the function the declaration at index names.
func (w *fileTally) start(index int) {
	name := packageLevel
	if index >= 0 {
		name = w.funcs[index].name
	}

	w.index = index
	w.current = &Function{Name: name, File: w.file}
	w.open = false
}

// flush keeps the current function when it holds an unrun block.
func (w *fileTally) flush() {
	if w.current != nil && len(w.current.Uncovered) > 0 {
		w.pkg.Functions = append(w.pkg.Functions, *w.current)
	}

	w.current = nil
}
