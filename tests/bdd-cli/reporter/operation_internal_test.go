package reporter

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// applyChecklist mirrors the shape of true-bdd/checklists/us-apply.yaml
// as the rewalk fixture filters it: one section, one prompt, with a fix
// template. The `F:` matters — a prompt that declares one is answered
// against Q and F both.
const applyChecklist = `
sections:
  - id: merge
    name: "Scenario Merge"
    validation_prompts:
      - Q: "does this AC already appear as a scenario in the registry?"
        F: "add this AC as a new scenario"
`

// The whole point of the exercise: turn #1 and turn #4 of
// us-apply-rewalk-converges are both role `prompt` on the same cell, and
// they must not read alike. #4 exists BECAUSE #3 applied a fix.
func TestOperationNamesTheRewalkFixture(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	write(t, dir, "true-bdd/checklists/us-apply.yaml", applyChecklist)
	writeLog(t, dir, applyRunRecords(t))

	fixture := loadFixtureForTest(t, dir)

	want := []string{
		"#1 Validate 99.3-001 against Q[1] — walk 1",
		"#2 Generate fix prompt for 99.3-001 from F[1] — Q[1] answered fail",
		"#3 Apply fix to 99.3-001 from 01-99.3-001-fix-prompts.md — user applied fix #1",
		"#4 Re-validate 99.3-001 against Q[1] — after fix #1 was applied",
	}

	got := make([]string, 0, len(fixture.Turns))
	for _, turn := range fixture.Turns {
		got = append(got, "#"+strconv.Itoa(turn.Number)+" "+turn.Op.Label+" — "+turn.Op.Why)
	}

	if len(got) != len(want) {
		t.Fatalf("turns = %d, want %d: %v", len(got), len(want), got)
	}

	for index := range want {
		if got[index] != want[index] {
			t.Errorf("turn %d:\n got %q\nwant %q", index+1, got[index], want[index])
		}
	}
}

// The compare view pairs turns by cell, so a matched row can hold a
// first entry on one side and a re-entry on the other. CellLabel must
// therefore never say "Re-".
func TestOperationCellLabelIsAttemptFree(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	write(t, dir, "true-bdd/checklists/us-apply.yaml", applyChecklist)
	writeLog(t, dir, applyRunRecords(t))

	fixture := loadFixtureForTest(t, dir)
	revalidate := fixture.Turns[3]

	if !strings.HasPrefix(revalidate.Op.Label, verbReValidate) {
		t.Errorf("label = %q, want it to start with %q", revalidate.Op.Label, verbReValidate)
	}

	if strings.Contains(revalidate.Op.CellLabel, "Re-") {
		t.Errorf("cell label = %q, must not claim a re-entry", revalidate.Op.CellLabel)
	}

	if revalidate.Op.CellLabel != fixture.Turns[0].Op.Label {
		t.Errorf("cell labels differ across attempts: %q vs %q",
			revalidate.Op.CellLabel, fixture.Turns[0].Op.Label)
	}
}

// A re-entry caused by ANOTHER item's fix is a re-walk, not this item's
// restart, and the two mean different things to a reader. The only
// evidence separating them is the walk boundary record.
func TestOperationSeparatesReWalkFromRestart(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	write(t, dir, "true-bdd/checklists/us-apply.yaml", applyChecklist)
	writeLog(t, dir, append(applyRunRecords(t),
		logLine(t, "2026-08-10T23:24:00Z", "Walk attempt started", map[string]any{
			"attempt": 2, "max_attempts": 5}),
		// 99.3-001 was already walked and already fixed, so walk 2 seeing
		// it again is the re-walk. 99.3-002 is a first look that happens
		// to fall in walk 2 — a different thing, and it must not borrow
		// the re-walk's explanation.
		promptTurn(t, 5, firstAC),
		promptTurn(t, 6, "99.3-002"),
	))

	fixture := loadFixtureForTest(t, dir)
	reWalked := fixture.Turns[4]
	firstLook := fixture.Turns[5]

	if reWalked.Cause.Kind != causeReWalk {
		t.Fatalf("cause = %q, want %q", reWalked.Cause.Kind, causeReWalk)
	}

	if reWalked.Op.Why != "re-walk 2/5 — fixes applied in walk 1" {
		t.Errorf("why = %q", reWalked.Op.Why)
	}

	if firstLook.Cause.Kind != causeWalk || firstLook.Op.Why != "walk 2" {
		t.Errorf("first look = %q / %q, want a plain walk 2",
			firstLook.Cause.Kind, firstLook.Op.Why)
	}
}

