package main

import (
	"os"
	"path/filepath"
	"testing"
)

// repoRoot walks up from the package directory to the module root.
func repoRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	for {
		_, statErr := os.Stat(filepath.Join(dir, "go.mod"))
		if statErr == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found above package dir")
		}

		dir = parent
	}
}

// TestUniverseCensus pins the verified shipped inventory: 33 Qs, 9 Fs,
// 75 branches. scen-check adds 8 Qs and no F, so it contributes 16
// pass/fail branches and no fix branch.
func TestUniverseCensus(t *testing.T) {
	t.Parallel()

	uni, err := LoadUniverse(filepath.Join(repoRoot(t), "true-bdd", "checklists"))
	if err != nil {
		t.Fatalf("loading universe: %v", err)
	}

	if got := len(uni.Prompts); got != 33 {
		t.Errorf("prompts: got %d, want 33", got)
	}

	fCount := 0

	for _, p := range uni.Prompts {
		if p.HasF {
			fCount++
		}
	}

	if fCount != 9 {
		t.Errorf("F templates: got %d, want 9", fCount)
	}

	if got := len(uni.Branches); got != 75 {
		t.Errorf("branches: got %d, want 75", got)
	}

	if len(uni.Diagnostics) != 0 {
		t.Errorf("unexpected universe diagnostics: %v", uni.Diagnostics)
	}
}

// TestSkipParity pins the production skip semantics: any non-empty
// string skips (even "false"); a YAML boolean decodes to "" and does
// not skip.
func TestSkipParity(t *testing.T) {
	t.Parallel()

	yamlDoc := `
sections:
  - id: s
    validation_prompts:
      - Q: "kept one?"
      - Q: "skipped string false?"
        skip: "false"
      - Q: "kept bool false?"
        skip: false
      - Q: "skipped anything?"
        skip: "anything"
`

	path := filepath.Join(t.TempDir(), "mini.yaml")

	err := os.WriteFile(path, []byte(yamlDoc), 0o644)
	if err != nil {
		t.Fatalf("writing checklist: %v", err)
	}

	prompts, loadErr := loadChecklistPrompts(path, "mini")
	if loadErr != nil {
		t.Fatalf("loading: %v", loadErr)
	}

	if len(prompts) != 2 {
		t.Fatalf("flattened prompts: got %d, want 2", len(prompts))
	}

	if prompts[0].QCollapsed != "kept one?" || prompts[1].QCollapsed != "kept bool false?" {
		t.Errorf("wrong prompts survived skip filtering: %+v", prompts)
	}

	if prompts[1].Global != 2 || prompts[1].Ordinal != 2 {
		t.Errorf("indices must be assigned after filtering: %+v", prompts[1])
	}
}

// TestOverrideMatching verifies eval-field matching: identical fields
// match; a changed rationale breaks the match even with identical Q.
func TestOverrideMatching(t *testing.T) {
	t.Parallel()

	shipped := `
sections:
  - id: s
    validation_prompts:
      - Q: "same question?"
        rationale: "original"
`

	dir := t.TempDir()

	err := os.WriteFile(filepath.Join(dir, "mini.yaml"), []byte(shipped), 0o644)
	if err != nil {
		t.Fatalf("writing: %v", err)
	}

	uni, uniErr := LoadUniverse(dir)
	if uniErr != nil {
		t.Fatalf("universe: %v", uniErr)
	}

	same := newUniversePrompt("mini", "s", 1, 1,
		promptDoc{Question: "same  question?", Rationale: "original"}, nil)
	if uni.MatchOverride("mini", same) == nil {
		t.Error("whitespace-reflowed identical prompt must match")
	}

	changed := newUniversePrompt("mini", "s", 1, 1,
		promptDoc{Question: "same question?", Rationale: "changed"}, nil)
	if uni.MatchOverride("mini", changed) != nil {
		t.Error("changed rationale must break the eval-field match")
	}
}
