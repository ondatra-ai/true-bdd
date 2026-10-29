package drill_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/scripts/drill"
)

const (
	interfaceArchitecture = "docs/architecture/architecture.yaml"
	cliDocument           = "api/cli.yaml"
	interfacePassage      = "The service accepts a number."
	unrelatedDocument     = "not an interface"
)

func interfaceFixture(t *testing.T, script string) (drill.Runner, drill.Record) {
	t.Helper()

	return fixtureEngineFiles(t, script, map[string]string{
		interfaceArchitecture: `architecture:
  services:
    - path: services/true-bdd
      public_interface: {kind: cli, document: api/cli.yaml}
    - path: services/other
      public_interface: {kind: http, document: z-contracts/http.yaml}
    - path: services/nested
      public_interface: {kind: cli, document: services/true-bdd/interface.yaml}
    - path: services/fixture
      public_interface: {kind: http, document: tests/true-bdd/fixtures/public/cassettes/interface.yaml}
    - path: services/web
      public_interface:
        kind: ui
        elements: [{page: /, role: button, name: Save}]
  document: api/undeclared.yaml
`,
		cliDocument:                        interfacePassage,
		"api/undeclared.yaml":              unrelatedDocument,
		"z-contracts/http.yaml":            "openapi: 3.0.0",
		"z-contracts/other.yaml":           unrelatedDocument,
		"services/true-bdd/interface.yaml": interfacePassage,
		"tests/true-bdd/fixtures/public/cassettes/interface.yaml": interfacePassage,
		"tests/true-bdd/fixtures/public/cassettes/other.yaml":     unrelatedDocument,
	})
}

func TestCutsKeepExactlyBaseInterfaceDocuments(t *testing.T) {
	runner, record := interfaceFixture(t, "#!/bin/sh\nexit 0\n")
	put(t, filepath.Join(runner.Root, interfaceArchitecture),
		"architecture:\n  services:\n    - public_interface: {kind: cli, document: api/undeclared.yaml}\n")

	for _, name := range []string{"a", "b", "c", "d"} {
		cut, err := runner.Cut(t.Context(), record.Directory, name)
		must(t, err)
		t.Cleanup(func() { _ = os.RemoveAll(cut.Directory) })

		files := strings.Split(git(t, cut.Directory, "ls-files"), "\n")

		expected := append(expectedCutFiles(name), cliDocument, "z-contracts/http.yaml",
			"services/true-bdd/interface.yaml", "tests/true-bdd/fixtures/public/cassettes/interface.yaml")
		if name == "a" || name == "b" {
			expected = append(expected, "tests/true-bdd/fixtures/public/cassettes/other.yaml")
		}

		slices.Sort(files)
		slices.Sort(expected)

		if !slices.Equal(files, expected) {
			t.Fatalf("cut %s files: %v; want %v", name, files, expected)
		}

		data, err := os.ReadFile(filepath.Join(cut.Directory, cliDocument))
		must(t, err)

		if string(data) != interfacePassage {
			t.Fatalf("cut %s changed interface document: %q", name, data)
		}
	}
}
