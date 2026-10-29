package commands

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
)

const interfaceFixArchitecture = `architecture:
  services:
  - name: calc
    path: services/calc
    language: go
    public_interface: {kind: cli, document: docs/calc-cli.yaml}
`

const interfaceFixPath = "docs/calc-cli.yaml"

const interfaceFixDocument = `opencli: 1.0.0
info: {title: Calc, version: 1.0.0}
commands:
  double:
    responses: {'0': {description: Doubled}}
`

// writeInterfaceFixHost lays out a host with an architecture and its CLI
// document, and the working copies a fix edits, under a fresh cwd.
func writeInterfaceFixHost(t *testing.T, editedDocument string) (template.FixDocument, []template.FixDocument) {
	t.Helper()
	t.Chdir(t.TempDir())

	for path, content := range map[string]string{
		"docs/architecture/architecture.yaml": interfaceFixArchitecture,
		interfaceFixPath:                      interfaceFixDocument,
		"tmp/architecture_yaml.yaml":          interfaceFixArchitecture,
		"tmp/interface-1-calc-cli.yaml":       editedDocument,
	} {
		err := os.MkdirAll(filepath.Dir(path), 0o750)
		if err != nil {
			t.Fatal(err)
		}

		err = os.WriteFile(path, []byte(content), 0o600)
		if err != nil {
			t.Fatal(err)
		}
	}

	return template.FixDocument{
		Key: "architecture_yaml", Path: "docs/architecture/architecture.yaml", Scratch: "tmp/architecture_yaml.yaml",
	}, []template.FixDocument{
		{Key: interfaceFixPath, Path: interfaceFixPath, Scratch: "tmp/interface-1-calc-cli.yaml"},
	}
}

func TestCommitInterfaceFixWritesTheAddedOperation(t *testing.T) {
	added := interfaceFixDocument + "  halve:\n    responses: {'0': {description: Halved}}\n"
	architectureDocument, interfaceDocuments := writeInterfaceFixHost(t, added)

	err := commitInterfaceFix(architectureDocument, interfaceDocuments)
	if err != nil {
		t.Fatal(err)
	}

	committed, err := os.ReadFile(interfaceFixPath)
	if err != nil || string(committed) != added {
		t.Fatalf("docs/calc-cli.yaml = %q, %v; want the working copy", committed, err)
	}
}

func TestCommitInterfaceFixRefusesADocumentItsKindCannotRead(t *testing.T) {
	broken := interfaceFixDocument + "  halve: {}\n"
	architectureDocument, interfaceDocuments := writeInterfaceFixHost(t, broken)

	err := commitInterfaceFix(architectureDocument, interfaceDocuments)
	if !errors.Is(err, ErrTermDocumentUnparseable) {
		t.Fatalf("error = %v, want ErrTermDocumentUnparseable", err)
	}

	kept, err := os.ReadFile(interfaceFixPath)
	if err != nil || string(kept) != interfaceFixDocument {
		t.Fatalf("docs/calc-cli.yaml = %q, %v; want it unchanged", kept, err)
	}
}