// A section can hold several prompts — us-create's `format` holds two —
// and they are different questions, not retries of one. Counting them
// as attempts of a single seat would label the second question
// "Re-validate", which is a claim about a check that had never run.
func TestOperationDoesNotConflatePromptsOfOneSection(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	write(t, dir, "true-bdd/checklists/us-create.yaml", `
sections:
  - id: format
    name: "Format"
    validation_prompts:
      - Q: "first question"
      - Q: "second question"
`)
	writeLog(t, dir, []string{
		logLine(t, "2026-08-10T23:17:01Z", "Loading full checklist", map[string]any{
			fieldCommand: "us-create", fieldPath: "true-bdd/checklists/us-create.yaml"}),
		sectionPromptTurn(t, 1, 1, "99.1", "us-create-format"),
		sectionPromptTurn(t, 2, 2, "99.1", "us-create-format"),
	})

	fixture := loadFixtureForTest(t, dir)

	for index, turn := range fixture.Turns {
		if turn.Attempt != 1 {
			t.Errorf("turn %d attempt = %d, want 1 — separate prompts are separate seats",
				index+1, turn.Attempt)
		}

		if !strings.HasPrefix(turn.Op.Label, verbValidate) {
			t.Errorf("turn %d label = %q, want a first-entry verb", index+1, turn.Op.Label)
		}
	}

	if fixture.Turns[1].Op.Ref != "Q[2]" {
		t.Errorf("second prompt ref = %q, want Q[2]", fixture.Turns[1].Op.Ref)
	}
}

// build-code subject ids contain slashes and angle brackets, so the
// filename carries a flattened form while the log carries the raw one.
// Keyed on the raw id, a build-code turn never finds the fix that was
// just applied to it and reports the wrong cause.
func TestOperationMatchesSanitisedSubjectIDs(t *testing.T) {
	t.Parallel()

	const raw = `frontend/integration/playwright:<startup>`

	dir := t.TempDir()
	write(t, dir, "true-bdd/checklists/build-code.yaml", `
sections:
  - id: test-passes
    name: "Test Passes"
    validation_prompts:
      - Q: "does the test pass?"
        F: "make it pass"
`)
	writeLog(t, dir, []string{
		logLine(t, "2026-08-10T23:02:42Z", "Loading full checklist", map[string]any{
			fieldCommand: "build-code", fieldPath: "true-bdd/checklists/build-code.yaml"}),
		sectionPromptTurn(t, 1, 1, testSubject+"-", testSection),
		logLine(t, "2026-08-10T23:03:00Z", "Generating fix prompt", map[string]any{
			fieldSubjectID: raw, fieldPromptIndex: 1,
			fieldSection: "build-code/test-passes", fieldIteration: 1}),
		logLine(t, "2026-08-10T23:03:00Z", "Prompt saved", map[string]any{
			fieldFile: "tmp/run/01-" + testSubject + "--fix-iter1-system.txt"}),
		dispatch(t, "2026-08-10T23:03:01Z", 2, roleFix, cliClaude, opusModel),
		returned(t, "2026-08-10T23:03:40Z", 2, roleFix),
		logLine(t, "2026-08-10T23:03:40Z", "Fix prompt saved", map[string]any{
			fieldFile: "tmp/run/01-" + testSubject + "--fix-prompts.md"}),
		logLine(t, "2026-08-10T23:03:40Z", "Applying fix prompt", map[string]any{
			fieldSubjectID: raw, fieldIteration: 1}),
		logLine(t, "2026-08-10T23:03:40Z", "Prompt saved", map[string]any{
			fieldFile: "tmp/run/apply-" + testSubject + "--iter1-system.txt"}),
		dispatch(t, "2026-08-10T23:03:41Z", 3, roleApply, cliCrush, crushModel),
		returned(t, "2026-08-10T23:05:00Z", 3, roleApply),
		logLine(t, "2026-08-10T23:05:00Z", "Fix applied successfully", map[string]any{
			fieldSubjectID: raw}),
		sectionPromptTurn(t, 4, 1, testSubject+"-", testSection),
	})

	fixture := loadFixtureForTest(t, dir)
	apply, revalidate := fixture.Turns[2], fixture.Turns[3]

	if apply.Op.Ref != "01-"+testSubject+"--fix-prompts.md" {
		t.Errorf("apply ref = %q, want the generated fix prompt", apply.Op.Ref)
	}

	if revalidate.Cause.Kind != causeAfterFix || revalidate.Cause.FixNumber != 1 {
		t.Errorf("re-validation cause = %+v, want the fix it followed", revalidate.Cause)
	}
}

