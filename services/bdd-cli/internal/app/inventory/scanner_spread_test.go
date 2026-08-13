package inventory_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/inventory"
)

// registryAppend602 covers lineage 60.2-002 with the EXACT story path,
// flipping 60.2 from 1/2 to 2/2 — the worker-side mutation P3 drives
// through Refresh (p3-inventory-refresh.spec.ts).
const registryAppend602 = `  E2E-604:
    description: "Not-shared documents surface the sharing message"
    service: "mcp-service"
    last_updated: "2026-07-29"
    user_stories:
      - story: "docs/prd/stories/60.2-summary-shared-docs.yaml"
        scenario_id: "60.2-002"
        merge_date: "2026-07-29"
    merged_steps:
      given:
        - "a Google Doc exists that is not shared with the Claude User's account"
`

// wantApplied asserts a countable x/y story-applied cell.
func wantApplied(t *testing.T, story inventory.Story, want inventory.Applied) {
	t.Helper()

	want.Status = inventory.AppliedCounted

	if story.Applied != want {
		t.Fatalf("story %s applied = %+v, want %+v", story.CreateID, story.Applied, want)
	}
}

func TestScanSpreadDocuments(t *testing.T) {
	t.Parallel()

	snap := scanFixture(t, "p3-inventory-spread")

	for _, key := range []string{"config", "prd", "architecture", "registry", "checklist-us-apply"} {
		wantDoc(t, snap, key, inventory.StatusPresent)
	}
}

func TestScanSpreadStoryStates(t *testing.T) {
	t.Parallel()

	snap := scanFixture(t, "p3-inventory-spread")

	missing := findStory(t, snap, "60.1")
	if missing.Created != inventory.CreatedMissing {
		t.Fatalf("60.1 created = %q, want missing", missing.Created)
	}

	if missing.Applied.Status != inventory.AppliedUnknown || missing.Applied.Reason != inventory.ReasonMissing {
		t.Fatalf("60.1 applied = %+v, want unknown(missing)", missing.Applied)
	}

	if missing.Refined != inventory.RefinedNotRecorded {
		t.Fatalf("60.1 refined = %q, want not_recorded", missing.Refined)
	}

	partial := findStory(t, snap, "60.2")
	if partial.Created != inventory.CreatedOne {
		t.Fatalf("60.2 created = %q, want one", partial.Created)
	}

	wantApplied(t, partial, inventory.Applied{Applied: 1, Total: 2})

	full := findStory(t, snap, "60.3")
	wantApplied(t, full, inventory.Applied{Applied: 2, Total: 2})
}

func TestScanSpreadRefreshPicksUpRegistryMutation(t *testing.T) {
	t.Parallel()

	folder := materialize(t, "p3-inventory-spread")

	before := findStory(t, inventory.Scan(folder), "60.2")
	wantApplied(t, before, inventory.Applied{Applied: 1, Total: 2})

	appendRegistry(t, filepath.Join(folder, "docs", "scenarios.yaml"), registryAppend602)

	after := findStory(t, inventory.Scan(folder), "60.2")
	wantApplied(t, after, inventory.Applied{Applied: 2, Total: 2})
}

// appendRegistry appends text to the registry file, mirroring the
// worker-side mutation P3 drives before clicking Refresh.
func appendRegistry(t *testing.T, path, text string) {
	t.Helper()

	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open registry: %v", err)
	}

	_, writeErr := file.WriteString(text)
	closeErr := file.Close()

	if writeErr != nil {
		t.Fatalf("append registry: %v", writeErr)
	}

	if closeErr != nil {
		t.Fatalf("close registry: %v", closeErr)
	}
}
