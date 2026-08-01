package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// corpus builds a self-contained synthetic run tree that replicates
// the real artifact shapes (see the Phase-1 inventories) without
// depending on the shipped checklists.
type corpus struct {
	t           *testing.T
	Root        string // run root (tmp/test_run analogue)
	FixturesDir string // fixture manifests dir
	Checklists  string // shipped-checklists analogue
}

const miniChecklist = `
version: "1.0"
sections:
  - id: s
    validation_prompts:
      - Q: "Question one?"
        rationale: "r1"
        docs: [prd]
      - Q: "Question two?"
        rationale: "r2"
        docs: [prd]
        F: "Fix it."
      - Q: "Question three?"
        rationale: "r3"
        docs: [prd]
      - Q: "Question four, never exercised?"
        rationale: "r4"
        docs: [prd]
`

// miniPart is the partition timestamp used across the corpus.
const miniPart = "2026-01-01-00-00"

// miniSubject is the single subject id used across the corpus.
const miniSubject = "9.9"

// newCorpus creates the corpus skeleton with the mini checklist.
func newCorpus(t *testing.T) *corpus {
	t.Helper()

	base := t.TempDir()
	corp := &corpus{
		t:           t,
		Root:        filepath.Join(base, "test_run"),
		FixturesDir: filepath.Join(base, "fixtures"),
		Checklists:  filepath.Join(base, "checklists"),
	}

	corp.write(filepath.Join(corp.Checklists, "us-mini.yaml"), miniChecklist)

	return corp
}

// write creates a file with parent dirs.
func (c *corpus) write(path, content string) {
	c.t.Helper()

	err := os.MkdirAll(filepath.Dir(path), 0o755)
	if err != nil {
		c.t.Fatalf("mkdir: %v", err)
	}

	err = os.WriteFile(path, []byte(content), 0o644)
	if err != nil {
		c.t.Fatalf("write %s: %v", path, err)
	}
}

// addFixture registers a fixture manifest and returns the run dir.
func (c *corpus) addFixture(session, fixture, cmd string) string {
	c.t.Helper()

	c.write(filepath.Join(c.FixturesDir, fixture, "fixture.yaml"), "cmd: "+cmd+"\ninput: input\n")

	runDir := filepath.Join(c.Root, session, fixture)

	err := os.MkdirAll(filepath.Join(runDir, "tmp", miniPart), 0o755)
	if err != nil {
		c.t.Fatalf("mkdir run: %v", err)
	}

	return runDir
}

// withChecklist copies a checklist into the run's config dir.
func (c *corpus) withChecklist(runDir, content string) {
	c.write(filepath.Join(runDir, "true-bdd", "checklists", "us-mini.yaml"), content)
}

// resultName builds the evaluator result filename for the mini corpus.
func resultName(idx int) string {
	return fmt.Sprintf("%02d-%s-checklist-us-mini-s-result.yaml", idx, miniSubject)
}

// evalCell writes a response (with production markers) and matching
// result file for one cell.
func (c *corpus) evalCell(runDir string, idx int, answer string) {
	part := filepath.Join(runDir, "tmp", miniPart)
	resultPath := "tmp/" + miniPart + "/" + resultName(idx)
	body := "answer: " + answer + "\ncontext:\n  - \"detail\"\n"

	response := "prose\n=== FILE_START: " + resultPath + " ===\n" + body +
		"=== FILE_END: " + resultPath + " ===\n"

	c.write(filepath.Join(part, strings.Replace(resultName(idx),
		"-result.yaml", "-response.txt", 1)), response)
	c.write(filepath.Join(part, resultName(idx)), body)
}

// fixEvidence writes the fix-prompts.md and fix-iter1-user.txt pair
// that recovers the pre-fix "fail" verdict for prompt 2 (the corpus
// F-bearing prompt).
func (c *corpus) fixEvidence(runDir string) {
	part := filepath.Join(runDir, "tmp", miniPart)
	prefix := fmt.Sprintf("02-%s-fix", miniSubject)

	c.write(filepath.Join(part, prefix+"-prompts.md"), "generated fix prompt\n")
	c.write(filepath.Join(part, prefix+"-iter1-user.txt"),
		"### Failed Check: us-mini/s\n\n**Question:** ...\n\n**Actual:** fail\n")
}

// applyResult writes one apply confirmation artifact (iteration 1).
func (c *corpus) applyResult(runDir string) {
	c.write(filepath.Join(runDir, "tmp", miniPart,
		"apply-"+miniSubject+"-iter1-result.yaml"), "applied: true\n")
}

// overwriteResult replaces one cell's result.yaml content, detaching it
// from the surviving response (the stale-response signature).
func (c *corpus) overwriteResult(runDir string, idx int, answer string) {
	c.write(filepath.Join(runDir, "tmp", miniPart, resultName(idx)),
		"answer: "+answer+"\n")
}

// logLines writes the run's slog JSON from prebuilt lines.
func (c *corpus) logLines(runDir string, lines ...string) {
	c.write(filepath.Join(runDir, "tmp", "true-bdd.log.json"), strings.Join(lines, "\n")+"\n")
}

// Log line builders mirroring the exact production literals.

func lineLoaded() string {
	return `{"msg":"Loaded prompts","command":"us mini","items":1,"prompts":4}`
}

func lineSaved(idx int) string {
	return fmt.Sprintf(`{"msg":"Result file saved","file":"tmp/%s/%s"}`,
		miniPart, resultName(idx))
}

func lineGen() string {
	return fmt.Sprintf(`{"msg":"Generating fix prompt","subjectID":%q,"promptIndex":2}`,
		miniSubject)
}

func lineApplied() string {
	return fmt.Sprintf(`{"msg":"Fix applied successfully","subjectID":%q}`, miniSubject)
}

func lineWarnBadYAML(idx int) string {
	return fmt.Sprintf(`{"msg":"Failed to parse result YAML","path":"tmp/%s/%s"}`,
		miniPart, resultName(idx))
}