// The point of the whole exercise is naming what was compared against
// what, so the file provenance has to survive the trip: the subject's
// source, the checklist that posed the question, the file fixes mutate,
// and every document the prompt actually resolved.
func TestOperationCarriesFileProvenance(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	write(t, dir, "true-bdd/checklists/us-refine.yaml", `
sections:
  - id: acceptance_criteria
    name: "Acceptance Criteria Quality"
    validation_prompts:
      - Q: "count the acceptance criteria"
`)
	writeLog(t, dir, []string{
		logLine(t, "2026-08-10T23:17:01Z", "Loading full checklist", map[string]any{
			fieldCommand: "us-refine", fieldPath: "true-bdd/checklists/us-refine.yaml"}),
		logLine(t, "2026-08-10T23:17:01Z", "Story document loaded", map[string]any{
			"story_number": "96.3", fieldFile: refineStoryFile}),
		logLine(t, "2026-08-10T23:17:01Z", "Seeded scratch registry", map[string]any{
			"from": "docs/scenarios.yaml", "to": scratchFile}),
		logLine(t, "2026-08-10T23:21:00Z", "Prompt documents resolved", map[string]any{
			fieldSubjectID: "96.3", fieldPromptIndex: 1,
			fieldSection: "us-refine/acceptance_criteria",
			"docs": map[string]string{
				"prd":               "docs/prd/prd.yaml",
				"architecture_yaml": "docs/architecture/architecture.yaml",
			}}),
		logLine(t, "2026-08-10T23:21:00Z", "Prompt saved", map[string]any{
			fieldFile: "tmp/run/01-96.3-checklist-us-refine-acceptance_criteria-system.txt"}),
		dispatch(t, "2026-08-10T23:21:01Z", 1, rolePrompt, cliClaude, opusModel),
		returned(t, "2026-08-10T23:21:20Z", 1, rolePrompt),
	})

	fixture := loadFixtureForTest(t, dir)

	if fixture.Meta.ItemsFile != refineStoryFile {
		t.Errorf("items file = %q", fixture.Meta.ItemsFile)
	}

	if fixture.Meta.TargetFile != scratchFile {
		t.Errorf("target file = %q", fixture.Meta.TargetFile)
	}

	turn := fixture.Turns[0]

	want := []string{
		"architecture_yaml → docs/architecture/architecture.yaml",
		"prd → docs/prd/prd.yaml",
	}
	if strings.Join(turn.Docs, "|") != strings.Join(want, "|") {
		t.Errorf("docs = %v, want %v sorted", turn.Docs, want)
	}

	if turn.Op.SectionName != testSectionName {
		t.Errorf("section name = %q, want the checklist author's own name", turn.Op.SectionName)
	}
}

// A prompt with no `F:` — every us-create prompt, seven of us-refine's
// twelve — is answered against Q alone. Naming an F index there would
// point at something that does not exist.
func TestOperationOmitsAbsentFixTemplate(t *testing.T) {
	t.Parallel()

	turn := &Turn{Role: rolePrompt, Attempt: 1, PromptIdx: 3, Cell: ChecklistCell{Subject: "99.1"}}
	doc := ChecklistDoc{Loaded: true, Prompts: []ChecklistPrompt{
		{SectionID: "format"}, {SectionID: "who"}, {SectionID: "why"},
	}}

	operation := describeTurn(turn, doc, "us-create")
	if operation.Ref != "Q[3]" {
		t.Errorf("ref = %q, want Q[3] with no F", operation.Ref)
	}

	if operation.Label != "Validate 99.1 against Q[3]" {
		t.Errorf("label = %q", operation.Label)
	}
}

// With no checklist to read, the section id still has to come from
// somewhere: the filename carries the command glued onto it. Every
// session recorded before this feature existed lands here.
func TestOperationFallsBackToTheFilenameSection(t *testing.T) {
	t.Parallel()

	turn := &Turn{
		Role: rolePrompt, Attempt: 1, PromptIdx: 1,
		Cell: ChecklistCell{Subject: testSubject, Section: testSection},
	}

	operation := describeTurn(turn, ChecklistDoc{}, "build-code")
	if operation.Section != "test-passes" {
		t.Errorf("section = %q, want the command prefix stripped", operation.Section)
	}

	if operation.SectionName != "" {
		t.Errorf("section name = %q, want empty when no checklist was read", operation.SectionName)
	}
}

