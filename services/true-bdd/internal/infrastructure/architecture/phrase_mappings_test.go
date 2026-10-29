package architecture_test

import (
	"path/filepath"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
)

const (
	architectureName = "architecture.yaml"
	numberFile       = "number"
	absentReference  = "absent-reference"
	orderPage        = "Order"
)

func TestLoadPhraseMappingsPreservesTemplatesAndDuplicates(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), architectureName)

	body := `architecture:
  phrases:
    - template: 'the number is "{value}"'
      service: calculator
      references: [{file: number}]
      does: Set up the declared number file with the quoted value.
    - template: 'the number is "{value}"'
      service: calculator
      intent: Arrange whatever number the user intended.
`

	err := disk.Write(path, []byte(body), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	mappings, err := architecture.LoadPhraseMappings(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(mappings) != 2 || mappings[0].Template != `the number is "{value}"` ||
		mappings[0].References[0].File != numberFile || mappings[1].Intent == "" {
		t.Fatalf("lost verbatim phrase or duplicate: %+v", mappings)
	}
}

func TestLoadedPublicInterfaceResolvesReferences(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	files := map[string]string{
		architectureName: `architecture:
  services:
  - name: calculator
    public_interface: {kind: cli, document: cli.yaml}
  - name: api
    public_interface: {kind: http, document: api.yaml}
  - name: web
    public_interface:
      kind: ui
      elements: [{page: Order, role: button, name: Save}]
`,
		"cli.yaml": `opencli: 1.0.0
info: {title: Calculator, version: "1"}
commands:
  number halve: {responses: {'0': {description: Done}}}
extensions:
  files:
    number: {path: number.txt, format: text, access: read-write}
  log_records:
    halved: {message: Halved, format: json}
  outgoing_calls:
    worker: {command: worker}
`,
		"api.yaml": `openapi: 3.0.3
info: {title: Calculator, version: "1"}
paths:
  /halve:
    post:
      operationId: halveNumber
      responses:
        '200': {description: Done}
`,
	}
	for name, body := range files {
		err := disk.Write(filepath.Join(root, name), []byte(body), disk.Shared)
		if err != nil {
			t.Fatal(err)
		}
	}

	services, err := architecture.LoadInterfaces(filepath.Join(root, architectureName), root)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		service   int
		reference architecture.InterfaceReference
		want      bool
	}{
		{0, architecture.InterfaceReference{Command: "number halve"}, true},
		{0, architecture.InterfaceReference{Command: "halve"}, false},
		{0, architecture.InterfaceReference{File: numberFile}, true},
		{0, architecture.InterfaceReference{LogRecord: "halved"}, true},
		{0, architecture.InterfaceReference{OutgoingCall: "worker"}, true},
		{0, architecture.InterfaceReference{File: absentReference}, false},
		{0, architecture.InterfaceReference{File: numberFile, Command: "number halve"}, false},
		{1, architecture.InterfaceReference{OperationID: "halveNumber"}, true},
		{1, architecture.InterfaceReference{OperationID: absentReference}, false},
		{1, architecture.InterfaceReference{Command: "halveNumber"}, false},
		{2, architecture.InterfaceReference{Element: &architecture.UIElement{
			Page: orderPage, Role: buttonRole, Name: saveButton,
		}}, true},
		{2, architecture.InterfaceReference{Element: &architecture.UIElement{
			Page: "Other", Role: buttonRole, Name: saveButton,
		}}, false},
	}
	for _, testCase := range cases {
		if got := services[testCase.service].PublicInterface.Declares(testCase.reference); got != testCase.want {
			t.Errorf("reference %s: got %v, want %v", testCase.reference, got, testCase.want)
		}
	}
}

func TestLoadRetainsPhraseMappingAndInterfaceReferences(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)

	body := specWith(allThreeCommands) + `      public_interface:
        kind: cli
        document: cli.yaml
  phrases:
    - template: request a greeting
      service: calc
      references: [{command: greet}]
      does: Call greet once.
`

	files := map[string]string{
		architectureName: body,
		"cli.yaml": "opencli: 1.0.0\ninfo: {title: Greeter, version: 1}\n" +
			"commands: {greet: {responses: {'0': {description: Done}}}}\n",
	}
	for name, content := range files {
		err := disk.Write(name, []byte(content), disk.Shared)
		if err != nil {
			t.Fatal(err)
		}
	}

	loaded, err := architecture.Load(architectureName)
	if err != nil {
		t.Fatal(err)
	}

	if len(loaded.Phrases) != 1 || !loaded.Services[0].PublicInterface.Declares(loaded.Phrases[0].References[0]) {
		t.Fatal("architecture lost its phrase or the referenced operation")
	}
}
