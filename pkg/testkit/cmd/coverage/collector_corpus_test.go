package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// buildFullCorpus assembles every scenario of the synthetic corpus.
func buildFullCorpus(t *testing.T) *corpus {
	t.Helper()

	corp := newCorpus(t)

	// S1: pass-only walk over prompts 1-3 (prompt 4 never exercised).
	run := corp.addFixture("S1", "mini-pass", "us mini 9.9")
	corp.withChecklist(run, miniChecklist)

	for idx := 1; idx <= 3; idx++ {
		corp.evalCell(run, idx, "pass")
	}

	corp.logLines(run, lineLoaded(), lineSaved(1), lineSaved(2), lineSaved(3))

	// S2: prompt 2 fails, fix applied, clean re-walk → fix-effective.
	run = corp.addFixture("S2", "mini-fix", "us mini 9.9")
	corp.withChecklist(run, miniChecklist)

	for idx := 1; idx <= 3; idx++ {
		corp.evalCell(run, idx, "pass")
	}

	corp.fixEvidence(run)
	corp.applyResult(run)
	corp.logLines(run, lineLoaded(),
		lineSaved(1), lineSaved(2),
		lineGen(), lineApplied(),
		lineSaved(1), lineSaved(2), lineSaved(3), lineSaved(4),
		lineSaved(1), lineSaved(2), lineSaved(3), lineSaved(4))

	// S3: same fix but no clean re-walk in the log → candidate only.
	run = corp.addFixture("S3", "mini-nowalk", "us mini 9.9")
	corp.withChecklist(run, miniChecklist)
	corp.evalCell(run, 2, "pass")
	corp.fixEvidence(run)
	corp.applyResult(run)
	corp.logLines(run, lineLoaded(), lineSaved(2),
		lineGen(), lineApplied(),
		lineSaved(1), lineSaved(2), lineSaved(3), lineSaved(4))

	// S4: protocol failure with a stale surviving result file.
	run = corp.addFixture("S4", "mini-protocol", "us mini 9.9")
	corp.withChecklist(run, miniChecklist)
	corp.evalCell(run, 1, "pass")
	corp.write(run+"/tmp/"+miniPart+"/"+
		strings.Replace(resultName(1), "-result.yaml", "-response.txt", 1),
		"no markers here at all")
	corp.logLines(run, lineLoaded())

	// S5: two failed prompts and one apply, with no fix/apply evidence in
	// the log → the apply is unattributed. The engine always writes
	// "Loaded prompts" before evaluating, so the log line stays present.
	run = corp.addFixture("S5", "mini-twofail", "us mini 9.9")
	corp.withChecklist(run, miniChecklist)
	corp.evalCell(run, 1, "fail")
	corp.evalCell(run, 3, "fail")
	corp.applyResult(run)
	corp.logLines(run, lineLoaded())

	// S6: overridden checklist whose first prompt differs → non-shipped.
	run = corp.addFixture("S6", "mini-override", "us mini 9.9")
	corp.withChecklist(run, strings.Replace(miniChecklist, "Question one?", "Question ONE modified?", 1))
	corp.evalCell(run, 1, "pass")
	corp.logLines(run, lineLoaded())

	addTailScenarios(corp)
	addAdversarialScenarios(corp)

	return corp
}

// addTailScenarios adds the help-flag shape (S7) and the genless-log
// scenario (S8: an apply with NO generation event must stay
// unattributed — never the single-failed-prompt fallback).
func addTailScenarios(corp *corpus) {
	run := corp.addFixture("S7", "help-flag", "--help")
	corp.logLines(run, "")

	run = corp.addFixture("S8", "mini-genless", "us mini 9.9")
	corp.withChecklist(run, miniChecklist)
	corp.evalCell(run, 2, "pass")
	corp.fixEvidence(run)
	corp.applyResult(run)
	corp.logLines(run, lineLoaded(), lineSaved(2),
		lineApplied(),
		lineSaved(1), lineSaved(2), lineSaved(3), lineSaved(4),
		lineSaved(1), lineSaved(2), lineSaved(3), lineSaved(4))
}

// addAdversarialScenarios covers the review-3 fail-closed edges.
func addAdversarialScenarios(corp *corpus) {
	// S9: response says pass but the surviving result.yaml says fail —
	// the stale-canonical-response signature. Cell must be uncredited.
	run := corp.addFixture("S9", "mini-conflict", "us mini 9.9")
	corp.withChecklist(run, miniChecklist)
	corp.evalCell(run, 3, "pass")
	corp.overwriteResult(run, 3, "fail")
	corp.logLines(run, lineLoaded())

	// S10: complete fix chain but a protocol WARN lands after the last
	// apply — the post-apply walk is not clean, so no fix credit.
	run = corp.addFixture("S10", "mini-warnwalk", "us mini 9.9")
	corp.withChecklist(run, miniChecklist)
	corp.evalCell(run, 2, "pass")
	corp.fixEvidence(run)
	corp.applyResult(run)
	corp.logLines(run, lineLoaded(), lineSaved(2),
		lineGen(), lineApplied(),
		lineSaved(1), lineSaved(2), lineSaved(3), lineSaved(4),
		lineSaved(1), lineSaved(2), lineSaved(3), lineSaved(4),
		lineWarnBadYAML(1))

	// S11: a malformed log line taints the whole run's chronology —
	// the otherwise complete chain must stay a candidate.
	run = corp.addFixture("S11", "mini-badlog", "us mini 9.9")
	corp.withChecklist(run, miniChecklist)
	corp.evalCell(run, 2, "pass")
	corp.fixEvidence(run)
	corp.applyResult(run)
	corp.logLines(run, lineLoaded(), lineSaved(2),
		"NOT VALID JSON AT ALL",
		lineGen(), lineApplied(),
		lineSaved(1), lineSaved(2), lineSaved(3), lineSaved(4),
		lineSaved(1), lineSaved(2), lineSaved(3), lineSaved(4))
}

