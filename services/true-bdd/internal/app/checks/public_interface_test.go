package checks_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/checks"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
)

func TestPublicInterfaceCheckNamesServiceWithoutConceptsOrStory(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "architecture.yaml")

	err := disk.Write(path, []byte("architecture:\n  services:\n    - name: greeter\n"), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	check, ok := checks.ForScenarios()["public-interface"]
	if !ok {
		t.Fatal("public-interface check is missing")
	}

	item := &template.ScenarioCheckData{ID: "example-001", Service: "greeter"}
	input := checks.Input{Docs: map[string]string{"architecture_yaml": path}}

	verdict, err := check.Run(item, input)
	if err != nil {
		t.Fatal(err)
	}

	if verdict.Pass || !strings.Contains(strings.Join(verdict.Context, "\n"), "greeter declares no Public interface") {
		t.Fatalf("want named service failure without activation exemptions: %+v", verdict)
	}
}