// The clarification cap is the engine's, not the report's: iterations
// past maxClarificationIterations are user refinements
// (cell_handler.go), and calling those "clarification round 7" would
// misreport who drove the turn.
func TestFixCauseSplitsClarificationFromRefinement(t *testing.T) {
	t.Parallel()

	cases := []struct {
		iteration int
		kind      string
		phrase    string
	}{
		{1, causeCheckFailed, "Q[1] answered fail"},
		{2, causeClarify, "clarification round 2"},
		{5, causeClarify, "clarification round 5"},
		{6, causeRefine, "user refinement 1"},
		{8, causeRefine, "user refinement 3"},
	}

	for _, testCase := range cases {
		cause := fixCause(testCase.iteration)
		if cause.Kind != testCase.kind {
			t.Errorf("iteration %d: kind = %q, want %q", testCase.iteration, cause.Kind, testCase.kind)
		}

		if got := cause.Phrase("Q[1]"); got != testCase.phrase {
			t.Errorf("iteration %d: phrase = %q, want %q", testCase.iteration, got, testCase.phrase)
		}
	}
}

// A turn whose artifacts never named a prompt index, and which logged no
// documents of its own, must read as "index unknown". Carrying the last
// index seen would resolve it against the checklist and print a section
// name and a Q[n] belonging to an entirely different check.
func TestTurnWithoutItsOwnRecordsInheritsNoIndex(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	write(t, dir, "true-bdd/checklists/us-apply.yaml", applyChecklist)
	writeLog(t, dir, []string{
		logLine(t, "2026-08-10T23:17:01Z", "Loading full checklist", map[string]any{
			fieldCommand: "us-apply", fieldPath: "true-bdd/checklists/us-apply.yaml"}),
		sectionPromptTurn(t, 1, 1, firstAC, "us-apply-merge"),
		// A turn killed before it wrote a single artifact: no cell, and no
		// documents record of its own.
		dispatch(t, "2026-08-10T23:22:00Z", 2, rolePrompt, cliClaude, opusModel),
		returned(t, "2026-08-10T23:22:20Z", 2, rolePrompt),
	})

	fixture := loadFixtureForTest(t, dir)
	orphan := fixture.Turns[1]

	if orphan.PromptIdx != 0 {
		t.Errorf("prompt index = %d, want 0 — it borrowed turn 1's", orphan.PromptIdx)
	}

	if orphan.Op.Ref != "" {
		t.Errorf("ref = %q, want empty rather than another check's question", orphan.Op.Ref)
	}

	if orphan.Op.SectionName != "" {
		t.Errorf("section name = %q, want empty", orphan.Op.SectionName)
	}
}

// A cause the log gives no evidence for produces no clause at all. An
// invented "walk 1" on a session that never logged its walk boundaries
// would be a guess presented as a fact.
func TestUnknownCauseSaysNothing(t *testing.T) {
	t.Parallel()

	if phrase := (TurnCause{}).Phrase("Q[1]"); phrase != "" {
		t.Errorf("phrase = %q, want empty", phrase)
	}
}

// loadFixtureForTest loads a synthetic run directory the way the server
// does, so the test exercises the real assembly rather than a stand-in.
func loadFixtureForTest(t *testing.T, dir string) *Fixture {
	t.Helper()

	fixture, err := LoadFixtureDir(dir, "synthetic", dir)
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}

	return fixture
}

// logLine renders one engine log line the way slog's JSON handler does,
// so these tests feed the reporter exactly the bytes the engine writes.
func logLine(t *testing.T, at, msg string, fields map[string]any) string {
	t.Helper()

	fields["time"] = at
	fields["level"] = "INFO"
	fields["msg"] = msg

	encoded, err := json.Marshal(fields)
	if err != nil {
		t.Fatalf("encode record: %v", err)
	}

	return string(encoded)
}

