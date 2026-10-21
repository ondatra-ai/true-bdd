package inventory_test

import (
	"testing"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/inventory"
)

// Chip keys more than one test asserts.
const (
	chipRegistry   = "registry"
	chipStoriesDir = "stories-dir"
)

// scanFixture materializes a harness fixture and scans it.
func scanFixture(t *testing.T, fixture string) inventory.Snapshot {
	t.Helper()

	return inventory.Scan(materialize(t, fixture))
}

// wantDoc asserts a chip's status.
func wantDoc(t *testing.T, snap inventory.Snapshot, key, status string) {
	t.Helper()

	got, ok := snap.Documents[key]
	if !ok {
		t.Fatalf("document chip %q not found in snapshot", key)
	}

	if got != status {
		t.Fatalf("chip %q status = %q, want %q (error=%q)", key, got, status, snap.DocumentErrors[key])
	}
}

func TestScanDocumentChips(t *testing.T) {
	t.Parallel()

	cases := []struct {
		fixture string
		key     string
		status  string
	}{
		{"p4-config-invalid", "config", inventory.StatusInvalid},
		{"p4-stories-not-a-dir", chipStoriesDir, inventory.StatusNotADir},
		{"p4-checklist-missing", "checklist-us-apply", inventory.StatusMissing},
		{"p4-checklist-invalid", "checklist-us-refine", inventory.StatusInvalid},
		{"p4-registry-present-empty", chipRegistry, inventory.StatusPresentEmpty},
	}

	for _, testCase := range cases {
		t.Run(testCase.fixture, func(t *testing.T) {
			t.Parallel()

			snap := scanFixture(t, testCase.fixture)
			wantDoc(t, snap, testCase.key, testCase.status)
		})
	}
}

func TestScanChecklistSiblingsStayPresent(t *testing.T) {
	t.Parallel()

	missing := scanFixture(t, "p4-checklist-missing")
	wantDoc(t, missing, "checklist-us-apply", inventory.StatusMissing)
	wantDoc(t, missing, "checklist-us-refine", inventory.StatusPresent)

	invalid := scanFixture(t, "p4-checklist-invalid")
	wantDoc(t, invalid, "checklist-us-refine", inventory.StatusInvalid)
	wantDoc(t, invalid, "checklist-us-apply", inventory.StatusPresent)
}

// TestScanReportsNoEpicChips pins ADR 0013: epics left the pipeline, so the
// scan names neither an epics directory nor a us-create checklist.
func TestScanReportsNoEpicChips(t *testing.T) {
	t.Parallel()

	snap := scanFixture(t, "p3-inventory-spread")

	for _, key := range []string{"epics-dir", "checklist-us-create"} {
		if status, ok := snap.Documents[key]; ok {
			t.Errorf("snapshot carries chip %q (status %q), want none", key, status)
		}
	}
}

func TestScanArchitecturePathMismatch(t *testing.T) {
	t.Parallel()

	snap := scanFixture(t, "p4-architecture-path-mismatch")

	if !snap.ArchitecturePathMismatch {
		t.Fatal("expected architecture_path_mismatch = true")
	}

	if snap.ConfiguredArchitecturePath != "docs/arch/architecture.yaml" {
		t.Fatalf("configured architecture path = %q", snap.ConfiguredArchitecturePath)
	}

	wantDoc(t, snap, "architecture", inventory.StatusPresent)
}
