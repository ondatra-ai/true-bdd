package alint_test

import (
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/cli/alint"
)

// The two subcommands write different keys — `violations` for check, `items`
// for fix — and one decoder reads both, so both shapes are pinned.
func TestDecodeCheckShape(t *testing.T) {
	t.Parallel()

	report := decode(t, `{
	  "schema_version": 1,
	  "summary": {"failing_rules": 1, "total_violations": 1},
	  "results": [{"id": "trailing", "level": "error", "passed": false,
	    "violations": [{"path": "f.txt", "message": "trailing whitespace on line 1",
	      "line": 1, "column": 3}]}]
	}`)

	if len(report.Findings) != 1 {
		t.Fatalf("findings: got %d, want 1", len(report.Findings))
	}

	found := report.Findings[0]
	want := alint.Finding{
		RuleID: "trailing", Level: "error", Path: "f.txt",
		Message: "trailing whitespace on line 1", Line: 1, Column: 3,
	}

	if found != want {
		t.Errorf("finding:\n got %+v\nwant %+v", found, want)
	}

	// A check attempts nothing, so every finding it reports is still standing.
	if len(report.Outstanding()) != 1 {
		t.Errorf("outstanding: got %d, want 1", len(report.Outstanding()))
	}
}

func TestDecodeFixShape(t *testing.T) {
	t.Parallel()

	report := decode(t, `{
	  "schema_version": 1,
	  "summary": {"applied": 1, "skipped": 0, "unfixable": 1},
	  "results": [{"id": "spacing", "level": "error", "items": [
	    {"path": "a.go", "message": "fixed", "status": "applied"},
	    {"path": "b.go", "message": "needs a real edit", "status": "unfixable"}]}]
	}`)

	if report.Applied != 1 || report.Unfixable != 1 {
		t.Errorf("summary: got applied=%d unfixable=%d, want 1 and 1",
			report.Applied, report.Unfixable)
	}

	// What alint rewrote is done with; only the rest is worth reporting on.
	left := report.Outstanding()
	if len(left) != 1 || left[0].Path != "b.go" {
		t.Errorf("outstanding: got %+v, want only b.go", left)
	}
}

func TestDecodeRejectsGarbage(t *testing.T) {
	t.Parallel()

	_, err := alint.Decode("not json")
	if err == nil {
		t.Fatal("decoding garbage must fail, not report a clean tree")
	}
}

// The manifest is read relative to its OWN directory, so every entry escapes
// tmp/ — an unescaped one matches nothing, silently.
func TestScopeBodyEscapesTmp(t *testing.T) {
	t.Parallel()

	got := alint.ScopeBody([]string{"scripts/lint/dispatch.go", "./docs/adr/0005.md"})
	want := "../scripts/lint/dispatch.go\n../docs/adr/0005.md\n"

	if got != want {
		t.Errorf("manifest:\n got %q\nwant %q", got, want)
	}
}

func TestScopeBodyEmpty(t *testing.T) {
	t.Parallel()

	if got := alint.ScopeBody(nil); got != "" {
		t.Errorf("an empty scope must write no lines, got %q", got)
	}
}

func decode(t *testing.T, stdout string) alint.Report {
	t.Helper()

	report, err := alint.Decode(stdout)
	if err != nil {
		t.Fatalf("decoding: %v", err)
	}

	return report
}
