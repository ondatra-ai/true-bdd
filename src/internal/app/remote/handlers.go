package remote

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/ondatra-ai/true-bdd/src/internal/app/inventory"
	"github.com/ondatra-ai/true-bdd/src/internal/app/store"
)

// handleWork dispatches one work item to its handler and replies. Each work
// item is handled in its own goroutine (the poll loop already repolled), so a
// slow inventory scan never blocks the poll. Mutations are serialized by the
// store (one nonterminal run per owner) and the single active executor.
func (a *Agent) handleWork(ctx context.Context, item workItem) {
	switch item.Type {
	case workQuery:
		a.handleQuery(ctx, item)
	case workDispatch:
		a.handleDispatch(ctx, item)
	case workAnswer:
		a.handleAnswer(ctx, item)
	default:
		a.reply(ctx, item, replyEnvelope{Status: http.StatusBadRequest, Body: errorBody("unknown_work_type")})
	}
}

// handleQuery builds the requested projection and replies with {status:200,
// body:<projection>} (plan §3).
func (a *Agent) handleQuery(ctx context.Context, item workItem) {
	var payload queryPayload

	err := json.Unmarshal(item.Payload, &payload)
	if err != nil {
		a.reply(ctx, item, replyEnvelope{Status: http.StatusBadRequest, Body: errorBody("invalid_query")})

		return
	}

	switch payload.View {
	case viewSessionStatus:
		a.reply(ctx, item, replyEnvelope{Status: http.StatusOK, Body: a.read.sessionStatus(a.sessionID)})
	case viewSessionDetail:
		a.reply(ctx, item, replyEnvelope{Status: http.StatusOK, Body: a.sessionDetail()})
	case viewRunDetail:
		a.reply(ctx, item, a.runDetailReply(payload.RunID))
	default:
		a.reply(ctx, item, replyEnvelope{Status: http.StatusBadRequest, Body: errorBody("invalid_view")})
	}
}

// sessionDetail builds session_status PLUS a fresh live inventory scan taken
// under the scan lock (plan §1.5): a planted command intent ⇒ inventory_busy ⇒
// inventory is null for this read (no retained memo).
func (a *Agent) sessionDetail() SessionDetail {
	status := a.read.sessionStatus(a.sessionID)
	detail := SessionDetail{SessionStatus: status}

	lease, outcome := a.locks.BeginScan()
	if outcome != lockOutcomeOK {
		return detail // inventory_busy (or lock error) ⇒ inventory: null
	}
	defer lease.Release()

	snapshot := inventory.ScanWithBudget(a.folder, a.inventoryBudget(status))
	detail.Inventory = &snapshot

	return detail
}

// inventoryBudget derives the snapshot-fit budget from the negotiated
// inventory budget minus the MEASURED reply envelope (the wrapper around the
// snapshot), so the serialized snapshot fits the server's full-request budget
// (plan §1.5). A non-negotiated (zero) budget falls back to a floor. This is
// the INVENTORY budget (configurable per server), NOT the large streamed reply
// cap — a tiny inventory budget degrades the snapshot (snapshot_truncated /
// limit_too_small) without 413-ing the read.
func (a *Agent) inventoryBudget(status SessionStatus) int {
	budget := a.getInventoryBudget()
	if budget <= 0 {
		budget = replyBudgetFloor
	}

	empty := inventory.Snapshot{}
	envelope := replyEnvelope{Status: http.StatusOK, Body: SessionDetail{SessionStatus: status, Inventory: &empty}}

	raw, err := json.Marshal(envelope)
	if err == nil {
		budget -= len(raw)
	}

	if budget < 1 {
		budget = 1
	}

	return budget
}

// runDetailReply projects a run detail, mapping absence to 404 run_pruned (a
// pruned run whose receipt survives) or 404 not_found (plan §3).
func (a *Agent) runDetailReply(runID string) replyEnvelope {
	detail, found, pruned := a.read.runDetail(a.sessionID, runID)
	if found {
		return replyEnvelope{Status: http.StatusOK, Body: detail}
	}

	if pruned {
		return replyEnvelope{Status: http.StatusNotFound, Body: errorBody("run_pruned")}
	}

	return replyEnvelope{Status: http.StatusNotFound, Body: errorBody("not_found")}
}

