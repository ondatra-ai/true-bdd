package main

import "testing"

// TestDecodeEvaluatorName covers the real-world filename traps: dotted
// and dashed subjects, and the trailing-hyphen build-code subject that
// produces a double delimiter.
func TestDecodeEvaluatorName(t *testing.T) {
	t.Parallel()

	paths := []string{"us-apply-merge", "build-code-test-passes", "us-create-why"}

	cases := []struct {
		name    string
		file    string
		wantIdx int
		wantSub string
		ok      bool
	}{
		{
			"dotted-dashed subject", "01-99.3-001-checklist-us-apply-merge-result.yaml",
			1, "99.3-001", true,
		},
		{
			"trailing-hyphen subject",
			"01-frontend-integration-playwright-startup--checklist-build-code-test-passes-result.yaml",
			1, "frontend-integration-playwright-startup-", true,
		},
		{
			"simple subject response", "09-99.1-checklist-us-create-why-response.txt",
			9, "99.1", true,
		},
		{"unknown section", "01-99.1-checklist-other-section-result.yaml", 0, "", false},
		{"no index prefix", "xx-99.1-checklist-us-create-why-result.yaml", 0, "", false},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got, decoded := decodeEvaluatorName(testCase.file, paths)
			if decoded != testCase.ok {
				t.Fatalf("ok: got %v, want %v", decoded, testCase.ok)
			}

			if decoded && (got.PromptIndex != testCase.wantIdx || got.Subject != testCase.wantSub) {
				t.Errorf("got %+v, want idx=%d subject=%q", got, testCase.wantIdx, testCase.wantSub)
			}
		})
	}
}

// TestDecodeFixNames covers the fix filename shapes.
func TestDecodeFixNames(t *testing.T) {
	t.Parallel()

	fix, fixOK := decodeFixName("09-99.1-fix-prompts.md")
	if !fixOK || fix.PromptIndex != 9 || fix.Subject != "99.1" || fix.Kind != kindPromptsMD {
		t.Errorf("fix-prompts decode: %+v ok=%v", fix, fixOK)
	}

	iter, iterOK := decodeFixName("02-99.3-001-fix-iter3-user.txt")
	if !iterOK || iter.PromptIndex != 2 || iter.Subject != "99.3-001" || iter.Iteration != 3 {
		t.Errorf("fix-iter decode: %+v ok=%v", iter, iterOK)
	}

	if _, badOK := decodeFixName("random-file.txt"); badOK {
		t.Error("random file must not decode as fix artifact")
	}
}

// TestDecodeApplyNames covers the apply filename shapes, including the
// trailing-hyphen greedy-subject trap.
func TestDecodeApplyNames(t *testing.T) {
	t.Parallel()

	apply, applyOK := decodeApplyName("apply-frontend-integration-playwright-startup--iter1-result.yaml")
	if !applyOK || apply.Subject != "frontend-integration-playwright-startup-" || apply.Iteration != 1 {
		t.Errorf("apply decode: %+v ok=%v", apply, applyOK)
	}

	if _, respOK := decodeApplyName("apply-99.1-iter1-response.txt"); !respOK {
		t.Error("apply response must decode")
	}
}
