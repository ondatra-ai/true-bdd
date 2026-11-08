package verification

import (
	"regexp/syntax"
)

const (
	maxFiniteValues = 64
	maxFiniteRepeat = 3
)

func finiteBranches(tree *syntax.Regexp) []string {
	values, finite := finiteValues(tree)
	if finite {
		return values
	}

	if tree.Op == syntax.OpCapture {
		return finiteBranches(tree.Sub[0])
	}

	if tree.Op != syntax.OpAlternate {
		return nil
	}

	for _, child := range tree.Sub {
		values = append(values, finiteBranches(child)...)
	}

	return values
}

func finiteValues(tree *syntax.Regexp) ([]string, bool) {
	switch tree.Op {
	case syntax.OpEmptyMatch:
		return []string{""}, true
	case syntax.OpLiteral:
		return []string{string(tree.Rune)}, true
	case syntax.OpCharClass:
		return finiteClass(tree.Rune)
	case syntax.OpCapture:
		return finiteValues(tree.Sub[0])
	case syntax.OpAlternate:
		return finiteAlternation(tree.Sub)
	case syntax.OpConcat:
		return finiteConcatenation(tree.Sub)
	case syntax.OpQuest:
		values, ok := finiteValues(tree.Sub[0])

		return append(values, ""), ok
	case syntax.OpRepeat:
		return finiteRepeat(tree)
	case syntax.OpNoMatch, syntax.OpAnyCharNotNL, syntax.OpAnyChar, syntax.OpBeginLine, syntax.OpEndLine,
		syntax.OpBeginText, syntax.OpEndText, syntax.OpWordBoundary, syntax.OpNoWordBoundary, syntax.OpStar, syntax.OpPlus:
		return nil, false
	}

	return nil, false
}

func finiteClass(ranges []rune) ([]string, bool) {
	const pair = 2

	values := make([]string, 0)

	for index := 0; index < len(ranges); index += pair {
		if ranges[index+1]-ranges[index] > maxFiniteValues {
			return nil, false
		}

		for char := ranges[index]; char <= ranges[index+1]; char++ {
			values = append(values, string(char))
			if len(values) > maxFiniteValues {
				return nil, false
			}
		}
	}

	return values, true
}

func finiteAlternation(children []*syntax.Regexp) ([]string, bool) {
	values := make([]string, 0)

	for _, child := range children {
		branch, ok := finiteValues(child)
		if !ok || len(values)+len(branch) > maxFiniteValues {
			return nil, false
		}

		values = append(values, branch...)
	}

	return values, true
}

func finiteConcatenation(children []*syntax.Regexp) ([]string, bool) {
	values := []string{""}

	for _, child := range children {
		suffixes, ok := finiteValues(child)
		if !ok || len(values)*len(suffixes) > maxFiniteValues {
			return nil, false
		}

		next := make([]string, 0, len(values)*len(suffixes))
		for _, prefix := range values {
			for _, suffix := range suffixes {
				next = append(next, prefix+suffix)
			}
		}

		values = next
	}

	return values, true
}

func finiteRepeat(tree *syntax.Regexp) ([]string, bool) {
	if tree.Max < 0 || tree.Max > maxFiniteRepeat {
		return nil, false
	}

	values := make([]string, 0)
	for count := tree.Min; count <= tree.Max; count++ {
		children := make([]*syntax.Regexp, count)
		for index := range children {
			children[index] = tree.Sub[0]
		}

		expanded, ok := finiteConcatenation(children)
		if !ok || len(values)+len(expanded) > maxFiniteValues {
			return nil, false
		}

		values = append(values, expanded...)
	}

	return values, true
}
