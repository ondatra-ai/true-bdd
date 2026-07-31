package store

// Answer lifecycle (critique §6): pending-prompt currentness, first-wins,
// exact-retry-200, terminal-race rejection, non-empty clarify/freetext, the
// PromptID actually used at delivery, delivery_started_at committed BEFORE
// the stdin write (at-most-once), and the cross-owner guard (plan §1.1).

import (
	"testing"
)

func seedRunWithPrompt(t *testing.T, store Store, owner, token, kind string) (string, string) {
	t.Helper()

	res := store.Dispatch(DispatchRequest{
		OwnerID: owner, Command: cmdUsRefine, StoryID: "1.1", Fix: true,
		ClientToken: token, RequestHash: "h-" + token,
	})
	if res.Kind != dispatchCreated {
		t.Fatalf("dispatch for prompt run: %+v", res)
	}

	runID := res.RunID
	promptID := "p-" + token

	appended := store.AppendEvent(runID, Event{
		Type: evtPrompt, PromptID: promptID, Kind: kind,
		Payload: `{"options":["one","two","three"]}`, Control: true,
	})
	if appended.Rejected {
		t.Fatalf("prompt event append rejected")
	}

	return runID, promptID
}

func TestAnswerFirstWinsExactRetryAndConflict(t *testing.T) {
	store := requireStore(t, tempDBPath(t))
	seedOwner(t, store, testOwner1)
	runID, promptID := seedRunWithPrompt(t, store, testOwner1, "fw", kindChoice)

	if got := store.SubmitAnswer(AnswerRequest{
		OwnerID: testOwner1, RunID: runID, PromptID: promptID, Kind: kindChoice, Value: answerExit,
	}); got != answerAccepted {
		t.Fatalf("first answer: %s, want accepted", got)
	}

	// Exact retry of the accepted answer stays accepted (a lost-response retry).
	if got := store.SubmitAnswer(AnswerRequest{
		OwnerID: testOwner1, RunID: runID, PromptID: promptID, Kind: kindChoice, Value: answerExit,
	}); got != answerAccepted {
		t.Fatalf("exact retry: %s, want accepted", got)
	}

	// A conflicting answer is a conflict, never a new insert (first-wins).
	if got := store.SubmitAnswer(AnswerRequest{
		OwnerID: testOwner1, RunID: runID, PromptID: promptID, Kind: kindChoice, Value: answerApply,
	}); got != answerConflict {
		t.Fatalf("conflicting answer: %s, want conflict", got)
	}

	life, ok := store.PromptLifecycle(runID, promptID)
	if !ok || life.Answer != answerExit {
		t.Fatalf("stored answer: %+v (ok=%v), want exit", life, ok)
	}
}

func TestAnswerValidation(t *testing.T) {
	store := requireStore(t, tempDBPath(t))
	seedOwner(t, store, testOwner1)

	clarifyRun, clarifyPrompt := seedRunWithPrompt(t, store, testOwner1, "cl", kindClarify)

	if got := store.SubmitAnswer(AnswerRequest{
		OwnerID: testOwner1, RunID: clarifyRun, PromptID: clarifyPrompt, Kind: kindClarify, Value: "",
	}); got != answerInvalid {
		t.Fatalf("empty clarify: %s, want invalid", got)
	}

	if got := store.SubmitAnswer(AnswerRequest{
		OwnerID: testOwner1, RunID: clarifyRun, PromptID: clarifyPrompt, Kind: kindClarify, Value: "with\nnewline",
	}); got != answerInvalid {
		t.Fatalf("multiline clarify: %s, want invalid", got)
	}

	if got := store.SubmitAnswer(AnswerRequest{
		OwnerID: testOwner1, RunID: clarifyRun, PromptID: clarifyPrompt, Kind: kindClarify, Value: "2",
	}); got != answerAccepted {
		t.Fatalf("valid clarify: %s, want accepted", got)
	}

	seedOwner(t, store, testOwner2)

	freeRun, freePrompt := seedRunWithPrompt(t, store, testOwner2, "ft", kindFreetext)

	if got := store.SubmitAnswer(AnswerRequest{
		OwnerID: testOwner2, RunID: freeRun, PromptID: freePrompt, Kind: kindFreetext, Value: "",
	}); got != answerInvalid {
		t.Fatalf("empty freetext: %s, want invalid", got)
	}

	if got := store.SubmitAnswer(AnswerRequest{
		OwnerID: testOwner2, RunID: freeRun, PromptID: freePrompt, Kind: kindFreetext, Value: "multi\nline\n",
	}); got != answerAccepted {
		t.Fatalf("valid freetext: %s, want accepted", got)
	}
}

