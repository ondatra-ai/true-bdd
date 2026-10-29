package stepcoverage_test

import (
	"path/filepath"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/stepcoverage"
)

func TestSuiteDefinesPhraseWithoutScenarioExamples(t *testing.T) {
	const statusPhrase = `\Athe status is [0-9]+\z`

	t.Parallel()

	cases := []struct {
		name, phrase, definition string
		want                     bool
	}{
		{"number", `\Athe status is (?:[0-9]+|no)\z`, `^the status is (\d+)$`, true},
		{"other operation", statusPhrase, `^the result is (\d+)$`, false},
		{"unanchored", statusPhrase, `status is`, true},
		{"literal subset", statusPhrase, `^the status is 418$`, true},
		{"anchors", statusPhrase, `^status is`, false},
		{"word boundary", `\Aword\z`, `\bword\b`, true},
		{"bad boundary", `\Asword\z`, `\bword\b`, false},
		{"fold case", `\AK\z`, `(?i)^k$`, true},
		{"newline", `\Aa\nb\z`, `^a.b$`, false},
		{"multiline", `\Aa\nb\z`, `(?m)^b$`, true},
		{"empty text", `\A.+\z`, `^$`, false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()

			source := "package steps\nfunc Register(suite Registrar) { suite.Step(`" + testCase.definition + "`, body) }\n"

			err := disk.Write(filepath.Join(dir, "steps.go"), []byte(source), disk.Shared)
			if err != nil {
				t.Fatal(err)
			}

			suite, err := stepcoverage.LoadSuite(dir)
			if err != nil {
				t.Fatal(err)
			}

			got, err := suite.Defines(testCase.phrase)
			if err != nil || got != testCase.want {
				t.Fatalf("Defines(%q) = %v, %v; want %v", testCase.phrase, got, err, testCase.want)
			}
		})
	}
}
