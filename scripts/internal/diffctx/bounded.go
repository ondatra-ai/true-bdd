package diffctx

import (
	"fmt"
	"unicode/utf8"
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

// diffVerb is the subcommand every scope here is an argument to.
const diffVerb = "diff"

// Git runs a git subcommand and returns its stdout. Every caller here already
// owns the policy for a failed git, so this signature has no error.
type Git func(args ...string) string

// Bounded is the complete --stat under label, plus as much of the diff body
// as the budget allows, with the body's shape named in the text.
func Bounded(git Git, label string, scope []string, budget int) string {
	stat := git(append([]string{diffVerb}, append(scope, "--stat")...)...)
	body := git(append([]string{diffVerb}, scope...)...)
	shape := "the complete diff"

	if utf8.RuneCountInString(body) > budget {
		filtered := append(append([]string{diffVerb}, scope...), "--", ".")
		body = git(append(filtered, Excludes...)...)
		shape = "the diff with recorded cassettes and doc-universe.html filtered out"
	}

	if utf8.RuneCountInString(body) > budget {
		body = truncate(body, budget)
		shape = "a PREFIX of the diff, truncated — lean on the complete --stat above " +
			"rather than describing this prefix as the whole change"
	}

	return fmt.Sprintf("=== %s (complete) ===\n%s\n\n=== Diff — this is %s ===\n%s\n",
		label, stat, shape, body)
}

// truncate cuts at a rune boundary, so the prefix is still valid UTF-8.
func truncate(text string, budget int) string {
	if utf8.RuneCountInString(text) <= budget {
		return text
	}

	runes := []rune(text)

	return string(runes[:budget])
}
