package reporter

import (
	"os"
	"path/filepath"
	"testing"
)

// The flattening MUST match the engine's (checklist_loader.go): sections
// in file order, prompts in file order, skipped prompts omitted. That
// walk assigns the %02d index in every artifact filename, so an
// off-by-one here mislabels every turn in the run.
func TestLoadChecklistDocFlattensLikeTheEngine(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	write(t, dir, "checklists/us-refine.yaml", `
sections:
  - id: acceptance_criteria
    name: "Acceptance Criteria Quality"
    validation_prompts:
      - Q: "first"
      - Q: "second"
        F: "fix me"
      - Q: "skipped"
        skip: "not yet"
  - id: invest
    name: "INVEST"
    validation_prompts:
      - Q: "third"
`)

	doc := loadChecklistDoc(dir, "checklists/us-refine.yaml")
	if !doc.Loaded {
		t.Fatal("doc did not load")
	}

	if len(doc.Prompts) != 3 {
		t.Fatalf("prompts = %d, want 3 (the skipped one must not consume an index)", len(doc.Prompts))
	}

	cases := []struct {
		index   int
		section string
		name    string
		hasFix  bool
	}{
		{1, "acceptance_criteria", testSectionName, false},
		{2, "acceptance_criteria", testSectionName, true},
		{3, "invest", "INVEST", false},
	}

	for _, testCase := range cases {
		got, ok := doc.Prompt(testCase.index)
		if !ok {
			t.Fatalf("prompt %d missing", testCase.index)
		}

		if got.SectionID != testCase.section || got.SectionName != testCase.name {
			t.Errorf("prompt %d = %q/%q, want %q/%q",
				testCase.index, got.SectionID, got.SectionName, testCase.section, testCase.name)
		}

		if got.HasFix != testCase.hasFix {
			t.Errorf("prompt %d HasFix = %v, want %v", testCase.index, got.HasFix, testCase.hasFix)
		}
	}
}

// Indices are 1-based because artifact filenames are. Asking for 0 or
// past the end must not panic and must not silently return the wrong
// prompt.
func TestChecklistDocPromptBounds(t *testing.T) {
	t.Parallel()

	doc := ChecklistDoc{Prompts: []ChecklistPrompt{{SectionID: "merge"}}}

	if _, ok := doc.Prompt(0); ok {
		t.Error("index 0 must not resolve")
	}

	if _, ok := doc.Prompt(2); ok {
		t.Error("index past the end must not resolve")
	}

	got, ok := doc.Prompt(1)
	if !ok || got.SectionID != "merge" {
		t.Errorf("prompt 1 = %+v ok=%v", got, ok)
	}
}

// A session recorded before the engine logged its checklist path, or one
// whose checklist has since been deleted, must degrade to an unloaded
// doc — the label then falls back to the raw section id rather than the
// whole report failing.
func TestLoadChecklistDocDegrades(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	if doc := loadChecklistDoc(dir, ""); doc.Loaded {
		t.Error("empty path must not report as loaded")
	}

	if doc := loadChecklistDoc(dir, "checklists/gone.yaml"); doc.Loaded {
		t.Error("missing file must not report as loaded")
	}

	write(t, dir, "broken.yaml", "sections: [oh: no\n")

	if doc := loadChecklistDoc(dir, "broken.yaml"); doc.Loaded {
		t.Error("malformed YAML must not report as loaded")
	}
}

func write(t *testing.T, dir, rel, body string) {
	t.Helper()

	path := filepath.Join(dir, rel)

	err := os.MkdirAll(filepath.Dir(path), 0o755)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	err = os.WriteFile(path, []byte(body), 0o600)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
}
