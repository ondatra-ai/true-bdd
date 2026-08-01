package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// soloOverrideChecklist reproduces a single-prompt fixture override:
// the run checklist contains ONLY the mini corpus's F-bearing prompt
// (shipped q2), copied verbatim. Its run-local index is therefore 1
// while the shipped global index is 2 — the reindexing case every
// single-prompt fixture relies on.
const soloOverrideChecklist = `
version: "1.0"
sections:
  - id: s
    validation_prompts:
      - Q: "Question two?"
        rationale: "r2"
        docs: [prd]
        F: "Fix it."
`

// buildSoloOverrideCorpus assembles one run that walks the solo
// checklist through a complete fail -> fix -> clean-walk chain, with
// every artifact named by the run-local index 1.
func buildSoloOverrideCorpus(t *testing.T) *corpus {
	t.Helper()

	corp := newCorpus(t)

	run := corp.addFixture("S12", "mini-solo-override", "us mini 9.9")
	corp.withChecklist(run, soloOverrideChecklist)

	// Final surviving verdict: pass (the post-fix re-evaluation).
	corp.evalCell(run, 1, "pass")

	// Pre-fix fail evidence under local index 1 (fixEvidence hardcodes
	// index 2, the full-checklist position).
	part := filepath.Join(run, "tmp", miniPart)
	prefix := fmt.Sprintf("01-%s-fix", miniSubject)

	corp.write(filepath.Join(part, prefix+"-prompts.md"), "generated fix prompt\n")
	corp.write(filepath.Join(part, prefix+"-iter1-user.txt"),
		"### Failed Check: us-mini/s\n\n**Question:** ...\n\n**Actual:** fail\n")

	corp.applyResult(run)

	// Loaded prompts reports the RUN checklist's dimensions (1 item x
	// 1 prompt); the clean post-apply walk needs >1 save after the
	// apply with no further generation.
	corp.logLines(run,
		`{"msg":"Loaded prompts","command":"us mini","items":1,"prompts":1}`,
		lineSaved(1),
		fmt.Sprintf(`{"msg":"Generating fix prompt","subjectID":%q,"promptIndex":1}`, miniSubject),
		lineApplied(),
		lineSaved(1),
		lineSaved(1))

	return corp
}

// TestSoloOverrideEarnsShippedCredit proves a single-prompt override
// run earns full shipped-branch credit for the prompt it copies: pass,
// semantic fail, and the complete fix-effective chain land on shipped
// q2 even though the run-local prompt index is 1.
func TestSoloOverrideEarnsShippedCredit(t *testing.T) {
	t.Parallel()

	_, profile := scanCorpus(t, buildSoloOverrideCorpus(t))

	covered := profile.CoveredIDs()
	joined := strings.Join(covered, "\n")

	if len(covered) != 3 {
		t.Fatalf("covered: got %d (%v), want exactly q2 pass+fail+fix", len(covered), covered)
	}

	for _, id := range covered {
		if !strings.Contains(id, "/q2@") {
			t.Errorf("covered id %s does not belong to q2", id)
		}
	}

	for _, must := range []string{"#pass", "#fail", "#fix"} {
		if !strings.Contains(joined, must) {
			t.Errorf("covered set lacks %q:\n%s", must, joined)
		}
	}

	assertSoloDiagnosticsClean(t, profile)
}

// assertSoloDiagnosticsClean requires the solo run to produce no
// uncredited signals in ANY bucket: the chain must be proven, not
// merely plausible.
func assertSoloDiagnosticsClean(t *testing.T, profile *Profile) {
	t.Helper()

	diags := profile.Diagnostics
	buckets := map[string][]string{
		"protocol fails":       diags.ProtocolFails,
		"non-canonical":        diags.NonCanonical,
		"non-shipped":          diags.NonShipped,
		"fix candidates":       diags.FixCandidates,
		"unattributed applies": diags.UnattributedApply,
		"unclassified fails":   diags.FailUnclassified,
		"run diagnostics":      diags.RunDiagnostics,
		"hard diagnostics":     diags.HardRunDiagnostics,
	}

	for label, bucket := range buckets {
		if len(bucket) != 0 {
			t.Errorf("unexpected %s: %v", label, bucket)
		}
	}
}
