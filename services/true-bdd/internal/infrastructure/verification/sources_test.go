package verification_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/verification"
)

func TestSourcesFollowLocalCallsAndKeepSelectedRegistration(t *testing.T) {
	root := t.TempDir()
	for path, source := range map[string]string{
		"steps/steps.go": `package steps
import "host/tests/helpers"
type State struct { Value int }
func Register(suite Registrar) { suite.Step("^selected$",selected); suite.Step("^other$",other) }
func selected(s *State,args []string) error { return helpers.Check(s.Value) }
func other(s *State,args []string) error { panic("unrelated definition") }
`,
		"helpers/check.go": `package helpers
const expected = 3
func Check(value int) error { return compare(value,expected) }
func compare(actual,expected int) error { return nil }
func Unused() { panic("unrelated helper") }
`,
	} {
		target := filepath.Join(root, path)

		err := os.MkdirAll(filepath.Dir(target), 0750)
		if err != nil {
			t.Fatal(err)
		}

		err = os.WriteFile(target, []byte(source), 0600)
		if err != nil {
			t.Fatal(err)
		}
	}

	source, err := verification.Sources(filepath.Join(root, "steps"), []string{"selected"})
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{"type State", `suite.Step("^selected$",selected)`,
		"func Check", "func compare", "const expected"} {
		if !strings.Contains(source, want) {
			t.Errorf("missing %q: %s", want, source)
		}
	}

	for _, unwanted := range []string{"unrelated definition", "unrelated helper", `"^other$"`} {
		if strings.Contains(source, unwanted) {
			t.Errorf("unreferenced source %q", unwanted)
		}
	}
}
