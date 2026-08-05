package inventory_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ondatra-ai/true-bdd/src/internal/app/inventory"
	storymodel "github.com/ondatra-ai/true-bdd/src/internal/domain/models/story"
	"gopkg.in/yaml.v3"
)

// This file is the inverted honesty probe (was tmp/codex_inventory_probe_test.gotxt,
// which asserted the BROKEN behavior). Every assertion here pins the
// CORRECT loader-mirroring surface (MUST-FIX #4): a missing/invalid
// registry taints the applied cell, a story the real typed model rejects
// is created=invalid, an architecture with no services is invalid, only
// canonical epic filenames are Create-addressable, and duplicate declared
// ids are detected snapshot-wide. Trees are built in-code so the golden
// cases need no Playwright fixture.

// Shared in-code fixture paths (canonical host layout under a tmpdir).
const (
	honestyEpicPath  = "docs/prd/epics/epic-70-probe.yaml"
	honestyStoryPath = "docs/prd/stories/70.1-probe.yaml"
	honestyRegPath   = "docs/scenarios.yaml"
)

const honestyEpic70 = `epic:
  id: 70
stories:
  - id: "70.1"
`

// honestyValidStory is apply-eligible: canonical AC-with-steps, non-empty
// internal id, non-deprecated — so only the registry status decides the
// applied cell.
const honestyValidStory = `story:
  id: "70.1"
  acceptance_criteria:
    - id: AC-1
      description: rule
      steps:
        - given:
            - precondition
`

// honestyScalarStepStory carries a scalar step (`- 42`) the real
// story.ScenarioStep decoder rejects — the scanner must mirror that as
// created=invalid, not created=one with a counted lineage.
const honestyScalarStepStory = `story:
  id: "70.1"
  acceptance_criteria:
    - id: AC-1
      description: rule
      steps:
        - 42
`

// writeInto writes content at root/rel, creating parent directories.
func writeInto(t *testing.T, root, rel, content string) {
	t.Helper()

	path := filepath.Join(root, rel)

	err := os.MkdirAll(filepath.Dir(path), 0o755)
	if err != nil {
		t.Fatalf("mkdir for %s: %v", rel, err)
	}

	err = os.WriteFile(path, []byte(content), 0o600)
	if err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

// honestyTree materializes an in-code host folder from a rel->content map
// and returns its root.
func honestyTree(t *testing.T, files map[string]string) string {
	t.Helper()

	root := t.TempDir()
	for rel, content := range files {
		writeInto(t, root, rel, content)
	}

	return root
}

// wantUnknown asserts a story-applied cell is unknown(<reason>) with no
// counted x/y leaking through.
func wantUnknown(t *testing.T, story inventory.Story, reason string) {
	t.Helper()

	if story.Applied.Status != inventory.AppliedUnknown {
		t.Fatalf("story %s applied status = %q, want unknown", story.CreateID, story.Applied.Status)
	}

	if story.Applied.Reason != reason {
		t.Fatalf("story %s applied reason = %q, want %q", story.CreateID, story.Applied.Reason, reason)
	}

	if story.Applied.Applied != 0 || story.Applied.Total != 0 {
		t.Fatalf("story %s applied leaked counted %+v, want none", story.CreateID, story.Applied)
	}
}

func TestHonestyMissingRegistryTaintsAppliedCell(t *testing.T) {
	t.Parallel()

	folder := honestyTree(t, map[string]string{
		honestyEpicPath:  honestyEpic70,
		honestyStoryPath: honestyValidStory,
	})

	snap := inventory.Scan(folder)
	wantDoc(t, snap, "registry", inventory.StatusMissing)

	story := findStory(t, snap, "70.1")
	if story.Created != inventory.CreatedOne {
		t.Fatalf("created = %q, want one", story.Created)
	}

	// Was rendered counted 0/1 by the broken scanner; now tainted.
	wantUnknown(t, story, inventory.ReasonRegistryMissing)
}

func TestHonestyInvalidRegistryTaintsAppliedCell(t *testing.T) {
	t.Parallel()

	folder := honestyTree(t, map[string]string{
		honestyEpicPath:  honestyEpic70,
		honestyStoryPath: honestyValidStory,
		honestyRegPath:   "scenarios:\n  broken: [unclosed\n",
	})

	snap := inventory.Scan(folder)
	wantDoc(t, snap, "registry", inventory.StatusInvalid)

	story := findStory(t, snap, "70.1")
	wantUnknown(t, story, inventory.ReasonRegistryInvalid)
}

func TestHonestyPresentEmptyRegistryStaysCounted(t *testing.T) {
	t.Parallel()

	folder := honestyTree(t, map[string]string{
		honestyEpicPath:  honestyEpic70,
		honestyStoryPath: honestyValidStory,
		honestyRegPath:   "scenarios: {}\n",
	})

	snap := inventory.Scan(folder)
	wantDoc(t, snap, "registry", inventory.StatusPresentEmpty)

	story := findStory(t, snap, "70.1")
	if story.Applied != (inventory.Applied{Status: inventory.AppliedCounted, Applied: 0, Total: 1}) {
		t.Fatalf("present_empty applied = %+v, want counted 0/1", story.Applied)
	}
}

func TestHonestySemanticInvalidStoryIsCreatedInvalid(t *testing.T) {
	t.Parallel()

	folder := honestyTree(t, map[string]string{
		honestyEpicPath:  honestyEpic70,
		honestyStoryPath: honestyScalarStepStory,
	})

	snap := inventory.Scan(folder)

	story := findStory(t, snap, "70.1")
	if story.Created != inventory.CreatedInvalid {
		t.Fatalf("scalar-step story created = %q, want invalid", story.Created)
	}

	// Was rendered created=one by the broken scanner; now invalid.
	wantUnknown(t, story, inventory.ReasonInvalid)

	// Prove the scanner now mirrors the real typed model: it rejects the
	// same scalar scenario step.
	var doc storymodel.StoryDocument

	err := yaml.Unmarshal([]byte(honestyScalarStepStory), &doc)
	if err == nil {
		t.Fatal("real story model unexpectedly accepted a scalar scenario step")
	}
}

func TestHonestyArchitectureMirrorsBuildCodeLoader(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		content string
		status  string
	}{
		{"no_services", "architecture:\n  services: []\n", inventory.StatusInvalid},
		{"absent_services", "architecture:\n  vocabulary: {}\n", inventory.StatusInvalid},
		{
			"with_services",
			"architecture:\n  services:\n    - name: svc\n      path: services/svc\n",
			inventory.StatusPresent,
		},
		// The workspace file-as-source schema (task workspace-overview-design-parity,
		// R3): a top-level `services:` MAP, no `architecture:` wrapper — mirrors
		// harness/src/app/lib/workspace/derive.ts's deriveArchitecture. Both schemas
		// must classify correctly since the SAME inventory chip serves the protocol
		// detail page AND the workspace overview canvas.
		{"workspace_schema_no_services", "services: {}\n", inventory.StatusInvalid},
		{
			"workspace_schema_with_services",
			"services:\n  mcp-service:\n    type: custom\n",
			inventory.StatusPresent,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			folder := honestyTree(t, map[string]string{
				"docs/architecture/architecture.yaml": testCase.content,
			})
			wantDoc(t, inventory.Scan(folder), "architecture", testCase.status)
		})
	}
}

