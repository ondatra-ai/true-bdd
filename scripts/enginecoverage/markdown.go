package enginecoverage

import (
	"bytes"
	"fmt"
)

// percent is the scale a coverage ratio is shown on.
const percent = 100

// Markdown renders the report: a headline, every package least covered first,
// then each package's unrun blocks grouped by function.
func (r Report) Markdown() []byte {
	var out bytes.Buffer

	uncovered := r.Statements() - r.Covered()

	fmt.Fprintf(&out, "# Engine coverage\n\n%d of %s covered (%s) in %s; %s in %s never ran.\n",
		r.Covered(), plural(r.Statements(), "statement"), Ratio(r.Covered(), r.Statements()),
		plural(len(r.Packages), "package"), plural(uncovered, "statement"),
		plural(r.UncoveredBlocks(), "block"))

	out.WriteString("\n## Packages\n\nLeast covered first.\n\n" +
		"| Package | Covered | Statements | Coverage |\n|---|---:|---:|---:|\n")

	for _, pkg := range r.Packages {
		fmt.Fprintf(&out, "| `%s` | %d | %d | %s |\n",
			pkg.Path, pkg.Covered, pkg.Statements, Ratio(pkg.Covered, pkg.Statements))
	}

	out.WriteString("\n## Uncovered blocks\n")

	for _, pkg := range r.Packages {
		if len(pkg.Functions) == 0 {
			continue
		}

		fmt.Fprintf(&out, "\n### `%s`\n\n", pkg.Path)

		for _, fn := range pkg.Functions {
			writeFunction(&out, fn)
		}
	}

	return out.Bytes()
}

// writeFunction is one function's line and one nested line per unrun span.
func writeFunction(out *bytes.Buffer, function Function) {
	unrun := function.UncoveredStatements()
	if unrun == function.Statements {
		fmt.Fprintf(out, "- `%s` in `%s`: never ran (%s)\n",
			function.Name, function.File, plural(function.Statements, "statement"))
	} else {
		fmt.Fprintf(out, "- `%s` in `%s`: %d of %s never ran\n",
			function.Name, function.File, unrun, plural(function.Statements, "statement"))
	}

	for _, span := range function.Uncovered {
		lines := fmt.Sprintf("lines %d-%d", span.FirstLine, span.LastLine)
		if span.FirstLine == span.LastLine {
			lines = fmt.Sprintf("line %d", span.FirstLine)
		}

		fmt.Fprintf(out, "  - %s: %s in %s\n",
			lines, plural(span.Statements, "statement"), plural(span.Blocks, "block"))
	}
}

// Ratio is covered of total as a percentage to one decimal.
func Ratio(covered, total int) string {
	if total == 0 {
		return "n/a"
	}

	return fmt.Sprintf("%.1f%%", float64(covered)*percent/float64(total))
}

// plural counts a noun: `1 block`, `3 blocks`.
func plural(count int, noun string) string {
	if count == 1 {
		return fmt.Sprintf("%d %s", count, noun)
	}

	return fmt.Sprintf("%d %ss", count, noun)
}
