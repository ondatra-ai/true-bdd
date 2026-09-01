package validate

import (
	"errors"
	"path/filepath"
	"testing"

	pkgerrors "github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/pkg/errors"
)

// A model answer the engine cannot read used to grade StatusFail, which
// reports a verdict the model never gave. See E2E-024's judge history for
// the same defect class one layer up.
func TestParseResultFileRefusesUnusableAnswers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		body    string
		wantErr error
	}{
		{
			name:    "no result block at all",
			body:    "",
			wantErr: pkgerrors.ErrChecklistAnswerMissing,
		},
		{
			name:    "block present but not YAML",
			body:    "answer: pass\n  context: [oops",
			wantErr: pkgerrors.ErrChecklistAnswerUnparseable,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			evaluator := &ChecklistEvaluator{}
			path := filepath.Join(t.TempDir(), "result.yaml")

			response := "I looked at the registry and it seems fine to me."
			if testCase.body != "" {
				response = "=== FILE_START: " + path + " ===\n" +
					testCase.body + "\n=== FILE_END: " + path + " ==="
			}

			_, err := evaluator.parseResultFile(response, path)
			if !errors.Is(err, testCase.wantErr) {
				t.Fatalf("parseResultFile error = %v, want %v", err, testCase.wantErr)
			}
		})
	}
}

// The universal contract is pass or fail. Anything else used to grade as a
// fail silently, which is a fabricated verdict rather than a refusal.
func TestCanonicalStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		answer  string
		wantErr bool
	}{
		{"pass", AnswerPass, false},
		{"fail", AnswerFail, false},
		{"case and space are free", "  PASS  ", false},
		{"a number", "5", true},
		{"a hedge", "maybe", true},
		{"nothing at all", "", true},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, err := canonicalStatus(testCase.answer, "tmp/result.yaml")

			switch {
			case testCase.wantErr && !errors.Is(err, pkgerrors.ErrChecklistAnswerNonCanonical):
				t.Fatalf("canonicalStatus(%q) error = %v, want non-canonical", testCase.answer, err)
			case !testCase.wantErr && err != nil:
				t.Fatalf("canonicalStatus(%q) = %v, want no error", testCase.answer, err)
			}
		})
	}
}
