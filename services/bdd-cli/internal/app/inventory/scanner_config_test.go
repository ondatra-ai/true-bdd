package inventory_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/inventory"
)

// These tests pin the scanner's config-driven paths after the engine
// moved them into true-bdd.yaml: paths.epics_dir (was top-level
// epics.path) and documents.scenarios_yaml (was hardcoded engine-wide).
// The scanner must honor configured locations, and — unlike the engine,
// which refuses to run on a missing key — fall back to the canonical
// defaults, because a scan never fails.

const configFileRel = "true-bdd/true-bdd.yaml"

const configEpic80 = `epic:
  id: 80
stories:
  - id: "80.1"
`

const configRegistry = `scenarios:
  SCEN-80-1:
    user_stories:
      - id: "80.1"
`

func writeConfigTree(t *testing.T, files map[string]string) string {
	t.Helper()

	folder := t.TempDir()

	for rel, content := range files {
		path := filepath.Join(folder, rel)

		err := os.MkdirAll(filepath.Dir(path), 0o755)
		if err != nil {
			t.Fatalf("MkdirAll %s: %v", path, err)
		}

		err = os.WriteFile(path, []byte(content), 0o644)
		if err != nil {
			t.Fatalf("WriteFile %s: %v", path, err)
		}
	}

	return folder
}

// TestScanHonorsConfiguredEpicsAndRegistryPaths relocates BOTH keys away
// from the canonical layout. Nothing exists at the default locations, so
// any leftover hardcode would report the epic and registry as missing.
func TestScanHonorsConfiguredEpicsAndRegistryPaths(t *testing.T) {
	t.Parallel()

	folder := writeConfigTree(t, map[string]string{
		configFileRel: `paths:
  epics_dir: "./planning/epics"
documents:
  scenarios_yaml: "./planning/registry.yaml"
`,
		"planning/epics/epic-80-config.yaml": configEpic80,
		"planning/registry.yaml":             configRegistry,
	})

	snapshot := inventory.Scan(folder)

	if got := snapshot.Documents["registry"]; got != inventory.StatusPresent {
		t.Errorf("registry status = %q, want %q", got, inventory.StatusPresent)
	}

	if snapshot.Totals.Epics != 1 {
		t.Errorf("Totals.Epics = %d, want 1 (epic under configured epics_dir)",
			snapshot.Totals.Epics)
	}
}

// TestScanFallsBackToCanonicalPathsWhenKeysAbsent: a present config that
// omits both keys must degrade to the canonical layout, not to nothing.
func TestScanFallsBackToCanonicalPathsWhenKeysAbsent(t *testing.T) {
	t.Parallel()

	folder := writeConfigTree(t, map[string]string{
		configFileRel:                       "paths:\n  stories_dir: \"docs/prd/stories\"\n",
		"docs/prd/epics/epic-80-canon.yaml": configEpic80,
		"docs/scenarios.yaml":               configRegistry,
	})

	snapshot := inventory.Scan(folder)

	if got := snapshot.Documents["registry"]; got != inventory.StatusPresent {
		t.Errorf("registry status = %q, want %q", got, inventory.StatusPresent)
	}

	if snapshot.Totals.Epics != 1 {
		t.Errorf("Totals.Epics = %d, want 1 (epic under canonical default)",
			snapshot.Totals.Epics)
	}
}

// TestScanFallsBackToCanonicalPathsOnBlankValues: whitespace-only values
// count as unset — same fallback, never an empty join.
func TestScanFallsBackToCanonicalPathsOnBlankValues(t *testing.T) {
	t.Parallel()

	folder := writeConfigTree(t, map[string]string{
		configFileRel: `paths:
  epics_dir: "   "
documents:
  scenarios_yaml: ""
`,
		"docs/prd/epics/epic-80-blank.yaml": configEpic80,
		"docs/scenarios.yaml":               configRegistry,
	})

	snapshot := inventory.Scan(folder)

	if got := snapshot.Documents["registry"]; got != inventory.StatusPresent {
		t.Errorf("registry status = %q, want %q", got, inventory.StatusPresent)
	}

	if snapshot.Totals.Epics != 1 {
		t.Errorf("Totals.Epics = %d, want 1 (blank keys fall back to defaults)",
			snapshot.Totals.Epics)
	}
}