// handleDispatch runs the store dispatch transaction, replies with the mapped
// status, and (on created) spawns the run (plan §1.4/§3). The reply is sent
// BEFORE the spawn so lock acquisition never delays the browser's dispatch
// response.
func (a *Agent) handleDispatch(ctx context.Context, item workItem) {
	var payload dispatchPayload

	err := json.Unmarshal(item.Payload, &payload)
	if err != nil {
		a.reply(ctx, item, replyEnvelope{Status: http.StatusBadRequest, Body: errorBody("invalid")})

		return
	}

	outcome := a.store.Dispatch(store.DispatchInput{
		OwnerID:     a.sessionID,
		Command:     payload.Command,
		StoryID:     payload.StoryID,
		Fix:         payload.Fix,
		ClientToken: payload.ClientToken,
		RequestHash: requestHash(payload.Command, payload.StoryID, payload.Fix),
	})

	a.reply(ctx, item, dispatchReply(outcome))

	if outcome.Kind == dispatchKindCreated {
		// Track the executor so a normal shutdown waits for it (finding 4).
		a.runWG.Add(1)

		go func() {
			defer a.runWG.Done()

			a.spawnRun(ctx, outcome.RunID)
		}()
	}
}

// dispatchReply maps a store dispatch outcome to a browser reply (plan §1.4).
func dispatchReply(outcome store.DispatchOutcome) replyEnvelope {
	switch outcome.Kind {
	case dispatchKindCreated:
		return replyEnvelope{Status: http.StatusCreated, Body: map[string]any{"run_id": outcome.RunID}}
	case dispatchKindDeduped:
		return replyEnvelope{Status: http.StatusOK, Body: map[string]any{"run_id": outcome.RunID}}
	case dispatchKindConflict:
		body := map[string]any{"error": "conflict"}
		if outcome.RunID != "" {
			body["run_id"] = outcome.RunID
		}

		return replyEnvelope{Status: http.StatusConflict, Body: body}
	default:
		return replyEnvelope{Status: http.StatusBadRequest, Body: errorBody("invalid")}
	}
}

// spawnRun loads the created run's command from the store and executes it as a
// child (plan §1.6/§3). Execute BLOCKS while it holds the command lease.
func (a *Agent) spawnRun(ctx context.Context, runID string) {
	spec, ok := a.read.runCommand(runID)
	if !ok {
		slog.Warn("remote: created run vanished before spawn", "run", runID)

		return
	}

	executor := NewRunExecutor(a.store, a.locks, a.children, a.binPath, a.folder, a.sessionID, spec)

	a.setActive(executor)
	executor.Execute(ctx)
	a.setActive(nil)
}

// handleAnswer resolves an answer through the store's atomic answer lifecycle,
// replies with the mapped status, and (only on the FIRST accept) delivers it to
// the child's stdin under the at-most-once + matching-local-executor guards
// (plan §1.4, finding 3). Kind/value are validated BEFORE the store commits, so
// an invalid answer is never persisted then rejected at delivery.
func (a *Agent) handleAnswer(ctx context.Context, item workItem) {
	var payload answerPayload

	err := json.Unmarshal(item.Payload, &payload)
	if err != nil {
		a.reply(ctx, item, replyEnvelope{Status: http.StatusBadRequest, Body: errorBody("invalid")})

		return
	}

	kind, ok := a.read.promptKind(payload.RunID, payload.PromptID)
	if !ok {
		// Unknown prompt — invalid before any commit.
		a.reply(ctx, item, replyEnvelope{Status: http.StatusBadRequest, Body: errorBody("invalid")})

		return
	}

	// Validate the answer against the prompt kind BEFORE the store commits it
	// (finding 3): a choice outside the enum or an over-cap value is rejected
	// invalid — never persisted then rejected at delivery time.
	validationErr := validateAnswer(kind, payload.Value)
	if validationErr != nil {
		a.reply(ctx, item, replyEnvelope{Status: http.StatusBadRequest, Body: errorBody("invalid")})

		return
	}

	// Capture the active executor ONCE so the delivery guard and the store
	// commit reason about the same live-prompt snapshot.
	exec := a.getActive()

	resolution := a.store.ResolveAnswer(store.AnswerInput{
		OwnerID:  a.sessionID,
		RunID:    payload.RunID,
		PromptID: payload.PromptID,
		Kind:     kind,
		Value:    payload.Value,
	})

	// Deliver ONLY the first accept — an exact retry (ShouldDeliver=false) never
	// re-writes the child's stdin (finding 3: the reproduced double-delivery).
	if string(resolution.Outcome) == answerOutcomeAccepted && resolution.ShouldDeliver {
		a.deliverAnswer(exec, payload, resolution.StoredKind)
	}

	a.reply(ctx, item, answerReply(resolution.Outcome))
}

