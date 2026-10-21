package steps_test

import (
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
	"github.com/ondatra-ai/true-bdd/tests/bdd-cli/steps"
)

// The record hint must name the test `build tests` generated, so the mirror
// CamelCases a key exactly as the engine's renderer does (ADR 0014): this is
// the table the engine's TestFuncNameCamelCasesTheKey pins.
func TestTestNameCamelCasesTheKey(t *testing.T) {
	t.Parallel()

	for key, want := range map[string]string{
		"E2E-021":                           "TestE2E021",
		"INT-900":                           "TestINT900",
		"int-900":                           "TestInt900",
		"96.2-001-foo-bar":                  "Test962001FooBar",
		"mcp-endpoint-001-initialize-ok":    "TestMcpEndpoint001InitializeOk",
		"scen-check-001-unknown-id-refused": "TestScenCheck001UnknownIdRefused",
	} {
		got := steps.TestName(bddgo.Scenario{ID: key})
		if got != want {
			t.Errorf("TestName(%q) = %q, want %q", key, got, want)
		}
	}
}
