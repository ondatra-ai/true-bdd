package validate_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/commands"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/generators/validate"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/domain/models/provider"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/template"
)

func TestWriterTemplateGivesCodexBothFormsAndPreservesCrushBytes(t *testing.T) {
	const root = "../../../../../../"

	data := validate.FixApplierData{
		Subject: &commands.BuildCodeItem{}, ResultPath: "/repo/tmp/result.yaml",
		TestCommand: &provider.TestCommand{Command: "go test ./tests/suite", Selector: "-run", Dir: "/repo"},
	}
	loader := template.NewTemplateLoader[validate.FixApplierData](root + "templates/build-code.fix-applier.prompt.tpl")

	crush, err := loader.LoadTemplate(data)
	if err != nil {
		t.Fatal(err)
	}

	expected := legacyCrushPrompt(t, data)
	if crush != expected {
		t.Fatal("crush request changed")
	}

	data.CodexWriter = true

	codex, err := loader.LoadTemplate(data)
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		"Whole suite: `go test ./tests/suite`",
		"Selection: `go test ./tests/suite -run '<selection>'`",
		"poll that process with your native process tool until it completes"} {
		if !strings.Contains(codex, want) {
			t.Errorf("codex request missing %q", want)
		}
	}

	if strings.Contains(codex, "job_output") {
		t.Fatal("codex received crush polling instruction")
	}
}

func legacyCrushPrompt(t *testing.T, data validate.FixApplierData) string {
	t.Helper()

	const root = "../../../../../../"

	fixture := "tests/bdd-cli/fixtures/build-code-partial/input/templates/build-code.fix-applier.prompt.tpl"

	legacy, err := disk.Read(root + fixture)
	if err != nil {
		t.Fatal(err)
	}

	prefix, _, ok := strings.Cut(string(legacy), "\n## Required fixture lifecycle protocol\n")
	if !ok {
		t.Fatal("missing legacy template boundary")
	}

	path := filepath.Join(t.TempDir(), "legacy.tpl")

	err = disk.Write(path, []byte(prefix), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	expected, err := template.NewTemplateLoader[validate.FixApplierData](path).LoadTemplate(data)
	if err != nil {
		t.Fatal(err)
	}

	return expected
}
