package commands_test

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/commands"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/checklist"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/config"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/docs"
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

func TestBuildStepsListsUnusedCataloguePhrase(t *testing.T) {
	deps := buildFixture(t)

	product := `catalogue:
  - template: the MCP server is running on its configured port
    block: given
    means: Start the server.
  - template: the Claude User posts a valid JSON-RPC initialize request to /mcp
    block: when
    means: Initialize.
  - template: the server returns HTTP {status:number}
    block: then
    means: Compare status.
  - template: the unused operation runs
    block: when
    means: Run the unused operation.
  - template: the foreign operation runs
    block: when
    means: Belongs to another service.
`

	err := disk.Write("docs/product/product.yaml", []byte(product), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	err = disk.Write("docs/architecture/architecture.yaml", []byte(`architecture:
  services:
    - {name: mcp-service, path: services/mcp-service}
  testing:
    framework: go-test
    commands:
      record: go test -json ./tests/...
      replay: go test -json ./tests/...
      live: go test -json ./tests/...
  phrases:
    - {template: the unused operation runs, service: mcp-service}
`), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	var output strings.Builder

	previous := slog.Default()

	slog.SetDefault(slog.New(slog.NewTextHandler(&output, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	err = commands.RunBuildSteps(context.Background(), deps,
		"docs/scenarios.yaml", "docs/architecture/architecture.yaml", false)
	if !errors.Is(err, commands.ErrMissingStepDefinitions) {
		t.Fatalf("want missing definitions, got %v", err)
	}

	for _, phrase := range []string{"the server returns HTTP {status:number}", "the unused operation runs"} {
		if !strings.Contains(output.String(), phrase) {
			t.Errorf("missing phrase %q: %s", phrase, output.String())
		}
	}

	if strings.Contains(output.String(), "the foreign operation runs") {
		t.Fatal("unmapped unused phrase became an Item")
	}
}

func TestBuildStepsListsPhraseOncePerSuite(t *testing.T) {
	deps := buildFixture(t)

	files := map[string]string{
		"docs/product/product.yaml": `catalogue:
  - template: the number is {value:number}
    block: given
    means: Set the input.
`,
		"docs/architecture/architecture.yaml": `architecture:
  testing:
    framework: go-test
    commands:
      record: go test -json ./tests/...
      replay: go test -json ./tests/...
      live: go test -json ./tests/...
  services:
    - {name: first, path: services/first}
    - {name: second, path: services/second}
`,
		"docs/scenarios.yaml": `scenarios:
  first-001:
    description: First input
    service: first
    path: tests/first/first_test.go
    merged_steps: {given: [the number is 1]}
  first-002:
    description: Second input
    service: first
    path: tests/first/first_test.go
    merged_steps: {given: [the number is 2]}
  second-001:
    description: Other suite
    service: second
    path: tests/second/second_test.go
    merged_steps: {given: [the number is 3]}
`,
	}
	for path, body := range files {
		err := disk.Write(path, []byte(body), disk.Shared)
		if err != nil {
			t.Fatal(err)
		}
	}

	var output strings.Builder

	previous := slog.Default()

	slog.SetDefault(slog.New(slog.NewTextHandler(&output, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	err := commands.RunBuildSteps(context.Background(), deps,
		"docs/scenarios.yaml", "docs/architecture/architecture.yaml", false)
	if !errors.Is(err, commands.ErrMissingStepDefinitions) {
		t.Fatal(err)
	}

	if strings.Count(output.String(), `msg="Missing step definition"`) != 2 {
		t.Fatalf("want one missing Item per suite, got %s", output.String())
	}
}
