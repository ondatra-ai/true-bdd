package inventory_test

// Size-discipline goldens (plan §1a/§4 "Go"): the request-budget degrade
// ladder, the always-fit FLOOR, the below-floor cannot-fit state, and the
// whole-file decoding parity for oversized epics.
//
// The degrade ladder is driven through a PINNED env knob,
// TRUE_BDD_INVENTORY_BUDGET_BYTES — the fit budget the remote derives from
// the server's negotiated inventory limit, exposed as an env override so
// the ladder is unit-testable without a running server. The implementer
// wires the fit step (inside Scan or a Scan-internal helper) to honor it.
// Compile-safe: every assertion decodes the REAL Scan output into the local
// target schema (content_extraction_golden_test.go), so the tests fail
// (red) — Scan ignores the budget today — and go green once the ladder
// emits the omission metadata. t.Setenv forbids t.Parallel here.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/src/internal/app/inventory"
)

const budgetEnvVar = "TRUE_BDD_INVENTORY_BUDGET_BYTES"

// bigStoryFolder builds one canonical epic declaring `n` stories, each
// backed by a story file whose ~20 KiB description makes the full snapshot
// far exceed any small budget — forcing the degrade ladder.
func bigStoryFolder(t *testing.T, storyCount int) (string, int) {
	t.Helper()

	files := map[string]string{}

	epic := &strings.Builder{}
	epic.WriteString("epic:\n  id: 70\n  name: Big Epic\nstories:\n")

	body := strings.Repeat("word ", 4096) // ~20 KiB per story description

	for index := 1; index <= storyCount; index++ {
		id := fmt.Sprintf("70.%d", index)
		fmt.Fprintf(epic, "  - id: %q\n", id)
		files[fmt.Sprintf("docs/prd/stories/%s-big.yaml", id)] = fmt.Sprintf(
			"story:\n  id: %q\n  title: Big %d\n  as_a: user\n  i_want: x\n  so_that: y\n"+
				"  acceptance_criteria:\n    - id: AC-1\n      description: %q\n",
			id, index, body,
		)
	}

	files["docs/prd/epics/epic-70-big.yaml"] = epic.String()

	return honestyTree(t, files), storyCount
}

func TestGoldenDegradeOrderRawBeforeContent(t *testing.T) {
	folder, _ := bigStoryFolder(t, 8)
	// A very small budget forces heavy degradation.
	t.Setenv(budgetEnvVar, "2000")

	snap := scanJSON(t, folder)

	if !snap.SnapshotTruncated {
		t.Fatal("snapshot_truncated = false under a tiny budget, want true")
	}

	// Degrade ORDER invariant: raw is dropped before content, so no story is
	// ever content_omitted while still carrying its raw.
	for _, epic := range snap.Epics {
		for _, story := range epic.Stories {
			if story.ContentOmitted && !story.RawOmitted {
				t.Fatalf("story %s dropped content before raw (violates raw→content order)", story.CreateID)
			}
		}
	}
}

func TestGoldenDegradeFloorDropsPerEpicEntries(t *testing.T) {
	folder, stories := bigStoryFolder(t, 8)
	// A budget below what any per-epic entry needs collapses to the FLOOR:
	// document chips + GLOBAL counts only, NO per-epic entries.
	t.Setenv(budgetEnvVar, "400")

	snap := scanJSON(t, folder)

	if !snap.SnapshotTruncated {
		t.Fatal("floor snapshot_truncated = false, want true")
	}

	if len(snap.Epics) != 0 {
		t.Fatalf("floor retains %d epic entries, want 0 (fixed-size floor)", len(snap.Epics))
	}

	if snap.Totals.Epics != 1 {
		t.Fatalf("floor totals.epics = %d, want 1", snap.Totals.Epics)
	}

	if snap.Totals.DeclaredStories != stories {
		t.Fatalf("floor totals.declared_stories = %d, want %d", snap.Totals.DeclaredStories, stories)
	}
}

func TestGoldenBelowFloorIsCannotFit(t *testing.T) {
	folder, _ := bigStoryFolder(t, 8)
	// A budget below MIN_INVENTORY_FLOOR (the envelope + floor worst case) is
	// a TERMINAL cannot-fit — reported honestly, never halved forever.
	t.Setenv(budgetEnvVar, "1")

	snap := scanJSON(t, folder)

	if snap.Unavailable != "limit_too_small" {
		t.Fatalf("below-floor unavailable = %q, want limit_too_small", snap.Unavailable)
	}

	if len(snap.Epics) != 0 {
		t.Fatalf("cannot-fit snapshot retains %d epics, want 0", len(snap.Epics))
	}
}

func TestGoldenOversizedEpicDecodingParity(t *testing.T) {
	// The 256 KiB cap applies ONLY to retained story raw, never to what is
	// read/decoded: a valid oversized epic decodes IDENTICALLY to us create
	// (parseable, declared content extracted), and an invalid-tail epic
	// beyond 256 KiB is STILL detected as invalid.
	pad := strings.Repeat("x", 300*1024) // > 256 KiB of valid content

	validOversized := "epic:\n  id: 70\n  name: Oversized Epic\n  context: \"" + pad + "\"\n" +
		"stories:\n  - id: \"70.1\"\n    title: Declared\n    as_a: user\n    i_want: want text\n    so_that: y\n" +
		"    acceptance_criteria:\n      - id: AC-1\n        description: rule\n"

	invalidTail := "epic:\n  id: 71\n  name: Oversized Broken\n  context: \"" + pad + "\"\nstories:\n  - id: [unclosed\n"

	folder := honestyTree(t, map[string]string{
		"docs/prd/epics/epic-70-oversized.yaml":     validOversized,
		"docs/prd/epics/epic-71-oversized-bad.yaml": invalidTail,
	})

	snap := scanJSON(t, folder)

	good := gEpicOf(t, snap, "epic-70-oversized.yaml")
	if good.Status != inventory.EpicParseable {
		t.Fatalf("valid oversized epic status = %q, want parseable (whole-file decode)", good.Status)
	}

	// NEW: the declared story's content is extracted from the full document.
	story := gStoryOf(t, snap, "70.1")
	if story.DeclaredContent == nil || story.DeclaredContent.Statement.IWant != "want text" {
		t.Fatal("declared_content must be extracted from an oversized (>256 KiB) epic")
	}

	bad := gEpicOf(t, snap, "epic-71-oversized-bad.yaml")
	if bad.Status != inventory.EpicInvalid {
		t.Fatalf("invalid-tail oversized epic status = %q, want invalid", bad.Status)
	}
}
