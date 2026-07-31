/**
 * NEW v2 relay unit tests (plan §5 vitest) — the stateless relay's pure
 * registry logic (`app/lib/relay/registry.ts`, critique §14). These pin the
 * behaviors that HTTP-level protocol specs (P11/P12) can only observe
 * coarsely: registry expiry ATOMICITY, waiter timeout, reply correlation,
 * queue bounds, and epoch/token rejection.
 *
 * The relay is a contract skeleton (createRelay throws until implemented),
 * so every table here is RED until the relay phase lands — by design.
 */

import { describe, expect, it } from "vitest";

import { createRelay, type Relay, type RegistryConfig } from "../../app/lib/relay/registry";

const CONFIG: RegistryConfig = {
  expiryMs: 10_000,
  perSessionQueueMax: 3,
  globalQueueMax: 5,
  replyBudgetBytes: 1024,
};

/** Registers one session and returns the relay + its fresh credentials. */
function withSession(nowMs = 0): { relay: Relay; sessionId: string; epoch: number; token: string } {
  const relay = createRelay(CONFIG);
  const sessionId = "sess-1";
  const reg = relay.register(sessionId, { pid: 1234, version: "0.0.0" }, nowMs);

  return { relay, sessionId, epoch: reg.connection_epoch, token: reg.capability_token };
}

describe("register: epoch + capability token", () => {
  it("returns a monotonic connection_epoch, a token, and the reply budget", () => {
    const relay = createRelay(CONFIG);
    const first = relay.register("sess-1", { pid: 1, version: "v" }, 0);
    const second = relay.register("sess-1", { pid: 1, version: "v" }, 1);

    expect(second.connection_epoch).toBeGreaterThan(first.connection_epoch);
    expect(second.capability_token).not.toBe("");
    expect(second.reply_budget_bytes).toBe(CONFIG.replyBudgetBytes);
  });
});

describe("poll/reply: epoch + token rejection", () => {
  it("rejects a poll on a stale epoch and a bad token; accepts the current creds", () => {
    const relay = createRelay(CONFIG);
    const first = relay.register("sess-1", { pid: 1, version: "v" }, 0);
    const second = relay.register("sess-1", { pid: 1, version: "v" }, 1); // bumps + invalidates first

    const stale = relay.poll("sess-1", first.connection_epoch, second.capability_token, 2);
    expect(stale).toEqual({ kind: "rejected", reason: "stale_epoch" });

    const badToken = relay.poll("sess-1", second.connection_epoch, "wrong", 2);
    expect(badToken).toEqual({ kind: "rejected", reason: "bad_token" });

    const ok = relay.poll("sess-1", second.connection_epoch, second.capability_token, 2);
    expect(ok.kind).toBe("empty"); // registered, no work queued

    const unknown = relay.poll("other", second.connection_epoch, second.capability_token, 2);
    expect(unknown).toEqual({ kind: "rejected", reason: "unknown_session" });
  });

  it("rejects a late reply from a stale epoch", () => {
    const relay = createRelay(CONFIG);
    const first = relay.register("sess-1", { pid: 1, version: "v" }, 0);
    relay.register("sess-1", { pid: 1, version: "v" }, 1); // bump

    const late = relay.reply(
      { sessionId: "sess-1", epoch: first.connection_epoch, token: first.capability_token, workId: "w" },
      4,
      2,
    );
    expect(late).toEqual({ kind: "rejected", reason: "stale_epoch" });
  });
});

describe("bounds: per-session and global queue caps", () => {
  it("rejects the enqueue that would exceed the per-session cap", () => {
    const { relay, sessionId } = withSession();

    for (let i = 0; i < CONFIG.perSessionQueueMax; i += 1) {
      expect(relay.enqueue(sessionId, { type: "query", payload: { i } }, 0).ok).toBe(true);
    }
    const overflow = relay.enqueue(sessionId, { type: "query", payload: {} }, 0);
    expect(overflow.ok).toBe(false);
    expect(overflow.reason).toBe("queue_full_session");
  });
});

