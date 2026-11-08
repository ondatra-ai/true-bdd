package story_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/config"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/story"
)

func TestStoryReadersRefuseUnknownStepKeys(t *testing.T) {
	for _, key := range []string{"and", "but", "whenever"} {
		t.Run(key, func(t *testing.T) {
			t.Chdir(t.TempDir())

			err := disk.Write("true-bdd/true-bdd.yaml", []byte("paths:\n  stories_dir: stories\n"), disk.Shared)
			if err != nil {
				t.Fatal(err)
			}

			body := "story:\n  id: summary\n  acceptance_criteria:\n  - id: 001-summary\n    steps:\n" +
				"    - then: [The summary is displayed]\n      " + key + ": The limit is displayed\n"

			err = disk.Write(filepath.Join("stories", "summary-story.yaml"), []byte(body), disk.Shared)
			if err != nil {
				t.Fatal(err)
			}

			cfg, err := config.NewViperConfig()
			if err != nil {
				t.Fatal(err)
			}

			_, loadErr := story.NewStoryLoader(cfg).Load("summary")

			_, _, applyErr := story.NewStoryScenarioParser(cfg).ParseStoryScenarios("summary", "tmp/registry.yaml")
			for _, readErr := range []error{loadErr, applyErr} {
				if readErr == nil || !strings.Contains(readErr.Error(), "unknown Story step key: "+key) ||
					!strings.Contains(readErr.Error(), "acceptance criterion 001-summary") {
					t.Errorf("read error = %v, want criterion and unknown key %s", readErr, key)
				}
			}
		})
	}
}
