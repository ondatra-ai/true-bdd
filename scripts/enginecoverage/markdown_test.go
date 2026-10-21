package enginecoverage_test

import (
	"testing"

	"github.com/ondatra-ai/true-bdd/scripts/enginecoverage"
)

// twoPackages is testdata/sample's report under Go 1.27, written out by hand.
func twoPackages() enginecoverage.Report {
	return enginecoverage.Report{Packages: []enginecoverage.Package{
		{
			Path: "greeting", Statements: 7, Covered: 3,
			Functions: []enginecoverage.Function{
				{
					Name: "Greet", File: "greeting/greeting.go", Statements: 3,
					Uncovered: []enginecoverage.Span{{FirstLine: 11, LastLine: 12, Statements: 1, Blocks: 1}},
				},
				{
					Name: "(*Greeter).Farewell", File: "greeting/greeting.go", Statements: 3,
					Uncovered: []enginecoverage.Span{{FirstLine: 24, LastLine: 28, Statements: 3, Blocks: 3}},
				},
			},
		},
		{
			Path: ".", Statements: 6, Covered: 5,
			Functions: []enginecoverage.Function{
				{
					Name: "main", File: "main.go", Statements: 6,
					Uncovered: []enginecoverage.Span{{FirstLine: 18, LastLine: 18, Statements: 1, Blocks: 1}},
				},
			},
		},
		{Path: "quiet", Statements: 2, Covered: 2},
	}}
}

const twoPackagesMarkdown = "# Engine coverage\n" +
	"\n" +
	"10 of 15 statements covered (66.7%) in 3 packages; 5 statements in 5 blocks never ran.\n" +
	"\n" +
	"## Packages\n" +
	"\n" +
	"Least covered first.\n" +
	"\n" +
	"| Package | Covered | Statements | Coverage |\n" +
	"|---|---:|---:|---:|\n" +
	"| `greeting` | 3 | 7 | 42.9% |\n" +
	"| `.` | 5 | 6 | 83.3% |\n" +
	"| `quiet` | 2 | 2 | 100.0% |\n" +
	"\n" +
	"## Uncovered blocks\n" +
	"\n" +
	"### `greeting`\n" +
	"\n" +
	"- `Greet` in `greeting/greeting.go`: 1 of 3 statements never ran\n" +
	"  - lines 11-12: 1 statement in 1 block\n" +
	"- `(*Greeter).Farewell` in `greeting/greeting.go`: never ran (3 statements)\n" +
	"  - lines 24-28: 3 statements in 3 blocks\n" +
	"\n" +
	"### `.`\n" +
	"\n" +
	"- `main` in `main.go`: 1 of 6 statements never ran\n" +
	"  - line 18: 1 statement in 1 block\n"

func TestMarkdownTablesEveryPackageThenListsUnrunSpansByFunction(t *testing.T) {
	t.Parallel()

	got := string(twoPackages().Markdown())
	if got != twoPackagesMarkdown {
		t.Errorf("report differs.\n--- got\n%s\n--- want\n%s", got, twoPackagesMarkdown)
	}
}
