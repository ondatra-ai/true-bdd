package architecture_test

import (
	"errors"
	"path/filepath"
	"strings"
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

func TestRetiredPhraseMappingsAreRefusedNamingConceptBindings(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)

	body := specWith(allThreeCommands) + `  phrases:
    - template: request a greeting
      service: calc
      references: [{command: greet}]
      does: Call greet once.
`

	err := disk.Write(architectureName, []byte(body), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	for name, load := range map[string]func() error{
		"startup": func() error { return architecture.ValidateForm(architectureName) },
		"load": func() error {
			_, err := architecture.Load(architectureName)

			return err
		},
	} {
		err := load()
		if !errors.Is(err, architecture.ErrPhrasesRetired) || !strings.Contains(err.Error(), "architecture.concepts") {
			t.Errorf("%s: want the retired phrases naming architecture.concepts, got %v", name, err)
		}
	}
}

func TestValidateFormLeavesAnUnreadableArchitectureToTheCommand(t *testing.T) {
	t.Parallel()

	path := writeSpec(t, "architecture: [\n")

	err := architecture.ValidateForm(path)
	if err != nil {
		t.Fatalf("an unreadable architecture is the command's finding, got %v", err)
	}

	err = architecture.ValidateForm(filepath.Join(t.TempDir(), "absent.yaml"))
	if err != nil {
		t.Fatalf("an absent architecture is the command's finding, got %v", err)
	}
}
