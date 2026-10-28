package architecture_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
)

func TestLoadRefusesMissingPublicInterfaceDocument(t *testing.T) {
	t.Parallel()

	body := specWith(allThreeCommands) + `      public_interface:
        kind: cli
        document: docs/missing-cli.yaml
`

	_, err := architecture.Load(writeSpec(t, body))
	if err == nil || !strings.Contains(err.Error(), "docs/missing-cli.yaml") {
		t.Fatalf("missing interface document must refuse loading: %v", err)
	}
}

func TestLoadPublicInterfaceKinds(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "architecture.yaml")

	body := `architecture:
  services:
    - name: greeter
      public_interface:
        kind: ui
        elements:
          - page: Order
            role: button
            name: Save
`

	err := disk.Write(path, []byte(body), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	services, err := architecture.LoadInterfaces(path, root)
	if err != nil {
		t.Fatal(err)
	}

	if len(services) != 1 || services[0].PublicInterface == nil {
		t.Fatalf("lost declaration: %+v", services)
	}

	if finding := services[0].PublicInterface.Finding(); finding != "" {
		t.Fatal(finding)
	}
}

const (
	cliKind    = "cli"
	httpKind   = "http"
	saveButton = "Save"
	buttonRole = "button"
)

func TestPublicInterfaceDeclarationFindings(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		declaration *architecture.PublicInterface
		want        string
	}{
		{"undeclared", nil, "declares no Public interface"},
		{"library", &architecture.PublicInterface{Kind: "library"}, "library is excluded"},
		{"future", &architecture.PublicInterface{Kind: "grpc"}, `"grpc" is not supported yet`},
		{"empty CLI", &architecture.PublicInterface{Kind: cliKind}, "requires a document"},
		{"empty UI", &architecture.PublicInterface{Kind: "ui"}, "requires inline elements"},
		{"unscoped UI", &architecture.PublicInterface{Kind: "ui", Elements: []architecture.UIElement{
			{Role: buttonRole, Name: saveButton},
		}}, "page or region"},
		{"repeated UI", &architecture.PublicInterface{Kind: "ui", Elements: []architecture.UIElement{
			{Page: "Order", Role: buttonRole, Name: saveButton}, {Page: "Order", Role: buttonRole, Name: saveButton},
		}}, "repeat a role"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if finding := testCase.declaration.Finding(); !strings.Contains(finding, testCase.want) {
				t.Fatalf("finding %q does not contain %q", finding, testCase.want)
			}
		})
	}
}

func TestInterfaceDocumentsMustParseInTheirKind(t *testing.T) {
	t.Parallel()

	cases := []struct{ name, kind, content, want string }{
		{"empty CLI", cliKind, "{}", "OpenCLI draft requires"},
		{"invalid CLI YAML", cliKind, "commands: [", "yaml:"},
		{"malformed trailing document", cliKind,
			"opencli: 0.1\ninfo: {title: CLI, version: 1}\n" +
				"commands: {greet: {exit_codes: {0: OK}}}\n---\n[", "yaml:"},
		{"wrong HTTP version", httpKind,
			`{"openapi":"2.0","info":{"title":"API","version":"1"},"paths":{}}`, "expected OpenAPI 3"},
		{"missing HTTP info", httpKind, `{"openapi":"3.0.3","paths":{}}`, "invalid info"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			path := filepath.Join(root, "architecture.yaml")
			body := "architecture:\n  services:\n    - name: greeter\n      public_interface:\n        kind: " +
				testCase.kind + "\n        document: interface.yaml\n"

			err := disk.Write(path, []byte(body), disk.Shared)
			if err != nil {
				t.Fatal(err)
			}

			err = disk.Write(filepath.Join(root, "interface.yaml"), []byte(testCase.content), disk.Shared)
			if err != nil {
				t.Fatal(err)
			}

			_, err = architecture.LoadInterfaces(path, root)
			if err == nil || !strings.Contains(err.Error(), "interface.yaml") || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("expected named document and kind parse error %q: %v", testCase.want, err)
			}
		})
	}
}

func TestHTTPInterfaceLoadsReferencedLocalDocuments(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	files := map[string]string{
		"architecture.yaml": `architecture:
  services:
    - name: greeter
      public_interface:
        kind: http
        document: api.yaml
`,
		"api.yaml": `openapi: 3.0.3
info: {title: Greeter, version: "1"}
paths:
  /greeting:
    get:
      responses:
        "200": {$ref: "responses.yaml#/greeting"}
`,
		"responses.yaml": "greeting: {description: Greeting returned}\n",
	}
	for name, body := range files {
		err := disk.Write(filepath.Join(root, name), []byte(body), disk.Shared)
		if err != nil {
			t.Fatal(err)
		}
	}

	_, err := architecture.LoadInterfaces(filepath.Join(root, "architecture.yaml"), root)
	if err != nil {
		t.Fatalf("valid local OpenAPI reference must load: %v", err)
	}
}

func TestUnsupportedInterfaceKindIsLeftForTheCheck(t *testing.T) {
	t.Parallel()
	path := writeSpec(t, specWith(allThreeCommands)+`      public_interface:
        kind: grpc
        document: docs/not-supported.proto
`)

	services, err := architecture.LoadInterfaces(path, ".")
	if err != nil {
		t.Fatalf("an unsupported kind must reach the named check: %v", err)
	}

	finding := services[0].PublicInterface.Finding()
	if !strings.Contains(finding, `"grpc" is not supported yet`) {
		t.Fatal(finding)
	}
}

func TestCLIInterfaceUsesOpenCLIParametersAndResponses(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "architecture.yaml")

	err := disk.Write(path, []byte(specWith(allThreeCommands)+`      public_interface:
        kind: cli
        document: cli.yaml
`), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	document := `opencli: 1.0.0
info: {title: Greeter, version: 1.0.0}
commands:
  greet:
    parameters:
      - {name: person, in: argument, required: true, schema: {type: string}}
      - {name: loud, in: flag, schema: {type: boolean}}
    responses:
      '0': {description: Greeting printed}
      '1': {description: Invalid input}
`

	err = disk.Write(filepath.Join(root, "cli.yaml"), []byte(document), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	_, err = architecture.LoadInterfaces(path, root)
	if err != nil {
		t.Fatalf("OpenCLI parameters and responses must load: %v", err)
	}
}
