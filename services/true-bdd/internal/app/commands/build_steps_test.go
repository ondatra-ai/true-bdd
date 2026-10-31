package commands_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/commands"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/checklist"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/config"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/docs"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/product"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/registry"
)

func TestBuildTestsRendersWithoutStepsOrRunner(t *testing.T) {
	deps := buildFixture(t)

	err := commands.RunBuildTests(context.Background(), commands.BuildTestsDeps{
		RegistryLoader: deps.RegistryLoader, DocResolver: deps.DocResolver,
	}, "docs/scenarios.yaml", "docs/architecture/architecture.yaml", true)
	if err != nil {
		t.Fatal(err)
	}

	body, err := disk.Read("tests/mcp/mcp_test.go")
	if err != nil || !strings.Contains(string(body), "scenarios.New(t,") {
		t.Fatalf("test not rendered: %s, %v", body, err)
	}
}

func buildFixture(t *testing.T) commands.BuildStepsDeps {
	t.Helper()

	root, err := filepath.Abs("../../../../..")
	if err != nil {
		t.Fatal(err)
	}

	host := t.TempDir()

	err = disk.CopyTree(host, filepath.Join(root, "tests/true-bdd/fixtures/build-tests-render-only/input"), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	for _, dir := range []string{"true-bdd", "templates"} {
		err = disk.CopyTree(filepath.Join(host, dir), filepath.Join(root, dir), disk.Shared)
		if err != nil {
			t.Fatal(err)
		}
	}

	t.Chdir(host)

	cfg, err := config.NewViperConfig()
	if err != nil {
		t.Fatal(err)
	}

	return commands.BuildStepsDeps{
		RegistryLoader: registry.NewRegistryLoader(), DocResolver: docs.NewResolver(cfg),
		ChecklistLoader: checklist.NewChecklistLoader(cfg),
	}
}

func TestBuildStepsRefusesAHostStillDeclaringTheCatalogue(t *testing.T) {
	deps := buildFixture(t)

	err := disk.Write("docs/product/product.yaml", []byte(`catalogue:
  - template: the number is {value:number}
    block: given
    means: Set the input.
`), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	err = commands.RunBuildSteps(context.Background(), deps,
		"docs/scenarios.yaml", "docs/architecture/architecture.yaml", false)
	if !errors.Is(err, product.ErrCatalogueRetired) || !strings.Contains(err.Error(), "concepts:") {
		t.Fatalf("want the retired catalogue refused naming concepts, got %v", err)
	}
}
