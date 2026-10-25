package validate_test

import (
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/commands"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/generators/validate"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/domain/models/provider"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/template"
	"strings"
	"testing"
)

func TestWriterTemplateGivesCodexBothFormsAndNativePolling(t *testing.T) {
	const root = "../../../../../../"

	data := validate.FixApplierData{
		Subject: &commands.BuildCodeItem{}, ResultPath: "/repo/tmp/result.yaml", CodexWriter: true,
		TestCommand: &provider.TestCommand{Command: "go test ./tests/suite", Selector: "-run", Dir: "/repo"},
	}
	loader := template.NewTemplateLoader[validate.FixApplierData](root + "templates/build-code.fix-applier.prompt.tpl")

	request, err := loader.LoadTemplate(data)
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		"Whole suite: `go test ./tests/suite`", "Selection: `go test ./tests/suite -run '<selection>'`",
		"poll that process with your native process tool until it completes",
	} {
		if !strings.Contains(request, want) {
			t.Errorf("writer request missing %q", want)
		}
	}
}
