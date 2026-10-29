package architecture_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/architecture"
)

func TestDeclaredInterfaceDocumentsListsEachCLIAndHTTPDocumentOnce(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "architecture.yaml")

	err := os.WriteFile(path, []byte(`architecture:
  services:
  - {name: calc, path: services/calc, language: go, public_interface: {kind: cli, document: docs/calc-cli.yaml}}
  - {name: twin, path: services/twin, language: go, public_interface: {kind: cli, document: docs/calc-cli.yaml}}
  - {name: api, path: services/api, language: go, public_interface: {kind: http, document: docs/api.yaml}}
  - name: web
    path: services/web
    language: go
    public_interface: {kind: ui, elements: [{page: home, role: button, name: Save}]}
  - {name: bare, path: services/bare, language: go}
`), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	documents, err := architecture.DeclaredInterfaceDocuments(path)
	if err != nil {
		t.Fatal(err)
	}

	want := []architecture.InterfaceDocument{
		{Service: "calc", Kind: "cli", Path: "docs/calc-cli.yaml"},
		{Service: "api", Kind: "http", Path: "docs/api.yaml"},
	}
	if len(documents) != len(want) || documents[0] != want[0] || documents[1] != want[1] {
		t.Fatalf("documents = %+v, want %+v", documents, want)
	}
}

func TestCheckInterfaceDocumentHoldsACLIDocumentToItsShape(t *testing.T) {
	t.Parallel()

	valid := []byte("opencli: 1.0.0\ninfo: {title: Calc, version: 1.0.0}\n" +
		"commands:\n  halve:\n    responses: {'0': {description: Halved}}\n")

	err := architecture.CheckInterfaceDocument("cli", valid, "docs/calc-cli.yaml", ".")
	if err != nil {
		t.Fatalf("valid document: %v", err)
	}

	missingResponses := []byte("opencli: 1.0.0\ninfo: {title: Calc, version: 1.0.0}\ncommands:\n  halve: {}\n")

	err = architecture.CheckInterfaceDocument("cli", missingResponses, "docs/calc-cli.yaml", ".")
	if !errors.Is(err, architecture.ErrInterfaceDocument) {
		t.Fatalf("command without responses: error = %v, want ErrInterfaceDocument", err)
	}
}
