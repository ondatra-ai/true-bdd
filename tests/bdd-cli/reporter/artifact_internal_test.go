package reporter

import (
	"testing"
	"time"
)

// TestCellFromArtifact pins the three artifact shapes back to one cell.
// Getting this wrong makes a fix turn look like a different checklist
// item from the validation turn that produced it.
func TestCellFromArtifact(t *testing.T) {
	cases := []struct {
		name        string
		file        string
		wantSubject string
		wantSection string
		wantOK      bool
	}{
		{
			name:        "validation turn names the section",
			file:        "01-frontend-integration-playwright-startup--checklist-build-code-test-passes-user.txt",
			wantSubject: testSubject,
			wantSection: testSection,
			wantOK:      true,
		},
		{
			name:        "fix turn carries subject only",
			file:        "01-frontend-integration-playwright-startup--fix-iter1-system.txt",
			wantSubject: testSubject,
			wantSection: "",
			wantOK:      true,
		},
		{
			name:        "apply turn is not index-prefixed",
			file:        "apply-frontend-integration-playwright-startup--iter1-user.txt",
			wantSubject: testSubject,
			wantSection: "",
			wantOK:      true,
		},
		{
			name:        "result yaml resolves like its prompt",
			file:        "01-story-99-1--checklist-us-apply-scenarios-result.yaml",
			wantSubject: "story-99-1",
			wantSection: "us-apply-scenarios",
			wantOK:      true,
		},
		{name: "transcript names no cell", file: "crush-turn003-crush.log"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			cell, ok := cellFromArtifact(testCase.file)
			if ok != testCase.wantOK {
				t.Fatalf("ok = %v, want %v", ok, testCase.wantOK)
			}

			if !ok {
				return
			}

			if cell.Subject != testCase.wantSubject {
				t.Errorf("subject = %q, want %q", cell.Subject, testCase.wantSubject)
			}

			if cell.Section != testCase.wantSection {
				t.Errorf("section = %q, want %q", cell.Section, testCase.wantSection)
			}
		})
	}
}

// TestInheritCellSections checks that the fix and apply turns adopt the
// section from the validation turn that opened their cell.
func TestInheritCellSections(t *testing.T) {
	turns := []*Turn{
		{Cell: ChecklistCell{Subject: "subj-a", Section: testSection}},
		{Cell: ChecklistCell{Subject: "subj-a"}},
		{Cell: ChecklistCell{Subject: "subj-b"}},
	}

	inheritCellSections(turns)

	if got := turns[1].Cell.Section; got != testSection {
		t.Errorf("fix turn section = %q, want the validation turn's", got)
	}

	if got := turns[2].Cell.Section; got != "" {
		t.Errorf("unrelated subject inherited %q, want empty", got)
	}
}

// TestArtifactsAttachToTheRightTurn is the positional contract the whole
// detail page rests on: prompts belong to the turn about to start,
// everything else to the turn that just finished.
func TestArtifactsAttachToTheRightTurn(t *testing.T) {
	base := time.Date(2026, time.August, 9, 20, 0, 0, 0, time.UTC)
	step := func(seconds float64) string {
		return base.Add(time.Duration(seconds * float64(time.Second))).Format(time.RFC3339Nano)
	}

	log := &EngineLog{Records: []LogRecord{
		record(step(0), msgPromptSaved, func(r *LogRecord) { r.File = "tmp/x/01-s--checklist-sec-system.txt" }),
		record(step(0), msgPromptSaved, func(r *LogRecord) { r.File = "tmp/x/01-s--checklist-sec-user.txt" }),
		record(step(1), msgDispatch, func(r *LogRecord) { one := 1; r.Turn = &one; r.CLI = cliClaude }),
		record(step(2), msgReturned, func(r *LogRecord) {
			one := 1
			r.Turn = &one
			ms := int64(1000)
			r.DurationMs = &ms
		}),
		record(step(3), msgPromptSaved, func(r *LogRecord) { r.File = "tmp/x/01-s--checklist-sec-response.txt" }),
		record(step(4), msgPromptSaved, func(r *LogRecord) { r.File = "tmp/x/01-s--fix-iter1-user.txt" }),
		record(step(5), msgDispatch, func(r *LogRecord) { two := 2; r.Turn = &two; r.CLI = cliClaude }),
	}}

	turns := log.Turns(t.TempDir())

	if len(turns) != 2 {
		t.Fatalf("got %d turns, want 2", len(turns))
	}

	if len(turns[0].Inputs) != 2 {
		t.Errorf("turn 1 inputs = %d, want the system and user prompt", len(turns[0].Inputs))
	}

	if len(turns[0].Outputs) != 1 {
		t.Errorf("turn 1 outputs = %d, want the response written after it closed",
			len(turns[0].Outputs))
	}

	if len(turns[1].Inputs) != 1 {
		t.Errorf("turn 2 inputs = %d, want the fix prompt written before it started",
			len(turns[1].Inputs))
	}
}

// TestSpawnAttachesToTheOpenTurn guards an ordering that is easy to get
// backwards: the provider logs its argv just AFTER the dispatch, so a
// spawn record belongs to the turn already open. Holding it for the next
// dispatch shifts every command one turn late — invisible while
// consecutive turns share a CLI, and flatly wrong the moment they do
// not.
func TestSpawnAttachesToTheOpenTurn(t *testing.T) {
	base := time.Date(2026, time.August, 9, 23, 0, 0, 0, time.UTC)
	step := func(seconds float64) string {
		return base.Add(time.Duration(seconds * float64(time.Second))).Format(time.RFC3339Nano)
	}

	turnRecord := func(stamp, msg string, number int, cli string) LogRecord {
		return record(stamp, msg, func(r *LogRecord) {
			value := number
			r.Turn = &value
			r.CLI = cli
		})
	}

	spawn := func(stamp, binary string) LogRecord {
		return record(stamp, msgSpawnAgent, func(r *LogRecord) {
			r.Binary = binary
			r.Args = []string{"run"}
		})
	}

	log := &EngineLog{Records: []LogRecord{
		turnRecord(step(0), msgDispatch, 1, cliClaude),
		spawn(step(0), cliClaude),
		turnRecord(step(1), msgReturned, 1, cliClaude),
		turnRecord(step(2), msgDispatch, 2, cliCrush),
		spawn(step(2), cliCrush),
		turnRecord(step(3), msgReturned, 2, cliCrush),
	}}

	turns := log.Turns(t.TempDir())

	if len(turns) != 2 {
		t.Fatalf("got %d turns, want 2", len(turns))
	}

	for index, want := range []string{cliClaude, cliCrush} {
		got := turns[index].Invocation.Binary
		if got != want {
			t.Errorf("turn %d spawned %q, want %q — spawn records are off by one",
				index+1, got, want)
		}

		if turns[index].Invocation.Reconstructed {
			t.Errorf("turn %d should use its logged command, not a reconstruction",
				index+1)
		}
	}
}

// record builds one log record with its timestamp parsed, the way
// loadEngineLog would.
func record(stamp, msg string, apply func(*LogRecord)) LogRecord {
	rec := LogRecord{Time: stamp, Msg: msg}
	if apply != nil {
		apply(&rec)
	}

	rec.At = parseLogTime(rec.Time)

	return rec
}
