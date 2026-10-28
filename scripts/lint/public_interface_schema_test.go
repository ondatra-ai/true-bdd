package lint_test

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/cli/yamale"
	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

func TestArchitectureSchemaPublicInterface(t *testing.T) {
	t.Parallel()

	err := yamale.Available()
	if err != nil {
		t.Skip(err)
	}

	cases := []struct {
		name, declaration string
		valid             bool
	}{
		{"optional", "", true},
		{"CLI", "public_interface: {kind: cli, document: docs/cli.yaml}", true},
		{"HTTP", "public_interface: {kind: http, document: docs/api.json}", true},
		{"UI", "public_interface: {kind: ui, elements: [{page: Order, role: button, name: Save}]}", true},
		{"future kind loads for the check", "public_interface: {kind: grpc}", true},
		{"library loads for the check", "public_interface: {kind: library}", true},
		{"document must be a path", "public_interface: {kind: cli, document: []}", false},
		{"UI elements must be a list", "public_interface: {kind: ui, elements: wrong}", false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			document := `architecture:
  testing:
    framework: go-test
    commands: {record: record, replay: replay, live: live}
  services:
    - name: greeter
      path: services/greeter
      language: go
      ` + testCase.declaration + "\n"
			path := filepath.Join(t.TempDir(), "architecture.yaml")

			err := disk.Write(path, []byte(document), disk.Shared)
			if err != nil {
				t.Fatal(err)
			}

			var report bytes.Buffer

			result, err := yamale.Validate(&report, "../../true-bdd/architecture_yaml-schema.yaml", path)
			if err != nil {
				t.Fatal(err)
			}

			if (result.Code == 0) != testCase.valid {
				t.Fatalf("unexpected schema verdict: %s", report.String())
			}
		})
	}
}