// scanCorpus runs the full pipeline over the corpus.
func scanCorpus(t *testing.T, corp *corpus) (*Universe, *Profile) {
	t.Helper()

	uni, err := LoadUniverse(corp.Checklists)
	if err != nil {
		t.Fatalf("universe: %v", err)
	}

	runs, err := ScanRoot(corp.Root)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}

	observations := make([]Observation, 0)
	diagnostics := make([]Diagnostic, 0)

	for _, run := range runs {
		obs, diags := CollectRun(run, uni, corp.FixturesDir)

		observations = append(observations, obs...)
		diagnostics = append(diagnostics, diags...)
	}

	return uni, BuildProfile(uni, observations, diagnostics)
}

// TestCorpusEndToEnd asserts the exact covered set and diagnostics of
// the synthetic corpus: 7 of 9 branches, q4 fully uncovered.
func TestCorpusEndToEnd(t *testing.T) {
	t.Parallel()

	uni, profile := scanCorpus(t, buildFullCorpus(t))

	if got := len(uni.Branches); got != 9 {
		t.Fatalf("universe branches: got %d, want 9", got)
	}

	covered := profile.CoveredIDs()

	const wantCovered = 7 // q1 pass+fail, q2 pass+fail+fix, q3 pass+fail

	if len(covered) != wantCovered {
		t.Fatalf("covered: got %d (%v), want %d", len(covered), covered, wantCovered)
	}

	assertCoverageStates(t, covered)
	assertDiagnostics(t, profile)
}

// assertCoverageStates checks the kind-level expectations.
func assertCoverageStates(t *testing.T, covered []string) {
	t.Helper()

	joined := strings.Join(covered, "\n")

	for _, must := range []string{"q1@", "q2@", "q3@", "#pass", "#fail", "#fix"} {
		if !strings.Contains(joined, must) {
			t.Errorf("covered set lacks %q:\n%s", must, joined)
		}
	}

	if strings.Contains(joined, "q4@") {
		t.Errorf("q4 must stay uncovered:\n%s", joined)
	}

	if strings.Count(joined, "#fix") != 1 {
		t.Errorf("exactly one fix branch must be covered:\n%s", joined)
	}
}

// assertDiagnostics checks every uncredited signal landed in its bucket.
func assertDiagnostics(t *testing.T, profile *Profile) {
	t.Helper()

	diags := profile.Diagnostics

	assertBucket(t, "protocol fails", diags.ProtocolFails, "mini-protocol")
	assertBucket(t, "fix candidates", diags.FixCandidates, "mini-nowalk")
	assertBucket(t, "fix candidates", diags.FixCandidates, "mini-warnwalk")
	assertBucket(t, "fix candidates", diags.FixCandidates, "mini-badlog")
	assertBucket(t, "unclassified fails", diags.FailUnclassified, "mini-conflict")
	assertBucket(t, "unattributed applies", diags.UnattributedApply, "mini-twofail")
	assertBucket(t, "unattributed applies", diags.UnattributedApply, "mini-genless")
	assertRunDiagnostics(t, diags.RunDiagnostics)
	assertBucket(t, "non-shipped", diags.NonShipped, "mini-override")

	if len(diags.RunDiagnostics) == 0 {
		t.Error("expected a stale-result run diagnostic")
	}

	if len(diags.HardRunDiagnostics) != 0 {
		t.Errorf("unexpected hard diagnostics: %v", diags.HardRunDiagnostics)
	}
}

// assertRunDiagnostics requires the soft diagnostics to include the
// malformed-log and verdict-conflict signals.
func assertRunDiagnostics(t *testing.T, diagnostics []string) {
	t.Helper()

	joined := strings.Join(diagnostics, "\n")

	if !strings.Contains(joined, "malformed line") {
		t.Errorf("expected a malformed-log diagnostic, got: %s", joined)
	}

	if !strings.Contains(joined, "verdict conflict") {
		t.Errorf("expected a response/result conflict diagnostic, got: %s", joined)
	}
}

// assertBucket requires a diagnostic bucket to contain an entry from
// the given fixture.
func assertBucket(t *testing.T, label string, bucket []string, fixture string) {
	t.Helper()

	for _, line := range bucket {
		if strings.Contains(line, fixture) {
			return
		}
	}

	t.Errorf("%s: %v (want entry from %s)", label, bucket, fixture)
}

// TestProfileDeterminism pins byte-reproducible profiles.
func TestProfileDeterminism(t *testing.T) {
	t.Parallel()

	corp := buildFullCorpus(t)

	_, first := scanCorpus(t, corp)
	_, second := scanCorpus(t, corp)

	firstJSON, err := json.Marshal(first)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	secondJSON, err := json.Marshal(second)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if string(firstJSON) != string(secondJSON) {
		t.Error("two scans of the same tree must produce identical profiles")
	}
}
