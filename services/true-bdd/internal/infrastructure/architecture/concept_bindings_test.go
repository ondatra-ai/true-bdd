package architecture_test

import (
	"path/filepath"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
)

func TestLoadRetainsConceptBindings(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)

	body := specWith(allThreeCommands) + `  concepts:
  - concept: number
    block: given
    service: calc
    references: [{file: number}]
    does: Write the number.
  - concept: number
    block: given
    service: calc
    intent: Put it in place.
`

	path := filepath.Join(root, architectureName)

	err := disk.Write(path, []byte(body), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	loaded, err := architecture.Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(loaded.Concepts) != 2 || loaded.Concepts[0].References[0].File != numberFile ||
		loaded.Concepts[1].Intent == "" {
		t.Fatalf("lost bindings or duplicate: %+v", loaded.Concepts)
	}
}
