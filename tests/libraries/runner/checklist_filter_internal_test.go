package runner

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// writeTestFixture materializes a loadable fixture directory: a
// fixture.yaml with the given extra manifest lines and an input tree
// populated by the caller's paths (path -> content).
func writeTestFixture(t *testing.T, manifestExtra string, inputFiles map[string]string) string {
	t.Helper()

	dir := t.TempDir()

	manifest := "input: input\n" + manifestExtra +
		"expected:\n  judge: |\n    stub rubric\n"

	err := os.WriteFile(filepath.Join(dir, "fixture.yaml"), []byte(manifest), 0o644)
	if err != nil {
		t.Fatalf("write fixture.yaml: %v", err)
	}

	err = os.MkdirAll(filepath.Join(dir, "input"), 0o755)
	if err != nil {
		t.Fatalf("mkdir input: %v", err)
	}

	for rel, content := range inputFiles {
		path := filepath.Join(dir, "input", rel)

		err = os.MkdirAll(filepath.Dir(path), 0o755)
		if err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}

		err = os.WriteFile(path, []byte(content), 0o644)
		if err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	return dir
}

// loadedChecklist is the minimal shape needed to inspect a generated
// checklist file.
type loadedChecklist struct {
	Config   map[string]any `yaml:"config"`
	Sections []struct {
		ID                string `yaml:"id"`
		ValidationPrompts []struct {
			Q string `yaml:"Q"` //nolint:tagliatelle // checklist schema key
			F string `yaml:"F"` //nolint:tagliatelle // checklist schema key
		} `yaml:"validation_prompts"`
	} `yaml:"sections"`
}

// parseGeneratedChecklist reads a checklist out of a prepared tmpdir.
func parseGeneratedChecklist(t *testing.T, tmpDir, stem string) loadedChecklist {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(tmpDir, "true-bdd", "checklists", stem+".yaml"))
	if err != nil {
		t.Fatalf("read generated checklist: %v", err)
	}

	var doc loadedChecklist

	err = yaml.Unmarshal(data, &doc)
	if err != nil {
		t.Fatalf("parse generated checklist: %v", err)
	}

	return doc
}

// TestPrepareRunDirGeneratesFilteredChecklist drives the real wiring
// (LoadFixture -> prepareRunDir) and asserts the tmpdir's us-refine
// checklist ends up with exactly the selected prompt while other
// checklists stay complete.
func TestPrepareRunDirGeneratesFilteredChecklist(t *testing.T) {
	t.Parallel()

	dir := writeTestFixture(t,
		"checklist_prompts:\n  us-refine:\n    - \"whether its steps field follows the Given/When/Then format\"\n",
		nil)

	fixture, err := LoadFixture(dir)
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}

	tmpDir, err := prepareRunDir(fixture, t.TempDir())
	if err != nil {
		t.Fatalf("prepare run dir: %v", err)
	}

	refined := parseGeneratedChecklist(t, tmpDir, "us-refine")
	if len(refined.Sections) != 1 || len(refined.Sections[0].ValidationPrompts) != 1 {
		t.Fatalf("generated us-refine: want 1 section with 1 prompt, got %+v", refined.Sections)
	}

	prompt := refined.Sections[0].ValidationPrompts[0]
	if !strings.Contains(prompt.Q, "steps field follows the") {
		t.Errorf("generated prompt is not the steps-format one: %q", prompt.Q)
	}

	if strings.TrimSpace(prompt.F) == "" {
		t.Error("generated prompt lost its F template")
	}

	created := parseGeneratedChecklist(t, tmpDir, "us-create")
	if len(created.Sections) < 2 {
		t.Errorf("us-create.yaml must stay unfiltered, got %d sections", len(created.Sections))
	}
}

