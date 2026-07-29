package remote

import (
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/src/internal/infrastructure/events"
)

func TestValidateAnswer(t *testing.T) {
	t.Parallel()

	choice := string(events.KindChoice)
	clarify := string(events.KindClarify)
	freetext := string(events.KindFreetext)

	cases := []struct {
		name  string
		kind  string
		value string
		valid bool
	}{
		{"choice enum apply", choice, answerApply, true},
		{"choice enum refine", choice, "refine", true},
		{"choice enum exit", choice, answerExit, true},
		{"choice numeric", choice, "2", true},
		{"choice off-enum", choice, "delete", false},
		{"choice newline", choice, "apply\n", false},
		{"clarify single line", clarify, "42", true},
		{"clarify newline rejected", clarify, "line1\nline2", false},
		{"clarify over cap", clarify, strings.Repeat("x", maxClarifyBytes+1), false},
		{"freetext multiline ok", freetext, "line one\nline two", true},
		{"freetext over cap", freetext, strings.Repeat("y", maxFreetextBytes+1), false},
		{"unknown kind", "", "anything", false},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			err := validateAnswer(testCase.kind, testCase.value)
			if (err == nil) != testCase.valid {
				t.Fatalf("validateAnswer(%q,%q) err=%v, want valid=%v",
					testCase.kind, testCase.value, err, testCase.valid)
			}
		})
	}
}

// TestFormatAnswerSerialization proves the exact stdin framing: choice and
// clarify are single newline-terminated lines; freetext gets the extra
// terminating blank line the collector's read-until-blank loop needs (A2).
func TestFormatAnswerSerialization(t *testing.T) {
	t.Parallel()

	cases := []struct {
		kind  string
		value string
		want  string
	}{
		{string(events.KindChoice), answerExit, "exit\n"},
		{string(events.KindChoice), answerApply, "apply\n"},
		{string(events.KindClarify), "2", "2\n"},
		{string(events.KindFreetext), "line1\nline2", "line1\nline2\n\n"},
		{string(events.KindFreetext), "already\n", "already\n\n"},
	}

	for _, testCase := range cases {
		t.Run(testCase.kind+"/"+testCase.value, func(t *testing.T) {
			t.Parallel()

			got := formatAnswer(testCase.kind, testCase.value)
			if got != testCase.want {
				t.Fatalf("formatAnswer(%q,%q) = %q, want %q",
					testCase.kind, testCase.value, got, testCase.want)
			}
		})
	}
}
