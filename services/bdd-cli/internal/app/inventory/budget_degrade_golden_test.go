package inventory_test

// Size-discipline goldens (plan §1a/§4 "Go"): a snapshot that fits is
// carried whole, one that does not collapses to the always-fit FLOOR, and a
// budget below the floor is the cannot-fit state. t.Setenv forbids t.Parallel.

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/inventory"
)

const budgetEnvVar = "TRUE_BDD_INVENTORY_BUDGET_BYTES"

// longPathFolder configures a ~4 KiB architecture path, an unbounded string
// the full snapshot carries and the floor drops, so the full snapshot is far
// larger than its document-chip floor.
func longPathFolder(t *testing.T) string {
	t.Helper()

	config := "documents:\n  architecture_yaml: \"docs/" + strings.Repeat("x", 4096) + "/architecture.yaml\"\n"

	return honestyTree(t, map[string]string{"true-bdd/true-bdd.yaml": config})
}

// serialized is the JSON length the remote would upload for a snapshot.
func serialized(t *testing.T, snap inventory.Snapshot) int {
	t.Helper()

	data, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}

	return len(data)
}

func TestGoldenUnlimitedBudgetKeepsWholeSnapshot(t *testing.T) {
	snap := inventory.Scan(longPathFolder(t))

	if snap.SnapshotTruncated || snap.Unavailable != "" {
		t.Fatalf("unlimited scan degraded: truncated=%v unavailable=%q", snap.SnapshotTruncated, snap.Unavailable)
	}

	if !strings.HasSuffix(snap.ConfiguredArchitecturePath, "/architecture.yaml") {
		t.Fatalf("unlimited scan dropped the configured architecture path, got %q", snap.ConfiguredArchitecturePath)
	}
}

func TestGoldenOverBudgetCollapsesToFloor(t *testing.T) {
	folder := longPathFolder(t)
	budget := inventory.MinInventoryFloor + 64
	t.Setenv(budgetEnvVar, strconv.Itoa(budget))

	snap := inventory.Scan(folder)

	if !snap.SnapshotTruncated {
		t.Fatal("over-budget snapshot_truncated = false, want true")
	}

	if snap.ConfiguredArchitecturePath != "" {
		t.Fatal("floor retains the configured architecture path, an unbounded string")
	}

	if !snap.ArchitecturePathMismatch {
		t.Fatal("floor dropped architecture_path_mismatch, a bounded flag it must keep")
	}

	wantDoc(t, snap, "config", inventory.StatusPresent)

	if got := serialized(t, snap); got > budget {
		t.Fatalf("floor serializes to %d bytes, over the %d budget", got, budget)
	}
}

func TestGoldenBelowFloorIsCannotFit(t *testing.T) {
	folder := longPathFolder(t)
	// A budget below MIN_INVENTORY_FLOOR (the envelope + floor worst case) is
	// a TERMINAL cannot-fit — reported honestly, never halved forever.
	t.Setenv(budgetEnvVar, "1")

	snap := inventory.Scan(folder)

	if snap.Unavailable != "limit_too_small" {
		t.Fatalf("below-floor unavailable = %q, want limit_too_small", snap.Unavailable)
	}

	if snap.Documents != nil {
		t.Fatalf("cannot-fit snapshot retains %d document chips, want none", len(snap.Documents))
	}
}
