package inventory_test

// Finding 2 (aggregate-memory bound + quadratic kill): the incremental
// scan must NOT retain the whole folder before fitting, and must NOT
// re-marshal the whole snapshot once per dropped item (the previous O(n²)
// degrade loop). These invariants are exercised on a pathological folder —
// many large story files under a tiny budget — and asserted two ways:
//
//   1. The RETAINED snapshot is bounded by the negotiated budget (its
//      serialized body is <= budget) and the retained raw/content across all
//      rows never exceeds budget + one decoded file.
//   2. The total bytes ALLOCATED by one Scan stay linear in the folder size
//      (bounded well below what an all-in-memory build + per-drop whole-
//      snapshot re-marshal would allocate — that path is quadratic and would
//      allocate on the order of files×passes, i.e. hundreds of MB→GB here).
//
// t.Setenv forbids t.Parallel.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/inventory"
)

// manyLargeStoriesFolder writes one canonical epic declaring n stories, each
// backed by a story file whose description is ~fileKiB KiB, so the full
// snapshot is far larger than any small budget and the all-in-memory build
// would retain n×fileKiB before fitting.
func manyLargeStoriesFolder(t *testing.T, storyCount, fileKiB int) string {
	t.Helper()

	root := t.TempDir()
	write := func(rel, content string) {
		path := filepath.Join(root, rel)

		mkdirErr := os.MkdirAll(filepath.Dir(path), 0o755)
		if mkdirErr != nil {
			t.Fatalf("mkdir %s: %v", rel, mkdirErr)
		}

		writeErr := os.WriteFile(path, []byte(content), 0o600)
		if writeErr != nil {
			t.Fatalf("write %s: %v", rel, writeErr)
		}
	}

	epic := &strings.Builder{}
	epic.WriteString("epic:\n  id: 70\n  name: Many Large\nstories:\n")

	body := strings.Repeat("word ", fileKiB*1024/5) // ~fileKiB KiB per description

	for index := 1; index <= storyCount; index++ {
		id := fmt.Sprintf("70.%d", index)
		fmt.Fprintf(epic, "  - id: %q\n", id)
		write(fmt.Sprintf("docs/prd/stories/%s-big.yaml", id), fmt.Sprintf(
			"story:\n  id: %q\n  title: Big %d\n  as_a: user\n  i_want: x\n  so_that: y\n"+
				"  acceptance_criteria:\n    - id: AC-1\n      description: %q\n",
			id, index, body,
		))
	}

	write("docs/prd/epics/epic-70-big.yaml", epic.String())

	return root
}

// retainedPayloadBytes sums the retained raw + serialized content bytes
// across every story row — the scanner's aggregate retained state.
func retainedPayloadBytes(t *testing.T, snap inventory.Snapshot) int {
	t.Helper()

	total := 0

	for _, epic := range snap.Epics {
		for _, story := range epic.Stories {
			total += len(story.Raw)
			if story.Content != nil {
				data, err := json.Marshal(story.Content)
				if err != nil {
					t.Fatalf("marshal content: %v", err)
				}

				total += len(data)
			}
		}
	}

	return total
}

func TestScanRetainedStateBoundedByBudget(t *testing.T) {
	const (
		stories = 200
		fileKiB = 48
		budget  = 8000
	)

	folder := manyLargeStoriesFolder(t, stories, fileKiB)
	t.Setenv("TRUE_BDD_INVENTORY_BUDGET_BYTES", strconv.Itoa(budget))

	snap := inventory.Scan(folder)

	// The retained snapshot's serialized body honours the budget.
	body, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}

	if len(body) > budget {
		t.Fatalf("retained snapshot = %d bytes, want <= budget %d", len(body), budget)
	}

	// The retained raw+content across all rows is bounded by the budget plus
	// at most one decoded file (never the whole folder).
	maxFileBytes := fileKiB * 1024 * 2 // generous ceiling for one decoded file

	retained := retainedPayloadBytes(t, snap)
	if retained > budget+maxFileBytes {
		t.Fatalf("retained raw+content = %d bytes, want <= budget+oneFile %d", retained, budget+maxFileBytes)
	}

	// The true global counts survive regardless of how far the ladder walked.
	if snap.Totals.DeclaredStories != stories {
		t.Fatalf("totals.declared_stories = %d, want %d", snap.Totals.DeclaredStories, stories)
	}
}

func TestScanAllocationIsLinearNotQuadratic(t *testing.T) {
	const (
		stories = 200
		fileKiB = 48
		budget  = 8000
	)

	folder := manyLargeStoriesFolder(t, stories, fileKiB)
	t.Setenv("TRUE_BDD_INVENTORY_BUDGET_BYTES", strconv.Itoa(budget))

	var before, after runtime.MemStats

	runtime.GC()
	runtime.ReadMemStats(&before)

	snap := inventory.Scan(folder)

	runtime.ReadMemStats(&after)

	// Touch the result so the scan is not elided.
	if snap.Totals.Epics != 1 {
		t.Fatalf("totals.epics = %d, want 1", snap.Totals.Epics)
	}

	allocatedMiB := float64(after.TotalAlloc-before.TotalAlloc) / (1024 * 1024)
	onDiskMiB := float64(stories*fileKiB) / 1024
	t.Logf("scan allocated %.1f MiB for a %.1f MiB folder (budget %d)", allocatedMiB, onDiskMiB, budget)

	// The incremental scan reads each file once and marshals only per-story;
	// it stays a small multiple of the on-disk size. The old build-all +
	// per-drop whole-snapshot re-marshal loop would allocate on the order of
	// stories×snapshot (hundreds of MiB→GiB here). A ceiling well below that
	// catches a regression to the quadratic path.
	const ceilingMiB = 200
	if allocatedMiB > ceilingMiB {
		t.Fatalf("scan allocated %.1f MiB, want <= %d MiB (quadratic re-marshal regression?)", allocatedMiB, ceilingMiB)
	}
}