func TestAnswerUnknownRunAndPrompt(t *testing.T) {
	store := requireStore(t, tempDBPath(t))
	seedOwner(t, store, testOwner1)
	runID, _ := seedRunWithPrompt(t, store, testOwner1, "u", kindChoice)

	if got := store.SubmitAnswer(AnswerRequest{
		OwnerID: testOwner1, RunID: "no-such-run", PromptID: "p", Kind: kindChoice, Value: answerExit,
	}); got != answerNotFound {
		t.Fatalf("unknown run: %s, want not_found", got)
	}

	if got := store.SubmitAnswer(AnswerRequest{
		OwnerID: testOwner1, RunID: runID, PromptID: "no-such-prompt", Kind: kindChoice, Value: answerExit,
	}); got != answerInvalid {
		t.Fatalf("unknown prompt: %s, want invalid", got)
	}
}

func TestAnswerTerminalRaceRejected(t *testing.T) {
	store := requireStore(t, tempDBPath(t))
	seedOwner(t, store, testOwner1)
	runID, promptID := seedRunWithPrompt(t, store, testOwner1, "tr", kindChoice)

	if got := store.SubmitAnswer(AnswerRequest{
		OwnerID: testOwner1, RunID: runID, PromptID: promptID, Kind: kindChoice, Value: answerExit,
	}); got != answerAccepted {
		t.Fatalf("pre-terminal answer: %s", got)
	}

	terminate(t, store, runID)

	// After terminal: an EXACT retry of the already-accepted answer stays
	// accepted, but a fresh/conflicting answer is rejected.
	if got := store.SubmitAnswer(AnswerRequest{
		OwnerID: testOwner1, RunID: runID, PromptID: promptID, Kind: kindChoice, Value: answerExit,
	}); got != answerAccepted {
		t.Fatalf("post-terminal exact retry: %s, want accepted", got)
	}

	if got := store.SubmitAnswer(AnswerRequest{
		OwnerID: testOwner1, RunID: runID, PromptID: promptID, Kind: kindChoice, Value: answerApply,
	}); got != answerConflict {
		t.Fatalf("post-terminal conflicting answer: %s, want conflict", got)
	}
}

func TestAnswerDeliveryStartedBeforeConsumed(t *testing.T) {
	store := requireStore(t, tempDBPath(t))
	seedOwner(t, store, testOwner1)
	runID, promptID := seedRunWithPrompt(t, store, testOwner1, "dl", kindChoice)

	if got := store.SubmitAnswer(AnswerRequest{
		OwnerID: testOwner1, RunID: runID, PromptID: promptID, Kind: kindChoice, Value: answerExit,
	}); got != answerAccepted {
		t.Fatalf("answer: %s", got)
	}

	// delivery_started_at is committed BEFORE the stdin write (at-most-once):
	// after BeginDelivery it is set while consumed_at is still nil.
	err := store.BeginDelivery(runID, promptID)
	if err != nil {
		t.Fatalf("begin delivery: %v", err)
	}

	mid, ok := store.PromptLifecycle(runID, promptID)
	if !ok || mid.DeliveryStartedAt == nil {
		t.Fatalf("delivery_started_at must be set before the stdin write: %+v", mid)
	}

	if mid.ConsumedAt != nil {
		t.Fatalf("consumed_at must be nil until the child confirms: %+v", mid)
	}

	err = store.ConfirmConsumed(runID, promptID)
	if err != nil {
		t.Fatalf("confirm consumed: %v", err)
	}

	done, _ := store.PromptLifecycle(runID, promptID)
	if done.ConsumedAt == nil || *done.ConsumedAt < *done.DeliveryStartedAt {
		t.Fatalf("consumed_at must follow delivery_started_at: %+v", done)
	}
}

func TestAnswerCrossOwnerGuard(t *testing.T) {
	store := requireStore(t, tempDBPath(t))
	seedOwner(t, store, "owner-B")
	runID, promptID := seedRunWithPrompt(t, store, "owner-B", "xo", kindChoice)
	seedOwner(t, store, "owner-A")

	// answer-via-A: the run's owner is B, so A's session is read-only for it —
	// rejected cross_owner, changing no row.
	if got := store.SubmitAnswer(AnswerRequest{
		OwnerID: "owner-A", RunID: runID, PromptID: promptID, Kind: kindChoice, Value: answerExit,
	}); got != answerCrossOwner {
		t.Fatalf("answer via a non-owner: %s, want cross_owner", got)
	}

	life, _ := store.PromptLifecycle(runID, promptID)
	if life.Answer != "" {
		t.Fatalf("cross-owner answer must not store a value: %+v", life)
	}

	// answer-via-B (the owner) succeeds.
	if got := store.SubmitAnswer(AnswerRequest{
		OwnerID: "owner-B", RunID: runID, PromptID: promptID, Kind: kindChoice, Value: answerExit,
	}); got != answerAccepted {
		t.Fatalf("answer via the owner: %s, want accepted", got)
	}
}
