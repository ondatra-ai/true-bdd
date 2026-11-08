package registry_test

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/registry"
)

func TestNamedFixturesFindsQuotedDirectoriesInStepOrder(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	for _, name := range []string{"offline", "second fixture", "observed", "unused"} {
		err := disk.Dir(filepath.Join(root, name), disk.Shared)
		if err != nil {
			t.Fatal(err)
		}
	}

	err := disk.Write(filepath.Join(root, "file"), []byte("not a fixture tree"), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	scenario := &registry.RegistryScenario{Statements: []registry.Statement{
		{Block: catalogue.BlockGiven, Text: `the toolchain is the "offline" fixture's`},
		{Block: catalogue.BlockWhen, Text: `use "second fixture" then "offline"`},
		{Block: catalogue.BlockThen, Text: `stdout matches ^.*"observed".*$`},
		{Block: catalogue.BlockThen, Text: `ignore "missing", "file", ".", ".." and "../offline"`},
	}}
	want := []string{"offline", "second fixture", "observed"}

	got := registry.NamedFixtures(scenario, root)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("fixture directories = %q, want %q", got, want)
	}
}
