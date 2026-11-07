package verification

import (
	"fmt"
	"regexp"
	"regexp/syntax"
	"slices"
	"strings"
)

// Form preserves literal wording and finite alternatives with selected complete original occurrences.
type Form struct {
	Form     string    `yaml:"form"`
	Examples []Example `yaml:"examples"`
}

// Example retains an original step and its complete capture tuple.
type Example struct {
	Text     string   `yaml:"text"`
	Captures []string `yaml:"captures"`
}

const examplesPerForm = 3

// Forms samples variable values, never literal alternatives or the admitted pattern language.
func Forms(pattern string, texts []string) ([]Form, error) {
	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("compile verification pattern: %w", err)
	}

	tree, err := syntax.Parse(pattern, syntax.Perl)
	if err != nil {
		return nil, fmt.Errorf("parse verification pattern: %w", err)
	}

	finite := captureLiterals(tree)
	groups := make(map[string][]Example)

	for _, text := range texts {
		spans := compiled.FindStringSubmatchIndex(text)
		if spans == nil {
			continue
		}

		form := stepForm(text, spans, finite)
		groups[form] = append(groups[form], Example{text, compiled.FindStringSubmatch(text)[1:]})
	}

	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}

	slices.Sort(keys)

	forms := make([]Form, 0, len(keys))
	for _, key := range keys {
		forms = append(forms, Form{key, selectExamples(groups[key])})
	}

	return forms, nil
}

func captureLiterals(tree *syntax.Regexp) map[int][]string {
	literals := make(map[int][]string)

	var visit func(*syntax.Regexp)

	visit = func(node *syntax.Regexp) {
		if node.Op == syntax.OpCapture {
			literals[node.Cap] = finiteBranches(node.Sub[0])
		}

		for _, child := range node.Sub {
			visit(child)
		}
	}
	visit(tree)

	return literals
}

func stepForm(text string, spans []int, literals map[int][]string) string {
	var result strings.Builder

	end := 0

	const pair = 2
	for index := 1; index < len(spans)/pair; index++ {
		start, stop := spans[index*pair], spans[index*pair+1]
		if start < end || start < 0 || literalCapture(text[start:stop], literals[index]) {
			continue
		}

		result.WriteString(text[end:start])
		fmt.Fprintf(&result, "<arg%d>", index)

		end = stop
	}

	result.WriteString(text[end:])

	return result.String()
}

func selectExamples(examples []Example) []Example {
	slices.SortFunc(examples, func(left, right Example) int { return strings.Compare(left.Text, right.Text) })

	selected := make([]Example, 0, examplesPerForm)
	seen := make(map[string]bool)

	for len(examples) > 0 && len(selected) < examplesPerForm {
		best := 0
		for index := range examples {
			if novelty(examples[index], seen) > novelty(examples[best], seen) {
				best = index
			}
		}

		example := examples[best]

		selected = append(selected, example)
		for index, value := range example.Captures {
			seen[fmt.Sprintf("%d:%s", index, value)] = true
		}

		examples = append(examples[:best], examples[best+1:]...)
	}

	return selected
}

func novelty(example Example, seen map[string]bool) int {
	count := 0

	for index, value := range example.Captures {
		if !seen[fmt.Sprintf("%d:%s", index, value)] {
			count++
		}
	}

	return count
}

func literalCapture(value string, literals []string) bool {
	for _, literal := range literals {
		if strings.EqualFold(value, literal) {
			return true
		}
	}

	return false
}
