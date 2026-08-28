package epic

import (
	"path/filepath"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/config"
)

// TestLoadStoryFromEpicUsesConfiguredEpicsDir pins the config key the loader
// reads (paths.epics_dir) using a non-default directory, so a silent
// fallback to docs/product/epics would fail the lookup.
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

	err := disk.Dir(filepath.Dir(path), disk.Shared)
	if err != nil {
		t.Fatalf("MkdirAll %s: %v", path, err)
	}

	err = disk.Write(path, []byte(content), disk.Shared)
	if err != nil {
		t.Fatalf("WriteFile %s: %v", path, err)
	}
}