// applyRunRecords is the record stream us-apply-rewalk-converges wrote
// for its first AC: validate → diagnose → apply → re-validate.
func applyRunRecords(t *testing.T) []string {
	t.Helper()

	return []string{
		logLine(t, "2026-08-10T23:17:01Z", "Loading full checklist", map[string]any{
			fieldCommand: "us-apply", fieldPath: "true-bdd/checklists/us-apply.yaml"}),
		logLine(t, "2026-08-10T23:17:01Z", "Parsed story scenarios for apply", map[string]any{
			"story": "99.3", fieldFile: storyFile}),
		logLine(t, "2026-08-10T23:17:01Z", "Seeded scratch registry", map[string]any{
			"from": "docs/scenarios.yaml", "to": scratchFile}),
		logLine(t, "2026-08-10T23:17:01Z", "Walk attempt started", map[string]any{
			"attempt": 1, "max_attempts": 5}),
		promptTurn(t, 1, firstAC),
		logLine(t, "2026-08-10T23:18:00Z", "Generating fix prompt", map[string]any{
			fieldSubjectID: firstAC, fieldPromptIndex: 1,
			fieldSection: "us-apply/merge", fieldIteration: 1}),
		logLine(t, "2026-08-10T23:18:00Z", "Prompt saved", map[string]any{
			fieldFile: "tmp/run/01-99.3-001-fix-iter1-system.txt"}),
		dispatch(t, "2026-08-10T23:18:01Z", 2, roleFix, cliClaude, opusModel),
		returned(t, "2026-08-10T23:18:40Z", 2, roleFix),
		logLine(t, "2026-08-10T23:18:40Z", "Fix prompt saved", map[string]any{
			fieldFile: "tmp/run/01-99.3-001-fix-prompts.md"}),
		logLine(t, "2026-08-10T23:18:40Z", "User chose fix action", map[string]any{
			"rawInput": "1", "action": "apply"}),
		logLine(t, "2026-08-10T23:18:40Z", "Applying fix prompt", map[string]any{
			fieldSubjectID: firstAC, fieldIteration: 1}),
		logLine(t, "2026-08-10T23:18:40Z", "Prompt saved", map[string]any{
			fieldFile: "tmp/run/apply-99.3-001-iter1-system.txt"}),
		dispatch(t, "2026-08-10T23:18:41Z", 3, roleApply, cliCrush, crushModel),
		returned(t, "2026-08-10T23:21:00Z", 3, roleApply),
		logLine(t, "2026-08-10T23:21:00Z", "Fix applied successfully", map[string]any{
			fieldSubjectID: firstAC}),
		promptTurn(t, 4, firstAC),
	}
}

// dispatch and returned bracket one model call.
func dispatch(t *testing.T, timestamp string, number int, role, cli, model string) string {
	t.Helper()

	return logLine(t, timestamp, "Dispatching AI turn", map[string]any{
		"turn": number, "role": role, "cli": cli, "model": model})
}

func returned(t *testing.T, timestamp string, number int, role string) string {
	t.Helper()

	return logLine(t, timestamp, "AI turn returned", map[string]any{
		"turn": number, "role": role, "duration_ms": 20000})
}

// promptTurn is one validation turn's records: the documents it
// resolved, the prompt file it wrote, the dispatch and the return.
func promptTurn(t *testing.T, number int, subject string) string {
	t.Helper()

	return sectionPromptTurn(t, number, 1, subject, "us-apply-merge")
}

// sectionPromptTurn is promptTurn with the prompt index and flattened
// section spelled out, for the cases where those are the point.
func sectionPromptTurn(t *testing.T, number, index int, subject, section string) string {
	t.Helper()

	timestamp := "2026-08-10T23:2" + strconv.Itoa(number) + ":00Z"
	name := fmt.Sprintf("tmp/run/%02d-%s-checklist-%s-system.txt", index, subject, section)

	return strings.Join([]string{
		logLine(t, timestamp, "Prompt documents resolved", map[string]any{
			fieldSubjectID: subject, fieldPromptIndex: index,
			fieldSection: section, "docs": map[string]string{}}),
		logLine(t, timestamp, "Prompt saved", map[string]any{fieldFile: name}),
		dispatch(t, timestamp, number, rolePrompt, cliClaude, opusModel),
		returned(t, timestamp, number, rolePrompt),
	}, "\n")
}

func writeLog(t *testing.T, dir string, records []string) {
	t.Helper()

	path := filepath.Join(dir, "tmp", "true-bdd.log.json")

	err := os.MkdirAll(filepath.Dir(path), 0o755)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	err = os.WriteFile(path, []byte(strings.Join(records, "\n")+"\n"), 0o600)
	if err != nil {
		t.Fatalf("write log: %v", err)
	}
}