func TestHonestyEpicCanonicalEncoding(t *testing.T) {
	t.Parallel()

	cases := []struct {
		file         string
		noncanonical bool
	}{
		{"epic-07-a.yaml", false},  // %02d of 7 is "07"
		{"epic-42-b.yaml", false},  // %02d of 42 is "42"
		{"epic-100-c.yaml", false}, // %02d of 100 is "100"
		{"epic-7-d.yaml", true},    // "7" != "07"
		{"epic-042-e.yaml", true},  // "042" != "42"
	}

	for _, testCase := range cases {
		t.Run(testCase.file, func(t *testing.T) {
			t.Parallel()

			folder := honestyTree(t, map[string]string{
				filepath.Join("docs/prd/epics", testCase.file): "epic:\n  id: 1\n",
			})
			epic := findEpic(t, inventory.Scan(folder), testCase.file)

			if epic.NoncanonicalFilename != testCase.noncanonical {
				t.Fatalf("%s noncanonical_filename = %v, want %v",
					testCase.file, epic.NoncanonicalFilename, testCase.noncanonical)
			}
		})
	}
}

func TestHonestyNoncanonicalEpicHasNoAddressableStories(t *testing.T) {
	t.Parallel()

	folder := honestyTree(t, map[string]string{
		"docs/prd/epics/epic-7-noncanon.yaml": "epic:\n  id: 7\nstories:\n  - id: \"7.1\"\n",
		"docs/prd/epics/epic-42-canon.yaml":   "epic:\n  id: 42\nstories:\n  - id: \"42.1\"\n",
	})

	snap := inventory.Scan(folder)

	noncanon := findEpic(t, snap, "epic-7-noncanon.yaml")
	if !noncanon.NoncanonicalFilename {
		t.Fatal("epic-7-noncanon.yaml expected noncanonical_filename")
	}

	if len(noncanon.Stories) != 0 {
		t.Fatalf("noncanonical epic has %d story rows, want 0 (us create cannot resolve it)", len(noncanon.Stories))
	}

	canon := findEpic(t, snap, "epic-42-canon.yaml")
	if canon.NoncanonicalFilename {
		t.Fatal("epic-42-canon.yaml should be canonical")
	}

	if len(canon.Stories) == 0 {
		t.Fatal("canonical epic should carry Create-addressable story rows")
	}
}

func TestHonestySnapshotWideDuplicateDeclaredID(t *testing.T) {
	t.Parallel()

	folder := honestyTree(t, map[string]string{
		"docs/prd/epics/epic-71-a.yaml": "epic:\n  id: 71\nstories:\n  - id: \"99.1\"\n",
		"docs/prd/epics/epic-72-b.yaml": "epic:\n  id: 72\nstories:\n  - id: \"99.1\"\n",
	})

	snap := inventory.Scan(folder)

	for _, createID := range []string{"71.1", "72.1"} {
		story := findStory(t, snap, createID)
		if !story.Flags.DuplicateDeclaredID {
			t.Fatalf("story %s expected snapshot-wide duplicate_declared_id flag", createID)
		}
	}
}
