package lint

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

const manifestPath = ".claude/skills/VENDORED-mattpocock.md"

// Everything this repository does not author or does not own the bytes of.
//
//	.claude/skills/<vendored>/   someone else's files (MIT, mattpocock) —
//	                             fixing them makes the next re-sync a
//	                             three-way merge instead of a copy
//	CLAUDE.md                    owned by ClaudeMD, and its KARPATHY block is
//	                             a verbatim upstream mirror: a finding inside
//	                             it would be unfixable
//	proto-product-snapshot.md    a Playwright accessibility-tree dump, so
//	                             every prose rule is wrong about it
//	*/testdata/*                 golden files — the bytes ARE the assertion
//	tests/{legacy,fixtures}/     parked and fixture trees, as in Comments
const alwaysExcluded = `^CLAUDE\.md$|^proto-product-snapshot\.md$|/testdata/|` +
	`^tests/(legacy|bdd-cli/fixtures)/`

var vendoredHeadingRE = regexp.MustCompile(`^(Engineering|Productivity):`)

// Markdown runs markdownlint-cli2 over the markdown this repository authors.
// The config is NOT passed with --config: cli2's walk up from each linted file
// is what makes its `overrides:` and per-directory config work at all.
func Markdown(out io.Writer, files []string) error {
	err := needTool("markdownlint-cli2", "brew install markdownlint-cli2")
	if err != nil {
		return err
	}

	excluded, err := markdownExclusions()
	if err != nil {
		return err
	}

	specs := []string{"."}
	if len(files) > 0 {
		specs = files
	}

	scoped, err := markdownFiles(specs, excluded)
	if err != nil {
		return err
	}

	if len(scoped) == 0 {
		_, _ = fmt.Fprintln(out, "lint-markdown: OK (no markdown in scope)")

		return nil
	}

	// --fix only when files are named; a bare run mirrors CI and must not
	// rewrite. The hook's message promises the fixing happened.
	args := scoped
	if len(files) > 0 {
		args = append([]string{"--fix"}, scoped...)
	}

	err = runTool(out, "markdownlint-cli2", args...)
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

// markdownExclusions reads the vendored skill names out of the manifest
// rather than repeating them here, so taking a 25th skill needs no edit.
func markdownExclusions() (*regexp.Regexp, error) {
	raw, err := disk.Read(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", manifestPath, err)
	}

	vendored := vendoredSkills(string(raw))
	if len(vendored) == 0 {
		return nil, fmt.Errorf("%w: parsed no vendored skills from %s", ErrFailed, manifestPath)
	}

	pattern := `^\.claude/skills/(` + strings.Join(vendored, "|") + `)/|` + alwaysExcluded

	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("building the exclusion pattern: %w", err)
	}

	return compiled, nil
}

// vendoredSkills reads the two prose runs ("Engineering: a, b, c" and
// "Productivity: …"), each wrapped over several lines and ended by a blank.
func vendoredSkills(manifest string) []string {
	var names []string

	collecting := false

	for _, line := range splitLines(manifest) {
		if vendoredHeadingRE.MatchString(line) {
			collecting = true
		}

		if collecting && strings.TrimSpace(line) == "" {
			collecting = false
		}

		if !collecting {
			continue
		}

		body := strings.TrimSuffix(vendoredHeadingRE.ReplaceAllString(line, ""), ".")
		for _, name := range strings.Split(body, ",") {
			if name = strings.Trim(name, " \t"); name != "" {
				names = append(names, regexp.QuoteMeta(name))
			}
		}
	}

	return names
}
