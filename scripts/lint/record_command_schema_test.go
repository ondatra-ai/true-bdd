package lint_test

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/cli/yamale"
	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

func TestArchitectureSchemaRecordCommand(t *testing.T) {
	t.Parallel()

	err := yamale.Available()
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		declaration string
		valid       bool
	}{
		{`record`, true}, {`{command: record}`, true}, {`{command: record, exclude: []}`, true},
		{`{command: record, exclude: ['cache/**', '**/private/**']}`, true},
		{`[]`, false}, {`null`, false}, {`42`, false}, {`true`, false}, {`{}`, false},
		{`{exclude: []}`, false}, {`{command: []}`, false}, {`{command: 42}`, false}, {`{command: null}`, false},
		{`{command: record, extra: value}`, false}, {`{command: record, exclude: private}`, false},
		{`{command: record, exclude: null}`, false}, {`{command: record, exclude: [42]}`, false},
		{`{command: record, exclude: [null]}`, false},
	}
	for _, testCase := range cases {
		t.Run(testCase.declaration, func(t *testing.T) {
			t.Parallel()

			document := `architecture:
  testing:
    framework: go-test
    commands:
      record: ` + testCase.declaration + `
      replay: replay
      live: live
  services:
    - name: probe
      path: services/probe
      language: go
`

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
