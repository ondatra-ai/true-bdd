package inventory_test

import (
	"path/filepath"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/inventory"
)

// These tests pin the scanner's config-driven paths (paths.stories_dir,
// documents.scenarios_yaml). Unlike the engine, which refuses on a missing
// key, the scanner falls back to canonical defaults — a scan never fails.

const configFileRel = "true-bdd/true-bdd.yaml"

const configRegistry = `scenarios:
  SCEN-80-1:
    user_stories:
      - story: "docs/product/stories/80.1-config.yaml"
        scenario_id: "80.1-001"
`

const configStory = "story:\n  id: \"80.1\"\n"

func writeConfigTree(t *testing.T, files map[string]string) string {
	t.Helper()

	folder := t.TempDir()

	for rel, content := range files {
		path := filepath.Join(folder, rel)

		err := disk.Dir(filepath.Dir(path), disk.Shared)
		if err != nil {
			t.Fatalf("MkdirAll %s: %v", path, err)
		}

		err = disk.Write(path, []byte(content), disk.Shared)
		if err != nil {
			t.Fatalf("WriteFile %s: %v", path, err)
		}
	}

	return folder
}

// wantPresent asserts the registry and stories-dir chips both read present.
func wantPresent(t *testing.T, snapshot inventory.Snapshot, why string) {
	t.Helper()

	for _, key := range []string{chipRegistry, chipStoriesDir} {
		if got := snapshot.Documents[key]; got != inventory.StatusPresent {
			t.Errorf("%s status = %q, want %q (%s)", key, got, inventory.StatusPresent, why)
		}
	}
}

// TestScanHonorsConfiguredStoriesAndRegistryPaths relocates BOTH keys away
// from the canonical layout. Nothing exists at the default locations, so
// any leftover hardcode would report the stories dir and registry missing.
func TestScanHonorsConfiguredStoriesAndRegistryPaths(t *testing.T) {
	t.Parallel()

	folder := writeConfigTree(t, map[string]string{
		configFileRel: `paths:
  stories_dir: "./planning/stories"
documents:
  scenarios_yaml: "./planning/registry.yaml"
`,
		"planning/stories/80.1-config.yaml": configStory,
		"planning/registry.yaml":            configRegistry,
	})

	wantPresent(t, inventory.Scan(folder), "configured paths")
}

// TestScanFallsBackToCanonicalPathsWhenKeysAbsent: a present config that
// omits both keys must degrade to the canonical layout, not to nothing.
func TestScanFallsBackToCanonicalPathsWhenKeysAbsent(t *testing.T) {
	t.Parallel()

	folder := writeConfigTree(t, map[string]string{
		configFileRel:                           "paths:\n  checklists_dir: \"true-bdd/checklists\"\n",
		"docs/product/stories/80.1-config.yaml": configStory,
		"docs/scenarios.yaml":                   configRegistry,
	})

	wantPresent(t, inventory.Scan(folder), "canonical defaults")
}

// TestScanFallsBackToCanonicalPathsOnBlankValues: whitespace-only values
// count as unset — same fallback, never an empty join.
func TestScanFallsBackToCanonicalPathsOnBlankValues(t *testing.T) {
	t.Parallel()

	folder := writeConfigTree(t, map[string]string{
		configFileRel: `paths:
  stories_dir: "   "
documents:
  scenarios_yaml: ""
`,
		"docs/product/stories/80.1-config.yaml": configStory,
		"docs/scenarios.yaml":                   configRegistry,
	})

	wantPresent(t, inventory.Scan(folder), "blank keys fall back to defaults")
}
