package scenariogen_test

import (
	"path/filepath"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/scenariogen"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/registry"
)

func TestScenarioTestIdentityUsesTheOwningModule(t *testing.T) {
	root := t.TempDir()
	for name, content := range map[string]string{
		"go.mod":              "module host/root\n",
		"tests/nested/go.mod": "module host/nested\n",
	} {
		err := disk.Write(filepath.Join(root, name), []byte(content), disk.Private)
		if err != nil {
			t.Fatal(err)
		}
	}

	for _, example := range []struct{ path, want string }{
		{"tests/calc/calc_test.go", "host/root/tests/calc::TestBuildCodeCommands003UnclosedQuoteRefused"},
		{"tests/nested/calc/calc_test.go", "host/nested/calc::TestBuildCodeCommands003UnclosedQuoteRefused"},
	} {
		scenario := &registry.RegistryScenario{ID: "build-code-commands-003-unclosed-quote-refused", Path: example.path}

		got, err := scenariogen.TestIdentity(root, scenario)
		if err != nil {
			t.Fatal(err)
		}

		if got != example.want {
			t.Fatalf("identity: got %q, want %q", got, example.want)
		}
	}
}
