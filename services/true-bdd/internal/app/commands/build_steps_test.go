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

func TestBuildTestsRefusesWithoutStepsBeforeWriting(t *testing.T) {
	deps := buildFixture(t)

	err := disk.RemoveTree("tests/mcp/steps")
	if err != nil {
		t.Fatal(err)
	}

	err = commands.RunBuildTests(context.Background(), commands.BuildTestsDeps{
		RegistryLoader: deps.RegistryLoader, DocResolver: deps.DocResolver,
	}, "docs/scenarios.yaml", "docs/architecture/architecture.yaml", true)
	if err == nil || !strings.Contains(err.Error(), "build steps --fix") {
		t.Fatalf("want missing Steps package refusal, got %v", err)
	}

	_, err = disk.Read("tests/mcp/mcp_test.go")
	if err == nil {
		t.Fatal("rendered before Steps package exists")
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

func TestBuildTestsRendersMainAndChecksTheHostFolder(t *testing.T) {
	for _, kind := range []string{currentFolder, "missing", "duplicate", "unknown", "orphan fixture"} {
		t.Run(kind, func(t *testing.T) {
			deps := buildFixture(t)

			err := disk.Write("tests/mcp/steps/state.go", []byte("package steps\ntype State struct{}\n"), disk.Shared)
			if err != nil {
				t.Fatal(err)
			}

			run := func(fix bool) error {
				return commands.RunBuildTests(t.Context(), commands.BuildTestsDeps{
					RegistryLoader: deps.RegistryLoader, DocResolver: deps.DocResolver,
				}, "docs/scenarios.yaml", "docs/architecture/architecture.yaml", fix)
			}

			err = run(true)
			if err != nil {
				t.Fatal(err)
			}

			requireHostMain(t)

			err = changeHostFolder(t, kind)
			if err != nil {
				t.Fatal(err)
			}

			err = run(false)
			if kind == currentFolder && err != nil {
				t.Fatal(err)
			}

			if kind != currentFolder && err == nil {
				t.Fatalf("accepted %s", kind)
			}
		})
	}
}

const currentFolder = "current"

func TestBuildTestsPreservesATestWithAQualifiedNewCall(t *testing.T) {
	deps := buildFixture(t)

	const (
		target = "tests/mcp/widget_test.go"
		source = "package mcp_test\nimport (\"testing\"; \"host/widget\")\n" +
			"func TestWidget(t *testing.T) { widget.New(t, \"widget-name\") }\n"
	)

	err := disk.Write(target, []byte(source), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	err = disk.Write("tests/mcp/steps/state.go", []byte("package steps\ntype State struct{}\n"), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	for _, fix := range []bool{true, false} {
		err = commands.RunBuildTests(t.Context(), commands.BuildTestsDeps{RegistryLoader: deps.RegistryLoader},
			"docs/scenarios.yaml", "docs/architecture/architecture.yaml", fix)
		if err != nil {
			t.Fatalf("build tests fix=%t: %v", fix, err)
		}
	}

	got, err := disk.Read(target)
	if err != nil || string(got) != source {
		t.Fatalf("preserved test changed: %s %v", got, err)
	}
}

func changeHostFolder(t *testing.T, kind string) error {
	t.Helper()

	var err error

	switch kind {
	case "missing":
		err = disk.Remove("tests/mcp/mcp_test.go")
	case "duplicate":
		source, readErr := disk.Read("tests/mcp/mcp_test.go")
		if readErr != nil {
			t.Fatal(readErr)
		}

		err = disk.Write("tests/mcp/extra_test.go", source, disk.Shared)
	case "unknown":
		err = disk.Write("tests/mcp/extra_test.go", []byte("// Code generated by tests. DO NOT EDIT.\n"+
			"package mcp_test\nfunc TestUnknown(t *testing.T) { New(t, \"absent\") }\n"), disk.Shared)
	case "orphan fixture":
		err = disk.Dir("tests/mcp/fixtures/unused", disk.Shared)
	}

	return err
}

func requireHostMain(t *testing.T) {
	t.Helper()

	main, err := disk.Read("tests/mcp/main_test.go")
	if err != nil || !strings.Contains(string(main), "func New(t *testing.T,") {
		t.Fatalf("main: %s %v", main, err)
	}
}

func TestBuildTestsRefreshesAMarkedMainWithBrokenSyntax(t *testing.T) {
	deps := buildFixture(t)
	source := "// Code generated by `true-bdd build tests`. DO NOT EDIT.\n" +
		"package mcp_test\nfunc TestMain("

	err := disk.Write("tests/mcp/main_test.go", []byte(source), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	err = commands.RunBuildTests(t.Context(), commands.BuildTestsDeps{RegistryLoader: deps.RegistryLoader},
		"docs/scenarios.yaml", "docs/architecture/architecture.yaml", true)
	if err != nil {
		t.Fatal(err)
	}

	requireHostMain(t)
}
