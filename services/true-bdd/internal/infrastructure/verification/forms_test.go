package verification_test

import (
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/verification"
)

func TestFormsKeepMeaningfulAlternativesAndCompleteExamples(t *testing.T) {
	forms, err := verification.Forms(`^file "([^"]+)" (matches|does not match) (.+)$`, []string{
		`file "a" matches alpha`, `file "b" matches beta`, `file "c" matches gamma`, `file "d" matches delta`,
		`file "a" does not match alpha`,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(forms) != 2 || len(forms[0].Examples) != 1 || len(forms[1].Examples) != 3 {
		t.Fatalf("forms: %+v", forms)
	}

	if !strings.Contains(forms[0].Form, "does not match") || !strings.Contains(forms[1].Form, "matches") {
		t.Fatal(forms)
	}

	if forms[0].Examples[0].Captures[2] != "alpha" {
		t.Fatal(forms[0].Examples)
	}
}

func TestFormsKeepFiniteBranchesInsideVariableCaptures(t *testing.T) {
	forms, err := verification.Forms(`^the count is (\d+|no)$`, []string{"the count is 12", "the count is no"})
	if err != nil {
		t.Fatal(err)
	}

	if len(forms) != 2 || forms[0].Form != "the count is <arg1>" || forms[1].Form != "the count is no" {
		t.Fatal(forms)
	}
}

func TestFormsKeepCaseInsensitiveNegation(t *testing.T) {
	forms, err := verification.Forms(`(?i)^output (matches|does not match) (.+)$`, []string{
		"output MATCHES alpha", "output DOES NOT MATCH beta",
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(forms) != 2 {
		t.Fatalf("negation collapsed: %+v", forms)
	}
}
