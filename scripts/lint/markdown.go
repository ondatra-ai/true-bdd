package lint

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/cli/markdownlint"
)

// Everything this repository does not author or does not own the bytes of.
//
//	.claude/skills/mpc_*/        someone else's files (MIT, mattpocock) —
//	                             fixing them makes the next re-sync a
//	                             three-way merge instead of a copy
//	proto-product-snapshot.md    a Playwright accessibility-tree dump, so
//	                             every prose rule is wrong about it
//	*/testdata/*                 golden files — the bytes ARE the assertion
//	tests/{legacy,fixtures}/     parked and fixture trees, as in Comments
const excluded = `^\.claude/skills/mpc_[^/]+/|^proto-product-snapshot\.md$|/testdata/|` +
	`^tests/(legacy|bdd-cli/fixtures)/`

var excludedRE = regexp.MustCompile(excluded)

// Markdown runs markdownlint-cli2 over the markdown this repository authors.
// The config is NOT passed with --config: cli2's walk up from each linted file
// is what makes its `overrides:` and per-directory config work at all.
func Markdown(out io.Writer, files []string) error {
	err := markdownlint.Available()
	if err != nil {
		return err
	}

	specs := []string{"."}
	if len(files) > 0 {
		specs = files
	}

	scoped, err := markdownFiles(specs, excludedRE)
	if err != nil {
		return err
	}

	if len(scoped) == 0 {
		_, _ = fmt.Fprintln(out, "lint-markdown: OK (no markdown in scope)")

		return nil
	}

	// --fix only when files are named; a bare run mirrors CI and must not
	// rewrite. The hook's message promises the fixing happened.
	result, err := markdownlint.Lint(out, len(files) > 0, scoped...)
	if err == nil && result.Code != 0 {
		err = ErrFailed
	}

	if err != nil {
		_, _ = fmt.Fprint(out, `
lint-markdown: fix each finding above. What --fix could rewrite is
already applied, so what remains needs a real edit — a fence without
a language, a heading a file genuinely lacks.
`)

		return err
	}

	_, _ = fmt.Fprintf(out, "lint-markdown: OK (%d markdown files)\n", len(scoped))

	return nil
}

func markdownFiles(specs []string, excluded *regexp.Regexp) ([]string, error) {
	paths, err := trackedFiles(specs...)
	if err != nil {
		return nil, err
	}

	var scoped []string

	for _, path := range paths {
		if strings.HasSuffix(path, ".md") && !excluded.MatchString(path) {
			scoped = append(scoped, path)
		}
	}

	return scoped, nil
}

