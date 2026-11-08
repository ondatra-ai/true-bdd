package steps

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

func TestConcurrentValidationStepChecksLogIndicesAgainstTheSelectedChecklist(t *testing.T) {
	t.Parallel()

	for _, example := range []struct {
		name    string
		changed int
	}{
		{"written order and own rechecks", -1},
		{"swapped prompt Qs", 2},
		{"wrong Q after apply", 7},
	} {
		t.Run(example.name, func(t *testing.T) {
			t.Parallel()

			records := concurrentStepRecords()
			if example.changed >= 0 {
				records[example.changed].PromptIndex = 3
			}

			root := t.TempDir()

			var content []byte

			for _, record := range records {
				line, err := json.Marshal(record)
				if err != nil {
					t.Fatal(err)
				}

				content = append(content, append(line, '\n')...)
			}

			files := map[string][]byte{
				"true-bdd/true-bdd.yaml": []byte("paths:\n  checklists_dir: selected\n"),
				"selected/scen-check.yaml": []byte(`sections:
- validation_prompts:
  - Q: {code: given-when-then}
  - Q: {prompt: first}
  - Q: {prompt: parked}
    skip: true
  - Q: {prompt: last}
`),
				engineLogRelPath: content,
			}
			for relative, data := range files {
				path := filepath.Join(root, relative)

				err := disk.Dir(filepath.Dir(path), disk.Private)
				if err != nil {
					t.Fatal(err)
				}

				err = disk.Write(path, data, disk.Private)
				if err != nil {
					t.Fatal(err)
				}
			}

			err := assertConcurrentValidation(runAt(root), nil)
			if (err != nil) != (example.changed >= 0) {
				t.Fatalf("step error=%v", err)
			}
		})
	}
}

func concurrentStepRecords() []engineLogRecord {
	initial := []engineLogRecord{
		{Msg: observedCode, SubjectID: firstSubject, PromptIndex: 1},
		{Msg: observedCode, SubjectID: secondSubject, PromptIndex: 1},
		{Msg: observedStart, SubjectID: firstSubject, PromptIndex: 2},
		{Msg: observedStart, SubjectID: secondSubject, PromptIndex: 2},
		{Msg: observedFinish, SubjectID: firstSubject, PromptIndex: 2},
		{Msg: observedFinish, SubjectID: secondSubject, PromptIndex: 2},
		{Msg: observedApply, SubjectID: firstSubject},
		{Msg: observedStart, SubjectID: firstSubject, PromptIndex: 2},
		{Msg: observedFinish, SubjectID: firstSubject, PromptIndex: 2},
	}
	betaFailed := subjectTrace(secondSubject, 1, 2)
	betaApply := []engineLogRecord{{Msg: observedApply, SubjectID: secondSubject}}

	return slices.Concat(initial, validationTrace(1, 2, 3), betaFailed, betaApply,
		subjectTrace(secondSubject, 2), validationTrace(1, 2, 3), subjectTrace(secondSubject, 1, 2, 3))
}

func subjectTrace(subject string, indices ...int) []engineLogRecord {
	records := validationTrace(indices...)
	for index := range records {
		records[index].SubjectID = subject
	}

	return records
}
