package steps

import (
	"path/filepath"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

const filePatternMatches = "matches"
const claudeSettingsFile = "settings.json"

func TestFilePatternsMatchSerializedPaths(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		file    string
		content string
	}{
		{claudeSettingsFile, `"/tmp/a\u0026b\u003cc\u003e/quote\"back\\slash.+/tmp"`},
		{"config.toml", `"/tmp/a&b<c>/quote\"back\\slash.+/tmp"`},
		{"prompt.txt", "/tmp/a&b<c>/quote\"back\\slash.+/tmp"},
	} {
		t.Run(test.file, func(t *testing.T) {
			t.Parallel()

			literal, err := serializedPatternPath(test.file, "/tmp/a&b<c>/quote\"back\\slash.+")
			if err != nil {
				t.Fatal(err)
			}

			prefix, suffix := "", ""
			if filepath.Ext(test.file) != ".txt" {
				prefix, suffix = `"`, `"`
			}

			if got := prefix + literal + "/tmp" + suffix; got != test.content {
				t.Fatalf("serialized path = %q, want %q", got, test.content)
			}
		})
	}
}

func TestEveryFilePatternUsesEachMatchedFilesFormat(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), `a&b<c>".+`)

	err := disk.Dir(root, disk.Private)
	if err != nil {
		t.Fatal(err)
	}

	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}

	for _, file := range []string{"config.toml", claudeSettingsFile, "prompt.txt"} {
		literal, encodeErr := serializedPatternPath(file, resolved)
		if encodeErr != nil {
			t.Fatal(encodeErr)
		}

		err := disk.Write(filepath.Join(root, file), []byte(literal+"/tmp"), disk.Private)
		if err != nil {
			t.Fatal(err)
		}
	}

	err = assertEveryFileMatches(runAt(resolved), []string{"*", filePatternMatches, `^{{CWD}}/tmp$`})
	if err != nil {
		t.Fatal(err)
	}

	err = assertEveryFileMatches(runAt(resolved), []string{"*", "does not match", `^{{CWD}}/other$`})
	if err != nil {
		t.Fatal(err)
	}
}
