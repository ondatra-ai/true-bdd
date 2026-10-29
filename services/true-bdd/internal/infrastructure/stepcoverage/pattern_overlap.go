package stepcoverage

import (
	"fmt"
	"regexp/syntax"
	"slices"
	"unicode"
)

// patternState is a pair of NFA positions and the previous rune's boundary class.
type patternState struct {
	left, right uint32
	previous    rune
}

// patternsOverlap decides whether a whole Catalogue phrase and a definition
// can bind the same text. The finite alphabet splits at every rune range and
// word/newline boundary, preserving regexp assertions without sample values.
func patternsOverlap(phrase, definition string) (bool, error) {
	left, err := patternProgram(phrase)
	if err != nil {
		return false, err
	}

	right, err := patternProgram("(?s:.*?)(?:" + definition + ")(?s:.*)")
	if err != nil {
		return false, err
	}

	alphabet := patternAlphabet(left, right)
	queue := []patternState{{uint32(left.Start), uint32(right.Start), -1}}
	seen := make(map[patternState]bool)

	for len(queue) > 0 {
		state := queue[0]
		queue = queue[1:]

		if seen[state] {
			continue
		}

		seen[state] = true
		if programAccepts(left, state.left, state.previous) && programAccepts(right, state.right, state.previous) {
			return true, nil
		}

		for _, next := range alphabet {
			for _, lhs := range programStep(left, state.left, state.previous, next) {
				for _, rhs := range programStep(right, state.right, state.previous, next) {
					queue = append(queue, patternState{lhs, rhs, boundaryRune(next)})
				}
			}
		}
	}

	return false, nil
}

func patternProgram(pattern string) (*syntax.Prog, error) {
	parsed, err := syntax.Parse(pattern, syntax.Perl)
	if err != nil {
		return nil, fmt.Errorf("parse step pattern: %w", err)
	}

	program, err := syntax.Compile(parsed.Simplify())
	if err != nil {
		return nil, fmt.Errorf("compile step pattern: %w", err)
	}

	return program, nil
}

func patternAlphabet(programs ...*syntax.Prog) []rune {
	edges := []rune{
		0, '\n', '\n' + 1, '0', '9' + 1, 'A', 'Z' + 1, '_', '_' + 1, 'a', 'z' + 1,
		0xd800, 0xe000, unicode.MaxRune + 1,
	}

	for _, program := range programs {
		for _, inst := range program.Inst {
			edges = append(edges, instructionEdges(inst)...)
		}
	}

	slices.Sort(edges)
	edges = slices.Compact(edges)

	alphabet := make([]rune, 0, len(edges))
	for _, char := range edges {
		if char <= unicode.MaxRune && (char < 0xd800 || char >= 0xe000) {
			alphabet = append(alphabet, char)
		}
	}

	return alphabet
}

func programClosure(program *syntax.Prog, start uint32, previous, next rune) []uint32 {
	pending := []uint32{start}
	seen := make(map[uint32]bool)
	terminal := make([]uint32, 0)
	context := syntax.EmptyOpContext(previous, next)

	for len(pending) > 0 {
		pos := pending[len(pending)-1]
		pending = pending[:len(pending)-1]

		if seen[pos] {
			continue
		}

		seen[pos] = true

		inst := program.Inst[pos]
		switch inst.Op {
		case syntax.InstAlt, syntax.InstAltMatch:
			pending = append(pending, inst.Out, inst.Arg)
		case syntax.InstCapture, syntax.InstNop:
			pending = append(pending, inst.Out)
		case syntax.InstEmptyWidth:
			if syntax.EmptyOp(inst.Arg)&^context == 0 {
				pending = append(pending, inst.Out)
			}
		case syntax.InstFail:
		case syntax.InstMatch, syntax.InstRune, syntax.InstRune1, syntax.InstRuneAny, syntax.InstRuneAnyNotNL:
			terminal = append(terminal, pos)
		}
	}

	return terminal
}

func programAccepts(program *syntax.Prog, start uint32, previous rune) bool {
	for _, pos := range programClosure(program, start, previous, -1) {
		if program.Inst[pos].Op == syntax.InstMatch {
			return true
		}
	}

	return false
}

func programStep(program *syntax.Prog, start uint32, previous, next rune) []uint32 {
	positions := make([]uint32, 0)

	for _, pos := range programClosure(program, start, previous, next) {
		inst := program.Inst[pos]
		switch inst.Op {
		case syntax.InstRune, syntax.InstRune1:
			if inst.MatchRune(next) {
				positions = append(positions, inst.Out)
			}
		case syntax.InstRuneAny:
			positions = append(positions, inst.Out)
		case syntax.InstRuneAnyNotNL:
			if next != '\n' {
				positions = append(positions, inst.Out)
			}
		case syntax.InstMatch, syntax.InstFail, syntax.InstAlt, syntax.InstAltMatch,
			syntax.InstCapture, syntax.InstEmptyWidth, syntax.InstNop:
		}
	}

	return positions
}

func boundaryRune(char rune) rune {
	if char == '\n' {
		return '\n'
	}

	if syntax.IsWordChar(char) {
		return 'a'
	}

	return '!'
}

func instructionEdges(inst syntax.Inst) []rune {
	edges := make([]rune, 0)
	for idx, char := range inst.Rune {
		edges = append(edges, char)
		if len(inst.Rune) == 1 || idx%2 == 1 {
			edges = append(edges, char+1)
		}

		if syntax.Flags(inst.Arg)&syntax.FoldCase == 0 {
			continue
		}

		for folded := unicode.SimpleFold(char); folded != char; folded = unicode.SimpleFold(folded) {
			edges = append(edges, folded, folded+1)
		}
	}

	return edges
}
