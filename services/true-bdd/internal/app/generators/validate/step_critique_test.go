package validate_test

import (
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/validate"
)

func TestStepCritiqueVerdict(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name, response      string
		wantFail, wantError bool
	}{
		{"pass", `{"answer":"pass","findings":[]}`, false, false},
		{"binding", `{"answer":"fail","findings":[
 {"definition":"double","category":"concept_binding","finding":"does not call double"}]}`, true, false},
		{"meaning", `{"answer":"fail","findings":[
 {"definition":"double","category":"concept_meaning","finding":"checks half instead"}]}`, true, false},
		{"computes", `{"answer":"fail","findings":[
 {"definition":"double","category":"computes_assertion","finding":"doubles locally"}]}`, true, false},
		{"unknown verdict", `{"answer":"maybe","findings":[]}`, false, true},
		{"empty failure", `{"answer":"fail","findings":[]}`, false, true},
		{"contradictory pass", `{"answer":"pass","findings":[
 {"definition":"double","category":"concept_meaning","finding":"wrong"}]}`, false, true},
		{"unnamed", `{"answer":"fail","findings":[
 {"definition":"","category":"concept_meaning","finding":"wrong"}]}`, false, true},
		{"unknown category", `{"answer":"fail","findings":[
 {"definition":"double","category":"style","finding":"wrong"}]}`, false, true},
		{"retired template category", `{"answer":"fail","findings":[
 {"definition":"double","category":"phrase_mapping","finding":"wrong"}]}`, false, true},
		{"empty finding", `{"answer":"fail","findings":[
 {"definition":"double","category":"concept_meaning","finding":" "}]}`, false, true},
		{"missing findings", `{"answer":"pass"}`, false, true},
		{"invalid JSON", `not an answer`, false, true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			findings, err := validate.ParseStepCritique(testCase.response)
			if (err != nil) != testCase.wantError {
				t.Fatalf("error=%v", err)
			}

			if (len(findings) > 0) != testCase.wantFail {
				t.Fatalf("findings=%v", findings)
			}

			if testCase.wantFail && (!strings.Contains(findings[0].String(), "double") ||
				!strings.Contains(findings[0].String(), "concept_") &&
					!strings.Contains(findings[0].String(), "computes_assertion")) {
				t.Fatalf("finding lost definition/category: %v", findings)
			}
		})
	}
}

// A concept critique's pattern categories are findings against the
// pattern, which the engine sends back to the proposal; every other
// category is a finding against the body, which goes back to the writer.
func TestConceptCritiqueFindingNamesItsTarget(t *testing.T) {
	t.Parallel()

	findings, err := validate.ParseStepCritique(`{"answer":"fail","findings":[
 {"definition":"halving","category":"pattern_captures","finding":"captures only the first digit"},
 {"definition":"halving","category":"pattern_scope","finding":"binds a doubling step too"},
 {"definition":"halving","category":"concept_binding","finding":"requires CALC_LICENCE"},
 {"definition":"halving","category":"concept_meaning","finding":"doubles instead"},
 {"definition":"halving","category":"computes_assertion","finding":"halves locally"}]}`)
	if err != nil {
		t.Fatal(err)
	}

	want := []bool{true, true, false, false, false}
	if len(findings) != len(want) {
		t.Fatalf("findings=%v", findings)
	}

	for index, finding := range findings {
		if finding.Pattern() != want[index] {
			t.Errorf("%s: against the pattern = %v, want %v", finding, finding.Pattern(), want[index])
		}
	}

	if findings[0].String() != "halving: pattern_captures: captures only the first digit" {
		t.Fatalf("finding text: %q", findings[0].String())
	}
}
