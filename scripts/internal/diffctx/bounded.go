package diffctx

import (
	"fmt"
	"unicode/utf8"

	"github.com/ondatra-ai/true-bdd/pkg/cli/git"
)

// DefaultBudget is roughly 50k tokens of diff, which leaves generous headroom.
const DefaultBudget = 200_000

// Excludes are the bulk-by-nature paths; they stay in the --stat.
//
//nolint:gochecknoglobals // a pathspec list; a constant in all but syntax.
var Excludes = []string{
	":(exclude)tests/bdd-cli/fixtures/*/cassettes/*",
	":(exclude)docs/doc-universe.html",
}

// Bounded is the complete --stat under label, plus as much of the diff body
// as the budget allows, with the body's shape named in the text.
func Bounded(label string, scope []string, budget int) (string, error) {
	stat, err := git.DiffStat(scope...)
	if err != nil {
		return "", err
	}

	body, err := git.Diff(scope...)
	if err != nil {
		return "", err
	}

	shape := "the complete diff"

	if utf8.RuneCountInString(body) > budget {
		body, err = git.DiffExcluding(scope, Excludes...)
		if err != nil {
			return "", err
		}

		shape = "the diff with recorded cassettes and doc-universe.html filtered out"
	}

	if utf8.RuneCountInString(body) > budget {
		body = truncate(body, budget)
		shape = "a PREFIX of the diff, truncated — lean on the complete --stat above " +
			"rather than describing this prefix as the whole change"
	}

	return fmt.Sprintf("=== %s (complete) ===\n%s\n\n=== Diff — this is %s ===\n%s\n",
		label, stat, shape, body), nil
}

// truncate cuts at a rune boundary, so the prefix is still valid UTF-8.
func truncate(text string, budget int) string {
	if utf8.RuneCountInString(text) <= budget {
		return text
	}

	runes := []rune(text)

	return string(runes[:budget])
}
