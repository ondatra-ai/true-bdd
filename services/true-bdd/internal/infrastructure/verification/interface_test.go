package verification_test

import (
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/verification"
)

func TestInterfaceEntriesIncludeOnlyBindingReferencesAndReferencedSchemas(t *testing.T) {
	document := `commands:
  halve: {description: Halve a number., responses: {0: {description: integer stdout}}}
  unrelated: {description: Unrelated operation.}
extensions:
  log_records:
    observed: {message: Observed, format: Complete retained meaning.}
`

	selected, err := verification.InterfaceEntries(document,
		[]architecture.InterfaceReference{{Command: "halve"}, {LogRecord: "observed"}})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(selected, "Complete retained meaning.") || strings.Contains(selected, "Unrelated operation.") {
		t.Fatal(selected)
	}
}
