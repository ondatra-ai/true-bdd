package epic

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ondatra-ai/true-bdd/src/internal/infrastructure/config"
)

// TestLoadStoryFromEpicUsesConfiguredEpicsDir pins the config key the
// loader reads: paths.epics_dir (the old top-level epics.path is gone).
// The epic lives at a NON-default directory so a silent fall back to
// docs/prd/epics would fail the lookup.
func TestLoadStoryFromEpicUsesConfiguredEpicsDir(t *testing.T) {
	root := t.TempDir()

	writeFile(t, filepath.Join(root, "true-bdd", "true-bdd.yaml"),
		"paths:\n  epics_dir: \"./planning/epics\"\n")
	writeFile(t, filepath.Join(root, "planning", "epics", "epic-07-sample.yaml"),
		`epic:
  id: 7
  name: Sample epic
stories:
  - id: "7.1"
    title: First story
`)

	t.Chdir(root)

	cfg, err := config.NewViperConfig()
	if err != nil {
		t.Fatalf("NewViperConfig: %v", err)
	}

	loaded, err := NewEpicLoader(cfg).LoadStoryFromEpic("7.1")
	if err != nil {
		t.Fatalf("LoadStoryFromEpic: unexpected error %v", err)
	}

	if loaded.Title != "First story" {
		t.Errorf("loaded story title = %q, want %q", loaded.Title, "First story")
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()

	err := os.MkdirAll(filepath.Dir(path), 0o755)
	if err != nil {
		t.Fatalf("MkdirAll %s: %v", path, err)
	}

	err = os.WriteFile(path, []byte(content), 0o644)
	if err != nil {
		t.Fatalf("WriteFile %s: %v", path, err)
	}
}
