package main

import "testing"

func TestGoDurationNormalizationPreservesLegacyRequestsAndFailureEvidence(t *testing.T) {
	t.Parallel()

	first := "failure\n--- FAIL: TestDouble (0.63s)\nexpected 6, got 3\n"
	second := "failure\n--- FAIL: TestDouble (12.46s)\nexpected 6, got 3\n"

	for _, enabled := range []bool{false, true} {
		leftArgs, leftInput := canonicalGoRequest([]string{"-p", first}, []byte(first), enabled)
		rightArgs, rightInput := canonicalGoRequest([]string{"-p", second}, []byte(second), enabled)

		equal := requestHash(leftArgs, leftInput, "/host") == requestHash(rightArgs, rightInput, "/host")
		if equal != enabled {
			t.Fatalf("normalization=%v: equal hashes=%v", enabled, equal)
		}
	}

	changed := "failure\n--- FAIL: TestDouble (0.63s)\nexpected 6, got 4\n"
 changedArgs, changedInput := canonicalGoRequest([]string{"-p", first}, []byte(changed), true)

	originalArgs, originalInput := canonicalGoRequest([]string{"-p", first}, []byte(first), true)
	if requestHash(changedArgs, changedInput, "/host") == requestHash(originalArgs, originalInput, "/host") {
		t.Fatal("normalization hid changed failure evidence")
	}
}
