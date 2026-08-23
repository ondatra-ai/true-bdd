package store

// Regression for the reproduced answer DOUBLE-DELIVERY (finding 3): ResolveAnswer
// must report ShouldDeliver=true ONLY on the first accept, never on a retry —
// see TestResolveAnswerDeliversOnceThenNeverAgain. Plus delivery_error recording.

import "testing"

func TestResolveAnswerDeliversOnceThenNeverAgain(t *testing.T) {
	store := requireStore(t, tempDBPath(t))
	seedOwner(t, store, testOwner1)
	runID, promptID := seedRunWithPrompt(t, store, testOwner1, "dd", kindChoice)

	first := store.ResolveAnswer(AnswerRequest{
		OwnerID: testOwner1, RunID: runID, PromptID: promptID, Kind: kindChoice, Value: answerExit,
	})
	if first.Outcome != answerAccepted || !first.ShouldDeliver {
		t.Fatalf("first accept: %+v, want accepted with ShouldDeliver=true", first)
	}

	if first.StoredKind != kindChoice {
		t.Fatalf("stored kind = %q, want choice", first.StoredKind)
	}

	// The reproduced defect: an EXACT retry (lost-response retry) stays accepted
	// but must NEVER be delivered again — otherwise the duplicate stdin line is
	// consumed as the NEXT prompt's answer.
	retry := store.ResolveAnswer(AnswerRequest{
		OwnerID: testOwner1, RunID: runID, PromptID: promptID, Kind: kindChoice, Value: answerExit,
	})
	if retry.Outcome != answerAccepted {
		t.Fatalf("exact retry: %+v, want accepted", retry)
	}

	if retry.ShouldDeliver {
		t.Fatal("an exact retry must NOT re-deliver (the double-delivery regression)")
	}

	// A conflicting answer never re-delivers either (first-wins).
	conflict := store.ResolveAnswer(AnswerRequest{
		OwnerID: testOwner1, RunID: runID, PromptID: promptID, Kind: kindChoice, Value: answerApply,
	})
	if conflict.Outcome != answerConflict || conflict.ShouldDeliver {
		t.Fatalf("conflicting answer: %+v, want conflict without delivery", conflict)
	}

	// A cross-owner submission never delivers and changes nothing.
	seedOwner(t, store, testOwner2)

	cross := store.ResolveAnswer(AnswerRequest{
		OwnerID: testOwner2, RunID: runID, PromptID: promptID, Kind: kindChoice, Value: answerExit,
	})
	if cross.Outcome != answerCrossOwner || cross.ShouldDeliver {
		t.Fatalf("cross-owner: %+v, want cross_owner without delivery", cross)
	}
}

func TestRecordDeliveryErrorPersistsDiagnostic(t *testing.T) {
	store := requireStore(t, tempDBPath(t))
	seedOwner(t, store, testOwner1)
	runID, promptID := seedRunWithPrompt(t, store, testOwner1, "de", kindChoice)

	if got := store.SubmitAnswer(AnswerRequest{
		OwnerID: testOwner1, RunID: runID, PromptID: promptID, Kind: kindChoice, Value: answerExit,
	}); got != answerAccepted {
		t.Fatalf("answer: %s", got)
	}

	err := store.RecordDeliveryError(runID, promptID, "stdin_write_failed")
	if err != nil {
		t.Fatalf("record delivery error: %v", err)
	}

	got, err := store.Scalar(
		`SELECT COUNT(*) FROM prompts WHERE run_id = ? AND delivery_error = 'stdin_write_failed'`, runID)
	if err != nil {
		t.Fatalf("scalar: %v", err)
	}

	if got != 1 {
		t.Fatalf("delivery_error rows = %d, want 1", got)
	}
}
