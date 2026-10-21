package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// checksChecklist mixes a model prompt with two built-in checks (ADR
// 0015): the same check twice, told apart by its `with:` parameters.
const checksChecklist = `
sections:
  - id: s
    validation_prompts:
      - Q: "Question one?"
        rationale: "r1"
      - D: given-when-then
        rationale: "kinds"
      - D: given-when-then
        with:
          min_steps: 2
        rationale: "kinds"
        F: "Add the missing step."
`

// loadMiniUniverse builds a universe from one checklist named "mini".
func loadMiniUniverse(t *testing.T, checklist string) *Universe {
	t.Helper()

	dir := t.TempDir()

	err := os.WriteFile(filepath.Join(dir, "mini.yaml"), []byte(checklist), 0o644)
	if err != nil {
		t.Fatalf("writing checklist: %v", err)
	}

	uni, err := LoadUniverse(dir)
	if err != nil {
		t.Fatalf("loading universe: %v", err)
	}

	return uni
}

// TestCheckPromptsAreCensusedByCheck proves a `D:` prompt is counted by
// the check it names and its parameters, never as the empty question it
// carries: two checks do not collide, and each gets its own branches.
func TestCheckPromptsAreCensusedByCheck(t *testing.T) {
	t.Parallel()

	uni := loadMiniUniverse(t, checksChecklist)

	if len(uni.Diagnostics) != 0 {
		t.Fatalf("checks differing in with: must not collide: %v", uni.Diagnostics)
	}

	ids := make([]string, 0, len(uni.Branches))
	for _, branch := range uni.Branches {
		ids = append(ids, branch.ID())
	}

	joined := strings.Join(ids, "\n")

	if len(ids) != 7 {
		t.Fatalf("branches: got %d, want 7 (q1 pass/fail, d2 pass/fail, d3 pass/fail/fix):\n%s", len(ids), joined)
	}

	for _, want := range []string{"mini/s/q1@", "mini/s/d2@", "mini/s/d3@"} {
		if !strings.Contains(joined, want) {
			t.Errorf("branch ids lack %q:\n%s", want, joined)
		}
	}

	check := uni.Prompts[1]
	if check.Snippet != "D: given-when-then" {
		t.Errorf("check snippet: got %q, want %q", check.Snippet, "D: given-when-then")
	}

	if check.QHash == hashText("") {
		t.Error("a check's identity must not be the hash of an empty question")
	}
}

// TestIdenticalCheckPromptsCollide holds the same check with the same
// parameters twice in one checklist to the duplicate-identity diagnostic
// two equal questions draw.
func TestIdenticalCheckPromptsCollide(t *testing.T) {
	t.Parallel()

	uni := loadMiniUniverse(t, `
sections:
  - id: s
    validation_prompts:
      - D: given-when-then
      - D: given-when-then
`)

	if len(uni.Diagnostics) != 1 {
		t.Fatalf("diagnostics: got %v, want one duplicate", uni.Diagnostics)
	}
}

// checkCorpusChecklist ships a model prompt and a built-in check, in the
// mini corpus's checklist slot.
const checkCorpusChecklist = `
version: "1.0"
sections:
  - id: s
    validation_prompts:
      - Q: "Question one?"
        rationale: "r1"
      - D: given-when-then
        rationale: "kinds"
`

// TestCheckCellEarnsShippedCredit proves the census credits a `D:` Cell
// from the verdict the engine saves for it: a schema-bearing turn's JSON
// answer, under the Cell's usual artifact names.
func TestCheckCellEarnsShippedCredit(t *testing.T) {
	t.Parallel()

	corp := newCorpus(t)
	corp.write(filepath.Join(corp.Checklists, "us-mini.yaml"), checkCorpusChecklist)

	run := corp.addFixture("S20", "mini-check", "us mini 9.9")
	corp.withChecklist(run, checkCorpusChecklist)

	verdict := `{"answer":"fail","context":["9.9: missing then step"]}`
	part := filepath.Join(run, "tmp", miniPart)

	corp.write(filepath.Join(part, strings.Replace(resultName(2), "-result.yaml", "-response.txt", 1)), verdict)
	corp.write(filepath.Join(part, resultName(2)), verdict)
	corp.logLines(run, `{"msg":"Loaded prompts","command":"us mini","items":1,"prompts":2}`, lineSaved(2))

	_, profile := scanCorpus(t, corp)

	covered := profile.CoveredIDs()
	if len(covered) != 1 || !strings.Contains(covered[0], "/d2@") || !strings.HasSuffix(covered[0], "#fail") {
		t.Fatalf("covered: got %v, want exactly the check's fail branch", covered)
	}

	if len(profile.Diagnostics.HardRunDiagnostics) != 0 {
		t.Errorf("unexpected hard diagnostics: %v", profile.Diagnostics.HardRunDiagnostics)
	}
}

// TestCheckOverrideMatching grants a fixture's `D:` prompt shipped
// credit only when its check and parameters match a shipped one.
func TestCheckOverrideMatching(t *testing.T) {
	t.Parallel()

	uni := loadMiniUniverse(t, checksChecklist)

	same := newUniversePrompt("mini", "s", 3, 3, promptDoc{
		Check: "given-when-then", With: map[string]any{"min_steps": 2},
		Rationale: "kinds", FixTemplate: "Add the missing step.",
	}, nil)
	if uni.MatchOverride("mini", same) == nil {
		t.Error("an identical check prompt must match the shipped one")
	}

	changed := newUniversePrompt("mini", "s", 3, 3, promptDoc{
		Check: "given-when-then", With: map[string]any{"min_steps": 3},
		Rationale: "kinds", FixTemplate: "Add the missing step.",
	}, nil)
	if uni.MatchOverride("mini", changed) != nil {
		t.Error("a check prompt with other parameters must not match")
	}
}
