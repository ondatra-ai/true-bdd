package main

import "testing"

const testResultPath = "tmp/2026-01-01-00-00/01-9.9-checklist-mini-s-result.yaml"

// wrapExact wraps content in exact production FILE markers.
func wrapExact(content string) string {
	return "prose before\n=== FILE_START: " + testResultPath + " ===\n" +
		content + "\n=== FILE_END: " + testResultPath + " ===\ntrailer"
}

// TestClassifyResponse pins the strict verdict classification.
func TestClassifyResponse(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		response string
		want     VerdictClass
	}{
		{"canonical pass", wrapExact("answer: pass\ncontext:\n  - ok"), ClassPassCanonical},
		{"canonical fail", wrapExact("answer: fail"), ClassFailCanonical},
		{"case and space folded", wrapExact("answer: ' PASS '"), ClassPassCanonical},
		{"non-canonical yes", wrapExact("answer: yes"), ClassNonCanonical},
		{"non-canonical warn", wrapExact("answer: warn"), ClassNonCanonical},
		{"missing answer", wrapExact("context:\n  - only context"), ClassNonCanonical},
		{"mapping answer", wrapExact("answer:\n  verdict: pass"), ClassNonCanonical},
		{"no markers at all", "I could not produce the file, sorry.", ClassProtocolNoMarker},
		{"malformed yaml", wrapExact("answer: [unclosed"), ClassProtocolBadYAML},
		{"fenced canonical yaml", wrapExact("```yaml\nanswer: pass\n```"), ClassPassCanonical},
		{"mapping-typed context", wrapExact("answer: pass\ncontext:\n  key: value"), ClassProtocolBadYAML},
		{"non-string fix_prompt", wrapExact("answer: fail\nfix_prompt:\n  a: b"), ClassProtocolBadYAML},
		{
			"fuzzy markers",
			"== \"FILE_START: " + testResultPath + "\" ==\nanswer: fail\n== FILE_END: " +
				testResultPath + " ==",
			ClassFailCanonical,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := ClassifyResponse(testCase.response, testResultPath)
			if got != testCase.want {
				t.Errorf("got %s, want %s", got, testCase.want)
			}
		})
	}
}

// TestVerdictCredits pins which classes may earn credit.
func TestVerdictCredits(t *testing.T) {
	t.Parallel()

	if !ClassPassCanonical.Credits() || !ClassFailCanonical.Credits() {
		t.Error("canonical classes must credit")
	}

	for _, c := range []VerdictClass{ClassNonCanonical, ClassProtocolNoMarker, ClassProtocolBadYAML} {
		if c.Credits() {
			t.Errorf("%s must not credit", c)
		}
	}
}
