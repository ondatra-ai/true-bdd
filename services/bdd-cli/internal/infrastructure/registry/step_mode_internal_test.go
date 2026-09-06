package registry

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

// writeSteps lays down a registry whose one scenario carries the given
// `merged_steps:` blocks verbatim, and returns its path.
func writeSteps(t *testing.T, blocks string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "scenarios.yaml")

	doc := `metadata: {title: t, version: "1", description: d}
scenarios:
  INT-900:
    description: d
    service: s
    path: tests/x/x_test.go
    user_stories: []
    merged_steps:
` + blocks

	err := disk.Write(path, []byte(doc), disk.Shared)
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	return path
}

// A model-run prefix sets the mode, and the TEXT keeps it verbatim: the
// generated test quotes Text and bddgo strips the prefix there, so
// stripping it here would rewrite every generated file.
func TestLoadClassifiesModelRunSteps(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		blocks string
		mode   StepMode
		text   string
	}{
		{"judge in then", "      then:\n        - 'judge: it reads the same'\n",
			ModeRule, "judge: it reads the same"},
		{"llm in when", "      when:\n        - 'llm: close the dialog'\n",
			ModeAct, "llm: close the dialog"},
		{"llm in given", "      given:\n        - 'llm: close the dialog'\n",
			ModeAct, "llm: close the dialog"},
		{"plain step", "      then:\n        - it exits 0\n",
			ModeDeterministic, "it exits 0"},
		{"and-modified judge at index 0", "      then:\n        - and: 'judge: it reads the same'\n",
			ModeRule, "judge: it reads the same"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			scenarios, err := NewRegistryLoader().Load(writeSteps(t, testCase.blocks))
			if err != nil {
				t.Fatalf("Load: %v", err)
			}

			got := scenarios[0].Statements[0]
			if got.Mode != testCase.mode {
				t.Errorf("mode = %d, want %d", got.Mode, testCase.mode)
			}

			if got.Text != testCase.text {
				t.Errorf("text = %q, want %q", got.Text, testCase.text)
			}
		})
	}
}

// A prefix in a block that cannot run it is refused, not reinterpreted.
func TestLoadRefusesPrefixInWrongBlock(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"judge in given":              "      given:\n        - 'judge: it reads the same'\n",
		"judge in when":               "      when:\n        - 'judge: it reads the same'\n",
		"llm in then":                 "      then:\n        - 'llm: close the dialog'\n",
		"and-modified llm at index 0": "      then:\n        - and: 'llm: close the dialog'\n",
	}

	for name, blocks := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := NewRegistryLoader().Load(writeSteps(t, blocks))
			if !errors.Is(err, ErrPrefixWrongBlock) {
				t.Fatalf("want ErrPrefixWrongBlock, got %v", err)
			}
		})
	}
}

// A prefix with nothing after it is the whole instruction a model would
// receive, so it is refused rather than passed on empty.
func TestLoadRefusesEmptyClause(t *testing.T) {
	t.Parallel()

	_, err := NewRegistryLoader().Load(writeSteps(t,
		"      then:\n        - 'judge:   '\n"))
	if !errors.Is(err, ErrEmptyClause) {
		t.Fatalf("want ErrEmptyClause, got %v", err)
	}
}

// Two malformed scenarios must name the SAME one on every run: ids are
// sorted before the loop, so a YAML map's unordered iteration cannot
// decide which refusal a person sees.
func TestLoadNamesTheSameMalformedScenarioEveryRun(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "scenarios.yaml")
	doc := `metadata: {title: t, version: "1", description: d}
scenarios:
  INT-901:
    description: d
    service: s
    path: tests/x/x_test.go
    user_stories: []
    merged_steps:
      given:
        - 'judge: second'
  INT-900:
    description: d
    service: s
    path: tests/x/x_test.go
    user_stories: []
    merged_steps:
      given:
        - 'judge: first'
`

	err := disk.Write(path, []byte(doc), disk.Shared)
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	for range 20 {
		_, loadErr := NewRegistryLoader().Load(path)
		if loadErr == nil {
			t.Fatal("want a refusal, got none")
		}

		if !strings.Contains(loadErr.Error(), "INT-900") {
			t.Fatalf("want the lowest id named, got %v", loadErr)
		}
	}
}