// TestPrepareRunDirWithoutFilterLeavesChecklistIdentical pins the
// no-declaration path: the tmpdir checklist is byte-identical to the
// shipped one.
func TestPrepareRunDirWithoutFilterLeavesChecklistIdentical(t *testing.T) {
	t.Parallel()

	dir := writeTestFixture(t, "", nil)

	fixture, err := LoadFixture(dir)
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}

	tmpDir, err := prepareRunDir(fixture, t.TempDir())
	if err != nil {
		t.Fatalf("prepare run dir: %v", err)
	}

	repoRoot, err := FindRepoRoot()
	if err != nil {
		t.Fatalf("find repo root: %v", err)
	}

	shipped, err := os.ReadFile(filepath.Join(repoRoot, "true-bdd", "checklists", "us-refine.yaml"))
	if err != nil {
		t.Fatalf("read shipped checklist: %v", err)
	}

	generated, err := os.ReadFile(filepath.Join(tmpDir, "true-bdd", "checklists", "us-refine.yaml"))
	if err != nil {
		t.Fatalf("read tmpdir checklist: %v", err)
	}

	if string(shipped) != string(generated) {
		t.Error("without checklist_prompts the tmpdir checklist must be byte-identical to shipped")
	}
}

// TestPrepareRunDirPreservesConfigBlock filters us-apply down to its
// first prompt and requires the shipped config block (the engine's
// max_apply_attempts source) to survive.
func TestPrepareRunDirPreservesConfigBlock(t *testing.T) {
	t.Parallel()

	dir := writeTestFixture(t, "", nil)

	fixture, err := LoadFixture(dir)
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}

	fixture.Cmd = "us apply 9.9 --fix"
	fixture.ChecklistPrompts = map[string][]string{
		"us-apply": {"already appear as a scenario in the registry"},
	}

	tmpDir, err := prepareRunDir(fixture, t.TempDir())
	if err != nil {
		t.Fatalf("prepare run dir: %v", err)
	}

	applied := parseGeneratedChecklist(t, tmpDir, "us-apply")
	if len(applied.Sections) != 1 || len(applied.Sections[0].ValidationPrompts) != 1 {
		t.Fatalf("generated us-apply: want 1 section with 1 prompt, got %+v", applied.Sections)
	}

	if applied.Config["max_apply_attempts"] != 5 {
		t.Errorf("config block lost: %v", applied.Config)
	}
}

// TestLoadFixtureRejectsBadFilterDeclarations covers the load-time
// invariants: stem/cmd mismatch, override conflict, empty selection.
func TestLoadFixtureRejectsBadFilterDeclarations(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		manifestExtra string
		inputFiles    map[string]string
		wantErr       error
	}{
		{
			name:          "stem does not match cmd",
			manifestExtra: "checklist_prompts:\n  us-create:\n    - \"a snippet\"\n",
			wantErr:       ErrFilterStemMismatch,
		},
		{
			name:          "declaration plus input override",
			manifestExtra: "checklist_prompts:\n  us-refine:\n    - \"a snippet\"\n",
			inputFiles: map[string]string{
				"true-bdd/checklists/us-refine.yaml": "version: \"1.0\"\nsections: []\n",
			},
			wantErr: ErrFilterConflict,
		},
		{
			name:          "empty snippet list",
			manifestExtra: "checklist_prompts:\n  us-refine: []\n",
			wantErr:       ErrSnippetEmpty,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			dir := writeTestFixture(t, testCase.manifestExtra, testCase.inputFiles)

			fixture, err := LoadFixture(dir)
			if err != nil {
				t.Fatalf("LoadFixture: %v", err)
			}

			// The filter rules depend on WHICH checklist is invoked, so
			// they cannot fire until the scenario names the command.
			err = fixture.UseCommand("us refine 9.9")
			if !errors.Is(err, testCase.wantErr) {
				t.Errorf("want %v, got %v", testCase.wantErr, err)
			}
		})
	}
}

