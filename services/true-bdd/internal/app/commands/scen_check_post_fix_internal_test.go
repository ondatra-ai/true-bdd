package commands

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/runner"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/docs"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
)

const scenFixProduct = `concepts:
  then:
    - concept: result
      means: The calculator prints the whole number the doubling produced.
      terms: []
`

// A Scenario fix whose product working copy widens a concept through a key
// beyond its shape commits no document, not even the architecture and
// interface copies it also changed, and resets every copy (#268).
func TestScenCheckPostFixTakesNoDocumentOfAFixAConceptKeyRefused(t *testing.T) {
	added := interfaceFixDocument + "  halve:\n    responses: {'0': {description: Halved}}\n"
	architectureDocument, interfaceDocuments := writeInterfaceFixHost(t, added)
	product := template.FixDocument{Key: docs.KeyProduct, Path: "docs/product.yaml", Scratch: "tmp/product.yaml"}
	widened := scenFixProduct + "      variations:\n        - The calculator prints the halved number.\n"
	widenedArchitecture := interfaceFixArchitecture + "  terms: []\n"

	for path, content := range map[string]string{
		product.Path: scenFixProduct, product.Scratch: widened, architectureDocument.Scratch: widenedArchitecture,
	} {
		writeScenFixFile(t, path, content)
	}

	item := &template.ScenarioCheckData{
		ID: "calc-001-halve", ConceptsDeclared: true,
		TermDocuments:      []template.FixDocument{product, architectureDocument},
		InterfaceDocuments: interfaceDocuments,
	}

	_, err := scenCheckPostFix(ScenCheckDeps{}, "")(context.Background(), item, "")
	if !errors.Is(err, runner.ErrFixNotTaken) ||
		!strings.Contains(err.Error(), "then concept 'result' declares variations") {
		t.Fatalf("scenCheckPostFix() error = %v, want a fix not taken naming the key", err)
	}

	for _, document := range append([]template.FixDocument{product, architectureDocument}, interfaceDocuments...) {
		assertScenFixUnchanged(t, document)
	}
}

func writeScenFixFile(t *testing.T, path, content string) {
	t.Helper()

	err := os.WriteFile(path, []byte(content), 0o600)
	if err != nil {
		t.Fatal(err)
	}
}

// assertScenFixUnchanged holds a document to what it read before the fix,
// and its working copy to the document.
func assertScenFixUnchanged(t *testing.T, document template.FixDocument) {
	t.Helper()

	want := map[string]string{
		docs.KeyProduct: scenFixProduct, docs.KeyArchitectureYAML: interfaceFixArchitecture,
		interfaceFixPath: interfaceFixDocument,
	}[document.Key]

	for _, path := range []string{document.Path, document.Scratch} {
		content, err := os.ReadFile(path)
		if err != nil || string(content) != want {
			t.Errorf("%s = %q (%v), want %q", path, content, err, want)
		}
	}
}
