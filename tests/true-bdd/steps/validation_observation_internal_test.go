package steps

import (
	"slices"
	"testing"
)

const (
	observedApply  = "Applying fix prompt"
	firstSubject   = "alpha"
	observedCode   = "Check answered"
	observedStart  = "Cell validation started"
	observedFinish = "Cell validation finished"
	secondSubject  = "beta"
)

func TestValidationObservationFollowsEveryChecklistItem(t *testing.T) {
	t.Parallel()

	for _, example := range []struct {
		name  string
		trace []engineLogRecord
		fails bool
	}{
		{"written order", validationTrace(1, 2, 3), false},
		{"swapped prompt Qs", validationTrace(1, 3, 2), true},
		{"missing code Q", validationTrace(2, 3), true},
		{"missing index", validationTrace(1, 0), true},
		{"correct recheck and restart", repairTrace(2, 1, 2, 3), false},
		{"wrong recheck", repairTrace(3, 1, 2, 3), true},
		{"restart before recheck", repairTrace(1, 2, 3), true},
		{"restart omits first Q", repairTrace(2, 2, 3), true},
		{"incomplete recheck", repairTrace(), true},
		{"incomplete walk", validationTrace(1, 2), true},
		{"explicit exit permits prefix", exitTrace(validationTrace(1, 2)), false},
		{"exit cannot omit recheck", exitTrace(repairTrace()), true},
		{"overlap within subject", []engineLogRecord{
			{Msg: observedCode, SubjectID: firstSubject, PromptIndex: 1},
			{Msg: observedStart, SubjectID: firstSubject, PromptIndex: 2},
			{Msg: observedStart, SubjectID: firstSubject, PromptIndex: 3},
		}, true},
		{"wrong model finish", []engineLogRecord{
			{Msg: observedCode, SubjectID: firstSubject, PromptIndex: 1},
			{Msg: observedStart, SubjectID: firstSubject, PromptIndex: 2},
			{Msg: observedFinish, SubjectID: firstSubject, PromptIndex: 3},
		}, true},
	} {
		t.Run(example.name, func(t *testing.T) {
			t.Parallel()

			observation := newValidationObservation(3)

			var err error
			for _, record := range example.trace {
				err = observation.observe(record)
				if err != nil {
					break
				}
			}

			if err == nil {
				err = observation.complete()
			}

			if (err != nil) != example.fails {
				t.Fatalf("error=%v, want failure=%v", err, example.fails)
			}
		})
	}
}

func validationTrace(indices ...int) []engineLogRecord {
	var records []engineLogRecord

	for _, index := range indices {
		if index == 1 {
			records = append(records, engineLogRecord{Msg: observedCode, SubjectID: firstSubject, PromptIndex: index})
		} else {
			records = append(records,
				engineLogRecord{Msg: observedStart, SubjectID: firstSubject, PromptIndex: index},
				engineLogRecord{Msg: observedFinish, SubjectID: firstSubject, PromptIndex: index})
		}
	}

	return records
}

func repairTrace(indices ...int) []engineLogRecord {
	records := validationTrace(1, 2)
	records = append(records, engineLogRecord{Msg: observedApply, SubjectID: firstSubject})

	return append(records, validationTrace(indices...)...)
}

func TestValidationObservationAllowsOverlapBetweenSubjects(t *testing.T) {
	t.Parallel()

	observation := newValidationObservation(3)
	records := []engineLogRecord{
		{Msg: observedCode, SubjectID: firstSubject, PromptIndex: 1},
		{Msg: observedStart, SubjectID: firstSubject, PromptIndex: 2},
		{Msg: observedCode, SubjectID: secondSubject, PromptIndex: 1},
		{Msg: observedStart, SubjectID: secondSubject, PromptIndex: 2},
		{Msg: observedFinish, SubjectID: secondSubject, PromptIndex: 2},
		{Msg: observedFinish, SubjectID: firstSubject, PromptIndex: 2},
		{Msg: observedApply, SubjectID: secondSubject},
		{Msg: observedStart, SubjectID: secondSubject, PromptIndex: 2},
		{Msg: observedFinish, SubjectID: secondSubject, PromptIndex: 2},
	}

	other := validationTrace(1, 2, 3)
	for index := range other {
		other[index].SubjectID = secondSubject
	}

	records = slices.Concat(records, validationTrace(1, 2, 3), other)

	for _, record := range records {
		err := observation.observe(record)
		if err != nil {
			t.Fatal(err)
		}
	}

	err := observation.complete()
	if err != nil {
		t.Fatal(err)
	}

	if !observation.overlap {
		t.Fatal("missing observed overlap between subjects")
	}
}

func exitTrace(records []engineLogRecord) []engineLogRecord {
	return append(records, engineLogRecord{Msg: "User chose fix action", Action: "exit"})
}