describe("correlation: a reply is routed by work_id only", () => {
  it("replying to A completes A and leaves B untouched", () => {
    const { relay, sessionId, epoch, token } = withSession();

    const a = relay.enqueue(sessionId, { type: "query", payload: { which: "A" } }, 0);
    const b = relay.enqueue(sessionId, { type: "query", payload: { which: "B" } }, 0);
    expect(a.ok && b.ok).toBe(true);

    // Deliver both (two poll/repoll cycles).
    const first = relay.poll(sessionId, epoch, token, 0);
    const second = relay.poll(sessionId, epoch, token, 0);
    expect(first.kind).toBe("work");
    expect(second.kind).toBe("work");

    // Reply to A's work id only.
    const reply = relay.reply({ sessionId, epoch, token, workId: a.work_id as string }, 8, 0);
    expect(reply.kind).toBe("ok");

    expect(relay.waiterState(a.work_id as string)).toBe("replied");
    expect(relay.waiterState(b.work_id as string)).not.toBe("replied");
  });

  it("rejects an over-cap reply body (correlated too_large, not a blind failure)", () => {
    const { relay, sessionId, epoch, token } = withSession();
    const w = relay.enqueue(sessionId, { type: "query", payload: {} }, 0);
    relay.poll(sessionId, epoch, token, 0);

    const over = relay.reply({ sessionId, epoch, token, workId: w.work_id as string }, CONFIG.replyBudgetBytes + 1, 0);
    expect(over).toEqual({ kind: "rejected", reason: "too_large" });
  });
});

describe("expiry: atomic session removal + waiter failure + mutation drop", () => {
  it("sweepExpired removes the session, fails its waiters, and drops queued mutations ATOMICALLY", () => {
    const { relay, sessionId } = withSession(0);

    const query = relay.enqueue(sessionId, { type: "query", payload: {} }, 0);
    const mutation = relay.enqueue(sessionId, { type: "dispatch", payload: {} }, 0);
    expect(query.ok && mutation.ok).toBe(true);

    // No further poll → the session's last-seen ages past expiryMs.
    const result = relay.sweepExpired(CONFIG.expiryMs + 1);

    expect(result.removedSessions).toContain(sessionId);
    expect(result.failedWaiters).toContain(query.work_id);
    expect(result.droppedMutations).toContain(mutation.work_id);

    // After expiry every waiter is terminal — nothing is left queued to be
    // delivered after a reconnect (no store-and-forward, plan §3).
    expect(relay.waiterState(query.work_id as string)).toBe("expired");
    expect(relay.waiterState(mutation.work_id as string)).toBe("expired");
  });

  it("does NOT expire a session whose open poll kept it alive", () => {
    const { relay, sessionId, epoch, token } = withSession(0);

    // An open poll just before the deadline counts as liveness.
    relay.poll(sessionId, epoch, token, CONFIG.expiryMs - 1);
    const result = relay.sweepExpired(CONFIG.expiryMs + 1);
    // The last poll was at expiryMs-1, so at expiryMs+1 the gap is only 2ms.
    expect(result.removedSessions).not.toContain(sessionId);
  });
});

describe("abort: a browser abort cancels an UNDELIVERED operation", () => {
  it("drops a queued (undelivered) mutation but not a delivered one", () => {
    const { relay, sessionId, epoch, token } = withSession(0);

    const queuedMutation = relay.enqueue(sessionId, { type: "answer", payload: {} }, 0);
    const abortQueued = relay.abort(queuedMutation.work_id as string);
    expect(abortQueued.droppedMutation).toBe(true);

    // A mutation already delivered to the CLI is NOT dropped (its retry
    // safety is the CLI DB, plan §3).
    const delivered = relay.enqueue(sessionId, { type: "answer", payload: {} }, 0);
    relay.poll(sessionId, epoch, token, 0); // delivers it
    const abortDelivered = relay.abort(delivered.work_id as string);
    expect(abortDelivered.droppedMutation).toBe(false);
  });
});

// ── Regression suites for the codex-reproduced defects (findings 1 & 2) ──

