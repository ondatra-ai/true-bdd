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
		{"mapping", `{"answer":"fail","findings":[
 {"definition":"double","category":"phrase_mapping","finding":"does not call double"}]}`, true, false},
		{"meaning", `{"answer":"fail","findings":[
 {"definition":"double","category":"phrase_meaning","finding":"checks half instead"}]}`, true, false},
		{"computes", `{"answer":"fail","findings":[
 {"definition":"double","category":"computes_assertion","finding":"doubles locally"}]}`, true, false},
		{"unknown verdict", `{"answer":"maybe","findings":[]}`, false, true},
		{"empty failure", `{"answer":"fail","findings":[]}`, false, true},
		{"contradictory pass", `{"answer":"pass","findings":[
 {"definition":"double","category":"phrase_meaning","finding":"wrong"}]}`, false, true},
		{"unnamed", `{"answer":"fail","findings":[
 {"definition":"","category":"phrase_meaning","finding":"wrong"}]}`, false, true},
		{"unknown category", `{"answer":"fail","findings":[
 {"definition":"double","category":"style","finding":"wrong"}]}`, false, true},
		{"empty finding", `{"answer":"fail","findings":[
 {"definition":"double","category":"phrase_meaning","finding":" "}]}`, false, true},
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

			if testCase.wantFail && (!strings.Contains(findings[0], "double") ||
				!strings.Contains(findings[0], "phrase_") && !strings.Contains(findings[0], "computes_assertion")) {
				t.Fatalf("finding lost definition/category: %v", findings)
			}
		})
	}
}
