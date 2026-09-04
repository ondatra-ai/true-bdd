package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestBaselineRoundTripAndRegression covers write/load/check plus the
// regression and newly-covered paths.
func TestBaselineRoundTripAndRegression(t *testing.T) {
	t.Parallel()

	corp := buildFullCorpus(t)
	_, full := scanCorpus(t, corp)

	baseline := BaselineFromProfile(full)
	if len(baseline.Covered) != 7 {
		t.Fatalf("baseline entries: got %d, want 7", len(baseline.Covered))
	}

	path := filepath.Join(t.TempDir(), "baseline.yaml")

	err := baseline.Write(path, nil, false)
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	loaded, err := LoadBaseline(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	_, checkErr := loaded.Check(full)
	if checkErr != nil {
		t.Fatalf("self-check must pass: %v", checkErr)
	}

	// A profile missing the S5 fail credits must regress.
	reduced := reducedProfile(t, corp)

	_, err = loaded.Check(reduced)
	if err == nil || !errors.Is(err, errBaselineRegression) {
		t.Fatalf("want regression error, got %v", err)
	}

	// The reverse direction reports newly covered candidates.
	reducedBaseline := BaselineFromProfile(reduced)

	newly, err := reducedBaseline.Check(full)
	if err != nil {
		t.Fatalf("reduced baseline vs full profile: %v", err)
	}

	if len(newly) != 2 {
		t.Errorf("newly covered: got %v, want the two S5 fail branches", newly)
	}
}

// reducedProfile rebuilds the corpus profile without the S5 session.
func reducedProfile(t *testing.T, corp *corpus) *Profile {
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

	for _, run := range runs {
		if run.Session == "S5" {
			continue
		}

		obs, _ := CollectRun(run, uni, corp.FixturesDir)
		observations = append(observations, obs...)
	}

	return BuildProfile(uni, observations, nil)
}

// TestBaselineWritePreservesExisting pins the shrink guard: a write
// that would drop entries fails without the explicit allow flag.
func TestBaselineWritePreservesExisting(t *testing.T) {
	t.Parallel()

	corp := buildFullCorpus(t)
	_, full := scanCorpus(t, corp)

	path := filepath.Join(t.TempDir(), "baseline.yaml")

	err := BaselineFromProfile(full).Write(path, nil, false)
	if err != nil {
		t.Fatalf("initial write: %v", err)
	}

	smaller := BaselineFromProfile(reducedProfile(t, corp))

	err = smaller.Write(path, nil, false)
	if err == nil || !errors.Is(err, errBaselineShrink) {
		t.Fatalf("want shrink refusal, got %v", err)
	}

	err = smaller.Write(path, nil, true)
	if err != nil {
		t.Fatalf("explicit rebaseline must succeed: %v", err)
	}
}

// TestGuardShrinkAbortsOnInvalidExisting pins the fail-closed rule for
// broken existing baselines: only a genuinely absent file may be
// written over without -rebaseline.
func TestGuardShrinkAbortsOnInvalidExisting(t *testing.T) {
	t.Parallel()

	corp := buildFullCorpus(t)
	_, full := scanCorpus(t, corp)
	baseline := BaselineFromProfile(full)

	path := filepath.Join(t.TempDir(), "baseline.yaml")

	writeErr := os.WriteFile(path, []byte("schema_version: 999\ncovered: []\n"), 0o644)
	if writeErr != nil {
		t.Fatalf("seeding invalid baseline: %v", writeErr)
	}

	err := baseline.Write(path, nil, false)
	if err == nil {
		t.Fatal("writing over an invalid existing baseline must fail without -rebaseline")
	}

	err = baseline.Write(path, nil, true)
	if err != nil {
		t.Fatalf("explicit rebaseline over an invalid file must succeed: %v", err)
	}
}

// TestCrossChecklistNoCreditBleed pins that an identical Q text in two
// different checklists never cross-credits: covering it in one leaves
// the other uncovered.
func TestCrossChecklistNoCreditBleed(t *testing.T) {
	t.Parallel()

	corp := newCorpus(t)
	corp.write(filepath.Join(corp.Checklists, "us-other.yaml"), miniChecklist)

	run := corp.addFixture("S1", "mini-pass", "us mini 9.9")
	corp.withChecklist(run, miniChecklist)
	corp.evalCell(run, 1, "pass")
	corp.logLines(run, lineLoaded())

	_, profile := scanCorpus(t, corp)

	for _, covered := range profile.CoveredIDs() {
		if strings.HasPrefix(covered, "us-other/") {
			t.Errorf("us-other must stay uncovered, got %s", covered)
		}
	}

	if len(profile.CoveredIDs()) != 1 {
		t.Errorf("exactly one branch must be covered, got %v", profile.CoveredIDs())
	}
}

// TestBaselineWriteRefusesHardDiagnostics pins the fail-closed rule.
func TestBaselineWriteRefusesHardDiagnostics(t *testing.T) {
	t.Parallel()

	baseline := &Baseline{SchemaVersion: baselineSchemaVersion}

	err := baseline.Write(filepath.Join(t.TempDir(), "b.yaml"),
		[]string{"fixture-x: broken"}, false)
	if err == nil || !errors.Is(err, errHardDiagnostics) {
		t.Fatalf("want hard-diagnostics refusal, got %v", err)
	}
}

// TestShippedBaselineValidity is the cheap PR gate: every checked-in
// baseline entry must still map onto the current shipped universe by
// machine hashes, and the baseline must cover the entire universe.
func TestShippedBaselineValidity(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)

	baseline, err := LoadBaseline(filepath.Join(root, "pkg", "testkit", "cmd", "coverage", "baseline.yaml"))
	if err != nil {
		t.Fatalf("checked-in baseline is required and must load: %v", err)
	}

	uni, err := LoadUniverse(filepath.Join(root, "true-bdd", "checklists"))
	if err != nil {
		t.Fatalf("universe: %v", err)
	}

	known := map[string]bool{}
	for _, b := range uni.Branches {
		known[b.Checklist+"/"+b.QHash+"+"+b.FHash+"#"+string(b.Kind)] = true
	}

	for _, entry := range baseline.Covered {
		key := entryKey(entry.ID, entry.QSHA256, entry.FSHA256)
		if !known[key] {
			t.Errorf("baselined branch %s no longer maps to any shipped prompt "+
				"(Q/F edited or deleted) — re-verify its fixtures and regenerate the baseline",
				entry.ID)
		}
	}

	if baseline.EvidenceQuality != evidenceQualityReconstructed {
		t.Errorf("bootstrap baseline must be labeled %q, got %q",
			evidenceQualityReconstructed, baseline.EvidenceQuality)
	}

	if len(baseline.Covered) == 0 {
		t.Error("baseline has no covered entries")
	}

	assertBaselineComplete(t, baseline, uni)
}

// assertBaselineComplete requires the baseline to be pinned to the
// current universe revision and to cover every one of its branches.
func assertBaselineComplete(t *testing.T, baseline *Baseline, uni *Universe) {
	t.Helper()

	if baseline.UniverseSHA256 != uni.SHA256() {
		t.Errorf("baseline universe_sha256 %s does not match the current universe %s — "+
			"shipped checklists changed; re-run the fixtures and regenerate the baseline",
			baseline.UniverseSHA256, uni.SHA256())
	}

	covered := map[string]bool{}
	for _, entry := range baseline.Covered {
		covered[entry.ID] = true
	}

	for _, branch := range uni.Branches {
		if !covered[branch.ID()] {
			t.Errorf("universe branch %s is not in the baseline — coverage regressed; "+
				"add or re-run the fixture that exercises it and regenerate the baseline",
				branch.ID())
		}
	}
}