// deliverAnswer delivers a FRESHLY-accepted answer to the child's stdin under
// the at-most-once guard (plan §1.4, finding 3): it requires a matching LOCAL
// executor, commits delivery_started_at BEFORE the write, and stamps
// consumed_at ONLY after a SUCCESSFUL write. If there is no matching executor
// or the write fails, it records delivery_error and terminates the run
// (interrupting the child) so the run never hangs answered-but-blocked.
func (a *Agent) deliverAnswer(exec *RunExecutor, payload answerPayload, kind string) {
	// Matching-local-executor guard (plan §1.1): the run must be executing HERE
	// for the answer to be deliverable.
	if exec == nil || exec.RunID() != payload.RunID {
		a.failDelivery(nil, payload, "no_local_executor")

		return
	}

	err := a.store.BeginDelivery(payload.RunID, payload.PromptID)
	if err != nil {
		slog.Warn("remote: begin delivery failed", "run", payload.RunID, "prompt", payload.PromptID, "error", err)
		a.failDelivery(exec, payload, "begin_delivery_failed")

		return
	}

	deliverErr := exec.DeliverAnswer(kind, payload.Value)
	if deliverErr != nil {
		slog.Warn("remote: deliver answer to child failed", "run", payload.RunID, "error", deliverErr)
		a.failDelivery(exec, payload, "stdin_write_failed")

		return
	}

	// consumed_at is stamped ONLY after a SUCCESSFUL stdin write (finding 3).
	err = a.store.ConfirmConsumed(payload.RunID, payload.PromptID)
	if err != nil {
		slog.Warn("remote: confirm consumed failed", "run", payload.RunID, "prompt", payload.PromptID, "error", err)
	}
}

// failDelivery records a delivery_error for a store-accepted answer that could
// not reach the child and terminates the run so it never hangs answered-but-
// blocked, interrupting the child if one is running (finding 3).
func (a *Agent) failDelivery(exec *RunExecutor, payload answerPayload, detail string) {
	err := a.store.RecordDeliveryError(payload.RunID, payload.PromptID, detail)
	if err != nil {
		slog.Warn("remote: record delivery error failed", "run", payload.RunID, "error", err)
	}

	a.store.AppendEvent(payload.RunID, store.EventInput{
		Type:        eventTypeTerminal,
		Outcome:     outcomeError,
		ErrorDetail: detailNoResult,
		Control:     true,
	})

	if exec != nil {
		exec.Interrupt()
	}
}

// answerReply maps a store answer outcome to a browser reply (plan §1.4).
func answerReply(outcome store.AnswerOutcome) replyEnvelope {
	switch string(outcome) {
	case answerOutcomeAccepted:
		return replyEnvelope{Status: http.StatusOK, Body: map[string]any{"ok": true}}
	case answerOutcomeConflict:
		return replyEnvelope{Status: http.StatusConflict, Body: errorBody("conflict")}
	case answerOutcomeCrossOwner:
		return replyEnvelope{Status: http.StatusConflict, Body: errorBody("cross_owner")}
	case answerOutcomeNotFound:
		return replyEnvelope{Status: http.StatusNotFound, Body: errorBody("not_found")}
	default:
		return replyEnvelope{Status: http.StatusBadRequest, Body: errorBody("invalid")}
	}
}

// reply posts the CLI result for one work item, correlating via the session +
// epoch bound INTO the work item plus the current capability token (plan §2).
func (a *Agent) reply(ctx context.Context, item workItem, env replyEnvelope) {
	status, err := a.client.Reply(ctx, item.SessionID, item.ConnectionEpoch, a.getToken(), item.WorkID, env)
	if err != nil {
		if ctx.Err() == nil {
			slog.Warn("remote: reply failed", "work", item.WorkID, "error", err)
		}

		return
	}

	if status != http.StatusOK {
		slog.Warn("remote: reply not accepted", "work", item.WorkID, "status", status)
	}
}

// requestHash is the dispatch idempotency fingerprint over the command args
// (plan §1.4): an exact retry hashes identically; a token reuse with different
// args mismatches and is rejected 409.
func requestHash(command, storyID string, fix bool) string {
	sum := sha256.Sum256([]byte(command + "\x00" + storyID + "\x00" + strconv.FormatBool(fix)))

	return hex.EncodeToString(sum[:])
}

// errorBody is the canonical {"error": reason} reply body.
func errorBody(reason string) map[string]any {
	return map[string]any{"error": reason}
}

// Store dispatch outcome kinds + answer outcome strings (mirrored from the
// store package's exported string contract — the constants themselves are
// unexported there).
const (
	dispatchKindCreated  = "created"
	dispatchKindDeduped  = "deduped"
	dispatchKindConflict = "conflict"

	answerOutcomeAccepted   = "accepted"
	answerOutcomeConflict   = "conflict"
	answerOutcomeCrossOwner = "cross_owner"
	answerOutcomeNotFound   = "not_found"
)