describe("SECURITY: reply auth + correlation (finding 1)", () => {
  it("crypto tokens/work ids are unguessable and unique — never counters", () => {
    const relay = createRelay(CONFIG);
    const first = relay.register("sess-1", { pid: 1, version: "v" }, 0);
    const second = relay.register("sess-2", { pid: 1, version: "v" }, 0);

    // A UUID token is not a predictable `tok-N` counter.
    expect(first.capability_token).not.toBe(second.capability_token);
    expect(first.capability_token).not.toMatch(/^tok-\d+$/);

    const a = relay.enqueue("sess-1", { type: "query", payload: {} }, 0);
    const b = relay.enqueue("sess-2", { type: "query", payload: {} }, 0);
    expect(a.work_id).not.toBe(b.work_id);
    expect(a.work_id).not.toMatch(/^w-\d+$/);
  });

  it("a WRONG-token cross-session reply CANNOT resolve another session's waiter", () => {
    const relay = createRelay(CONFIG);
    const A = relay.register("sess-A", { pid: 1, version: "v" }, 0);
    const B = relay.register("sess-B", { pid: 2, version: "v" }, 0);

    // B enqueues + is delivered a work item.
    const bWork = relay.enqueue("sess-B", { type: "query", payload: { which: "B" } }, 0);
    const delivered = relay.poll("sess-B", B.connection_epoch, B.capability_token, 0);
    expect(delivered.kind).toBe("work");

    // Attacker (session A, deliberately WRONG token) targets B's work id — the
    // reproduced injection. It must be rejected and leave B's waiter untouched.
    const wrongToken = relay.reply(
      { sessionId: "sess-B", epoch: B.connection_epoch, token: "not-B-token", workId: bWork.work_id as string },
      8,
      0,
    );
    expect(wrongToken).toEqual({ kind: "rejected", reason: "bad_token" });
    expect(relay.waiterState(bWork.work_id as string)).toBe("delivered");

    // Attacker uses their OWN valid creds but B's work id — waiter ownership
    // mismatch, still rejected.
    const foreignWork = relay.reply(
      { sessionId: "sess-A", epoch: A.connection_epoch, token: A.capability_token, workId: bWork.work_id as string },
      8,
      0,
    );
    expect(foreignWork).toEqual({ kind: "rejected", reason: "unknown_work" });
    expect(relay.waiterState(bWork.work_id as string)).toBe("delivered");

    // Only B, with its own current creds, resolves its own delivered work.
    const legit = relay.reply(
      { sessionId: "sess-B", epoch: B.connection_epoch, token: B.capability_token, workId: bWork.work_id as string },
      8,
      0,
    );
    expect(legit).toEqual({ kind: "ok" });
    expect(relay.waiterState(bWork.work_id as string)).toBe("replied");
  });

  it("a reply to work that was never DELIVERED is rejected (at-most-once resolve)", () => {
    const { relay, sessionId, epoch, token } = withSession(0);
    const w = relay.enqueue(sessionId, { type: "query", payload: {} }, 0); // queued, not delivered

    const early = relay.reply({ sessionId, epoch, token, workId: w.work_id as string }, 4, 0);
    expect(early).toEqual({ kind: "rejected", reason: "not_delivered" });

    // Deliver, resolve once, then a duplicate reply is rejected.
    relay.poll(sessionId, epoch, token, 0);
    expect(relay.reply({ sessionId, epoch, token, workId: w.work_id as string }, 4, 0).kind).toBe("ok");
    const dup = relay.reply({ sessionId, epoch, token, workId: w.work_id as string }, 4, 0);
    expect(dup).toEqual({ kind: "rejected", reason: "not_delivered" });
  });

  it("authenticateForFailure authenticates BEFORE failing an over-cap waiter (finding 1)", () => {
    const relay = createRelay(CONFIG);
    const B = relay.register("sess-B", { pid: 2, version: "v" }, 0);
    const bWork = relay.enqueue("sess-B", { type: "query", payload: {} }, 0);
    relay.poll("sess-B", B.connection_epoch, B.capability_token, 0);

    // A wrong-token over-cap failure attempt must NOT resolve the waiter.
    const forged = relay.authenticateForFailure(
      { sessionId: "sess-B", epoch: B.connection_epoch, token: "wrong", workId: bWork.work_id as string },
      0,
    );
    expect(forged).toEqual({ kind: "rejected", reason: "bad_token" });
    expect(relay.waiterState(bWork.work_id as string)).toBe("delivered");

    // The authentic over-cap failure consumes the waiter.
    const authentic = relay.authenticateForFailure(
      { sessionId: "sess-B", epoch: B.connection_epoch, token: B.capability_token, workId: bWork.work_id as string },
      0,
    );
    expect(authentic).toEqual({ kind: "ok" });
    expect(relay.waiterState(bWork.work_id as string)).toBe("cancelled");
  });
});

