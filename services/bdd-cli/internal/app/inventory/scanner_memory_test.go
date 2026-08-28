package inventory_test

// Finding 2 (aggregate-memory bound + quadratic-remarshal kill): the
// incremental scan must not retain the whole folder before fitting, nor
// re-marshal per dropped item (t.Setenv below forbids t.Parallel).

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/inventory"
)

// manyLargeStoriesFolder writes one canonical epic declaring n stories,
// each backed by a ~fileKiB KiB story file — large enough that the full
// snapshot dwarfs any small test budget.
func manyLargeStoriesFolder(t *testing.T, storyCount, fileKiB int) string {
	t.Helper()

	root := t.TempDir()
	write := func(rel, content string) {
		path := filepath.Join(root, rel)

		mkdirErr := disk.Dir(filepath.Dir(path), disk.Shared)
		if mkdirErr != nil {
			t.Fatalf("mkdir %s: %v", rel, mkdirErr)
		}

		writeErr := disk.Write(path, []byte(content), disk.Shared)
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
		write(fmt.Sprintf("docs/product/stories/%s-big.yaml", id), fmt.Sprintf(
			"story:\n  id: %q\n  title: Big %d\n  as_a: user\n  i_want: x\n  so_that: y\n"+
				"  acceptance_criteria:\n    - id: AC-1\n      description: %q\n",
			id, index, body,
		))
	}

	write("docs/product/epics/epic-70-big.yaml", epic.String())

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

	// Linear scan stays a small multiple of on-disk size; the old build-all +
	// per-drop whole-snapshot re-marshal loop would allocate on the order of
	// stories×snapshot (hundreds of MiB→GiB here) — 200 catches that regression.
	const ceilingMiB = 200
	if allocatedMiB > ceilingMiB {
		t.Fatalf("scan allocated %.1f MiB, want <= %d MiB (quadratic re-marshal regression?)", allocatedMiB, ceilingMiB)
	}
}