// TestPrepareRunDirRejectsBadSnippets covers the prep-time snippet
// resolution failures: unmatched, ambiguous, and duplicate snippets.
func TestPrepareRunDirRejectsBadSnippets(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		snippets []string
		wantErr  error
	}{
		{
			name:     "unmatched snippet",
			snippets: []string{"this text appears in no shipped prompt"},
			wantErr:  ErrSnippetUnmatched,
		},
		{
			name:     "ambiguous snippet",
			snippets: []string{"For every AC"},
			wantErr:  ErrSnippetAmbiguous,
		},
		{
			name: "two snippets resolving to one prompt",
			snippets: []string{
				"whether its steps field follows the Given/When/Then format",
				"steps field does not have at least one Given step",
			},
			wantErr: ErrSnippetDuplicate,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			dir := writeTestFixture(t, "", nil)

			fixture, err := LoadFixture(dir)
			if err != nil {
				t.Fatalf("load fixture: %v", err)
			}

			fixture.ChecklistPrompts = map[string][]string{"us-refine": testCase.snippets}

			_, err = prepareRunDir(fixture, t.TempDir())
			if !errors.Is(err, testCase.wantErr) {
				t.Errorf("want %v, got %v", testCase.wantErr, err)
			}
		})
	}
}

// writeTempChecklist materializes a checklist file for direct
// FilterChecklistFile tests.
func writeTempChecklist(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "us-x.yaml")

	err := os.WriteFile(path, []byte(content), 0o644)
	if err != nil {
		t.Fatalf("write checklist: %v", err)
	}

	return path
}

// TestFilterChecklistFileRejectsUnsupportedYAML pins the deliberate
// refusals: aliases/merge keys (pruning could orphan anchors) and
// multi-document streams (only the first would survive).
func TestFilterChecklistFileRejectsUnsupportedYAML(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		content string
	}{
		{
			name: "alias",
			content: "anchor: &a \"text\"\nsections:\n  - id: s\n    validation_prompts:\n" +
				"      - Q: *a\n        rationale: r\n",
		},
		{
			name: "merge key",
			content: "base: &b\n  rationale: r\nsections:\n  - id: s\n    validation_prompts:\n" +
				"      - <<: *b\n        Q: \"question\"\n",
		},
		{
			name:    "multi document",
			content: "sections: []\n---\nsections: []\n",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			path := writeTempChecklist(t, testCase.content)

			err := FilterChecklistFile(path, []string{"question"})
			if !errors.Is(err, ErrUnsupportedYAML) {
				t.Errorf("want ErrUnsupportedYAML, got %v", err)
			}
		})
	}
}

// TestFilterChecklistFileExcludesSkippedPrompts pins production-parity
// skip semantics: a string skip hides the prompt from selection, while
// a boolean skip (which production decodes to "") does not.
func TestFilterChecklistFileExcludesSkippedPrompts(t *testing.T) {
	t.Parallel()

	content := "sections:\n  - id: s\n    validation_prompts:\n" +
		"      - Q: \"skipped question\"\n        skip: \"flaky\"\n" +
		"      - Q: \"bool-skip question\"\n        skip: false\n"

	path := writeTempChecklist(t, content)

	err := FilterChecklistFile(path, []string{"skipped question"})
	if !errors.Is(err, ErrSnippetUnmatched) {
		t.Errorf("string-skipped prompt must be unselectable, got %v", err)
	}

	path = writeTempChecklist(t, content)

	err = FilterChecklistFile(path, []string{"bool-skip question"})
	if err != nil {
		t.Errorf("bool skip decodes to no-skip in production and must stay selectable, got %v", err)
	}
}

// TestLoadFixtureRejectsEmptyFilterDeclaration pins the
// declared-but-empty failure: a checklist_prompts key selecting nothing
// must not silently run the full checklist.
func TestLoadFixtureRejectsEmptyFilterDeclaration(t *testing.T) {
	t.Parallel()

	for _, extra := range []string{"checklist_prompts: {}\n", "checklist_prompts:\n"} {
		dir := writeTestFixture(t, extra, nil)

		_, err := LoadFixture(dir)
		if !errors.Is(err, ErrFilterDeclaredEmpty) {
			t.Errorf("manifest extra %q: want ErrFilterDeclaredEmpty, got %v", extra, err)
		}
	}
}