describe("epoch/expiry: no store-and-forward, no revival (finding 2)", () => {
  it("re-registration DROPS a queued mutation — it is never delivered under the new epoch", () => {
    const relay = createRelay(CONFIG);
    const first = relay.register("sess-1", { pid: 1, version: "v" }, 0);

    // A dispatch is queued but NOT yet delivered.
    const mutation = relay.enqueue("sess-1", { type: "dispatch", payload: {} }, 0);
    expect(mutation.ok).toBe(true);

    // The CLI re-registers (relay restart / reconnect): the new epoch (strictly
    // greater) atomically expires the old queued mutation and reports it so the
    // hub fails its waiter.
    const second = relay.register("sess-1", { pid: 1, version: "v" }, 1);
    expect(second.connection_epoch).toBeGreaterThan(first.connection_epoch);
    expect(second.expiredWaiters).toContain(mutation.work_id);
    expect(relay.waiterState(mutation.work_id as string)).toBe("expired");

    // A poll under the NEW epoch finds nothing — the queued mutation was dropped,
    // never store-and-forwarded across the reconnect.
    const poll = relay.poll("sess-1", second.connection_epoch, second.capability_token, 2);
    expect(poll.kind).toBe("empty");
  });

  it("a poll after the expiry window REJECTS rather than reviving the session", () => {
    const relay = createRelay(CONFIG);
    const reg = relay.register("sess-1", { pid: 1, version: "v" }, 0);
    const queued = relay.enqueue("sess-1", { type: "query", payload: {} }, 0);

    // No poll for longer than expiryMs — the session is expired on next access.
    const late = relay.poll("sess-1", reg.connection_epoch, reg.capability_token, CONFIG.expiryMs + 1);
    expect(late).toEqual({ kind: "rejected", reason: "unknown_session" });

    // Its queued waiter was synchronously expired (not left to be revived).
    expect(relay.waiterState(queued.work_id as string)).toBe("expired");

    // A subsequent register mints a FRESH session — the revived poll never
    // resurrected the old one.
    const reReg = relay.register("sess-1", { pid: 1, version: "v" }, CONFIG.expiryMs + 2);
    expect(reReg.connection_epoch).toBeGreaterThan(reg.connection_epoch);
  });

  it("enqueue after expiry is rejected unknown_session (synchronous expiry)", () => {
    const relay = createRelay(CONFIG);
    relay.register("sess-1", { pid: 1, version: "v" }, 0);

    const late = relay.enqueue("sess-1", { type: "query", payload: {} }, CONFIG.expiryMs + 1);
    expect(late.ok).toBe(false);
    expect(late.reason).toBe("unknown_session");
  });

  it("capacity counts delivered + in-flight work, not just queued (finding 2)", () => {
    const { relay, sessionId, epoch, token } = withSession(0);

    // Fill the per-session cap, then DELIVER them all — they still count.
    const ids: string[] = [];
    for (let i = 0; i < CONFIG.perSessionQueueMax; i += 1) {
      const e = relay.enqueue(sessionId, { type: "query", payload: { i } }, 0);
      expect(e.ok).toBe(true);
      ids.push(e.work_id as string);
      relay.poll(sessionId, epoch, token, 0); // move it queued → delivered
    }

    // A further enqueue is still rejected — delivered work occupies capacity.
    const overflow = relay.enqueue(sessionId, { type: "query", payload: {} }, 0);
    expect(overflow.ok).toBe(false);
    expect(overflow.reason).toBe("queue_full_session");

    // Resolving a delivered waiter frees a slot.
    relay.reply({ sessionId, epoch, token, workId: ids[0] }, 4, 0);
    expect(relay.enqueue(sessionId, { type: "query", payload: {} }, 0).ok).toBe(true);
  });
});
