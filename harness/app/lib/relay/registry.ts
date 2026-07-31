/**
 * v2 STATELESS RELAY — pure registry logic (plan §3, critique §14).
 *
 * This module is the pinned CONTRACT the test-first suite
 * (`tests/unit/relay-registry.test.ts`) exercises. The relay holds NO
 * database: it is an in-process register / poll / reply message bus whose
 * pure, deterministic core (session registry, bounded work queues, waiter
 * lifecycle, atomic expiry) is isolated here so it can be unit-tested
 * without HTTP. The three Next route handlers share ONE instance via an
 * explicit globalThis singleton (critique §14); that wiring is NOT part of
 * this pure core.
 *
 * Security invariants (critique findings 1 & 2):
 *   - capability tokens and work ids are CRYPTOGRAPHIC (crypto.randomUUID),
 *     never guessable counters;
 *   - every waiter is bound to {sessionId, epoch}; a reply is accepted ONLY
 *     for the SAME session + epoch + capability token AND only when the work
 *     is `delivered` — so a wrong-token / cross-session reply can never
 *     resolve another session's waiter;
 *   - re-registration atomically EXPIRES the old epoch's waiters and DROPS
 *     its queued mutations (no store-and-forward);
 *   - expiry is checked SYNCHRONOUSLY on every access (poll/reply/enqueue) so
 *     a poll after the expiry window REJECTS rather than reviving the session;
 *   - terminal waiters are retained only as BOUNDED tombstones; capacity
 *     counts in-flight (queued + delivered) work only, so the waiter map can
 *     never grow without bound.
 */

/** Cryptographically-random opaque id (capability token / work id). */
function randomToken(): string {
  return globalThis.crypto.randomUUID();
}

export interface RegistryConfig {
  /** Absence-of-poll expiry (ms). An open poll counts as liveness (plan §3). */
  expiryMs: number;
  /** Per-session bounded in-flight work depth (429/503 when exhausted). */
  perSessionQueueMax: number;
  /** Global bounded in-flight work depth across all sessions. */
  globalQueueMax: number;
  /** Negotiated reply byte cap returned at register time. */
  replyBudgetBytes: number;
}

export interface RegisterResult {
  connection_epoch: number;
  capability_token: string;
  reply_budget_bytes: number;
  /** Work ids of the PREVIOUS epoch that this registration expired — the hub
   * fails their awaiting browser waiters (plan §3, finding 2). */
  expiredWaiters: string[];
}

export type WorkType = "query" | "dispatch" | "answer";

export interface EnqueueResult {
  ok: boolean;
  work_id?: string;
  reason?: "queue_full_session" | "queue_full_global" | "unknown_session";
}

export type PollResult =
  | { kind: "work"; item: { work_id: string; type: WorkType; payload: unknown } }
  | { kind: "empty" }
  | { kind: "rejected"; reason: "unknown_session" | "stale_epoch" | "bad_token" };

export type ReplyRejectReason =
  | "stale_epoch"
  | "bad_token"
  | "unknown_work"
  | "not_delivered"
  | "too_large"
  | "unknown_session";

export type ReplyResult = { kind: "ok" } | { kind: "rejected"; reason: ReplyRejectReason };

export type WaiterState = "queued" | "delivered" | "replied" | "cancelled" | "expired";

/** The atomic outcome of an expiry sweep (plan §3 — all-or-nothing). */
export interface ExpiryResult {
  removedSessions: string[];
  failedWaiters: string[];
  droppedMutations: string[];
}

export interface Correlation {
  sessionId: string;
  epoch: number;
  token: string;
  workId: string;
}

export interface Relay {
  register(sessionId: string, meta: { pid: number; version: string }, nowMs: number): RegisterResult;
  enqueue(sessionId: string, work: { type: WorkType; payload: unknown }, nowMs: number): EnqueueResult;
  poll(sessionId: string, epoch: number, token: string, nowMs: number): PollResult;
  reply(correlation: Correlation, bodyByteLength: number, nowMs: number): ReplyResult;
  /** Authenticates a reply's correlation WITHOUT (or without trusting) its
   * body — used for an over-cap (413) or malformed (502) reply, so the failure
   * still resolves the CORRECT browser waiter and ONLY that session's waiter
   * (finding 1). Consumes the waiter on success so it is never double-resolved. */
  authenticateForFailure(correlation: Correlation, nowMs: number): ReplyResult;
  waiterState(workId: string): WaiterState | undefined;
  /** Removes sessions whose last poll is older than expiryMs, atomically
   * failing their waiters and dropping their queued mutations (plan §3). */
  sweepExpired(nowMs: number): ExpiryResult;
  /** Browser abort: cancels an UNDELIVERED operation (read OR mutation).
   * An already-delivered mutation is NOT dropped (its retry safety is the
   * CLI DB). Returns whether a queued mutation was dropped. */
  abort(workId: string): { droppedMutation: boolean };
}

export function createRelay(config: RegistryConfig): Relay {
  interface SessionState {
    epoch: number;
    token: string;
    lastSeen: number;
  }

  interface WaiterRecord {
    workId: string;
    sessionId: string;
    epoch: number;
    type: WorkType;
    payload: unknown;
    state: WaiterState;
  }

  const sessions = new Map<string, SessionState>();
  // Map insertion order == enqueue order, so iterating yields FIFO delivery.
  const waiters = new Map<string, WaiterRecord>();
  // Bounded FIFO of terminal work ids: retained so `waiterState` can still
  // report a recently-finished outcome, but evicted past the cap so the map
  // never grows without bound (finding 2).
  const tombstones: string[] = [];
  const TOMBSTONE_CAP = 512;

  let epochCounter = 0;

  const isMutation = (type: WorkType): boolean => type === "dispatch" || type === "answer";
  const isInFlight = (state: WaiterState): boolean => state === "queued" || state === "delivered";

  /** Retires a waiter to a terminal state and records a bounded tombstone,
   * evicting the oldest terminal record when the cap is exceeded. */
  const retire = (waiter: WaiterRecord, state: "replied" | "cancelled" | "expired"): void => {
    waiter.state = state;
    tombstones.push(waiter.workId);
    while (tombstones.length > TOMBSTONE_CAP) {
      const evicted = tombstones.shift();
      if (evicted !== undefined) {
        const record = waiters.get(evicted);
        // Only evict a still-terminal record (an id could be reused? no — ids
        // are unique, so this is always the same terminal waiter).
        if (record !== undefined && !isInFlight(record.state)) {
          waiters.delete(evicted);
        }
      }
    }
  };

  /** Expires every in-flight waiter of one session (used by re-register and
   * by the expiry sweep). Appends outcomes to the accumulators. */
  const expireSessionWaiters = (
    sessionId: string,
    failedWaiters: string[],
    droppedMutations: string[],
  ): void => {
    for (const waiter of waiters.values()) {
      if (waiter.sessionId !== sessionId || !isInFlight(waiter.state)) {
        continue;
      }
      const wasQueued = waiter.state === "queued";
      failedWaiters.push(waiter.workId);
      if (wasQueued && isMutation(waiter.type)) {
        // Queued mutations are dropped — never store-and-forwarded (plan §3).
        droppedMutations.push(waiter.workId);
      }
      retire(waiter, "expired");
    }
  };

  /** Synchronous expiry check on access: if the session's last poll is older
   * than expiryMs, remove it and expire its waiters, returning true (the
   * session is gone — a later poll must REJECT, never revive). */
  const expireIfStale = (sessionId: string, nowMs: number): boolean => {
    const session = sessions.get(sessionId);
    if (session === undefined) {
      return true;
    }
    if (nowMs - session.lastSeen <= config.expiryMs) {
      return false;
    }
    sessions.delete(sessionId);
    expireSessionWaiters(sessionId, [], []);

    return true;
  };

  const countInFlight = (sessionId: string): { session: number; global: number } => {
    let session = 0;
    let global = 0;
    for (const waiter of waiters.values()) {
      if (!isInFlight(waiter.state)) {
        continue;
      }
      global += 1;
      if (waiter.sessionId === sessionId) {
        session += 1;
      }
    }

    return { session, global };
  };

  /** Shared correlation gate for reply/authenticateOverCap: proves the caller
   * holds the session's CURRENT epoch + token and owns the DELIVERED waiter. */
  const authenticate = (correlation: Correlation, nowMs: number): { ok: true; waiter: WaiterRecord } | ReplyResult => {
    if (expireIfStale(correlation.sessionId, nowMs)) {
      return { kind: "rejected", reason: "unknown_session" };
    }
    const session = sessions.get(correlation.sessionId);
    if (session === undefined) {
      return { kind: "rejected", reason: "unknown_session" };
    }
    if (correlation.epoch !== session.epoch) {
      return { kind: "rejected", reason: "stale_epoch" };
    }
    if (correlation.token !== session.token) {
      return { kind: "rejected", reason: "bad_token" };
    }
    const waiter = waiters.get(correlation.workId);
    if (waiter === undefined || waiter.sessionId !== correlation.sessionId || waiter.epoch !== correlation.epoch) {
      // Unknown, or belongs to a DIFFERENT session/epoch — never resolvable
      // by this caller (defeats cross-session injection, finding 1).
      return { kind: "rejected", reason: "unknown_work" };
    }
    if (waiter.state !== "delivered") {
      // A reply for queued/terminal work is rejected (at-most-once resolve).
      return { kind: "rejected", reason: "not_delivered" };
    }

    return { ok: true, waiter };
  };

  const relay: Relay = {
    register(sessionId, _meta, nowMs) {
      // Re-registration atomically expires the PREVIOUS epoch's waiters and
      // drops its queued mutations BEFORE the new credentials take effect —
      // no store-and-forward across a reconnect (plan §3, finding 2).
      const expiredWaiters: string[] = [];
      if (sessions.has(sessionId)) {
        expireSessionWaiters(sessionId, expiredWaiters, []);
      }

      epochCounter += 1;
      const epoch = epochCounter;
      const token = randomToken();
      sessions.set(sessionId, { epoch, token, lastSeen: nowMs });

      return {
        connection_epoch: epoch,
        capability_token: token,
        reply_budget_bytes: config.replyBudgetBytes,
        expiredWaiters,
      };
    },

    enqueue(sessionId, work, nowMs) {
      if (expireIfStale(sessionId, nowMs)) {
        return { ok: false, reason: "unknown_session" };
      }
      const session = sessions.get(sessionId);
      if (session === undefined) {
        return { ok: false, reason: "unknown_session" };
      }

      const counts = countInFlight(sessionId);
      if (counts.session >= config.perSessionQueueMax) {
        return { ok: false, reason: "queue_full_session" };
      }
      if (counts.global >= config.globalQueueMax) {
        return { ok: false, reason: "queue_full_global" };
      }

      const workId = randomToken();
      waiters.set(workId, {
        workId,
        sessionId,
        epoch: session.epoch,
        type: work.type,
        payload: work.payload,
        state: "queued",
      });

      return { ok: true, work_id: workId };
    },

    poll(sessionId, epoch, token, nowMs) {
      if (expireIfStale(sessionId, nowMs)) {
        // Absent OR aged past expiry — a poll never revives a gone session.
        return { kind: "rejected", reason: "unknown_session" };
      }
      const session = sessions.get(sessionId);
      if (session === undefined) {
        return { kind: "rejected", reason: "unknown_session" };
      }
      if (epoch !== session.epoch) {
        return { kind: "rejected", reason: "stale_epoch" };
      }
      if (token !== session.token) {
        return { kind: "rejected", reason: "bad_token" };
      }

      // A completed poll counts as liveness for the expiry sweep (plan §3).
      session.lastSeen = nowMs;

      for (const waiter of waiters.values()) {
        if (waiter.sessionId === sessionId && waiter.epoch === session.epoch && waiter.state === "queued") {
          waiter.state = "delivered";

          return {
            kind: "work",
            item: { work_id: waiter.workId, type: waiter.type, payload: waiter.payload },
          };
        }
      }

      return { kind: "empty" };
    },

    reply(correlation, bodyByteLength, nowMs) {
      const auth = authenticate(correlation, nowMs);
      if ("kind" in auth) {
        return auth;
      }
      if (bodyByteLength > config.replyBudgetBytes) {
        // Correlated over-cap rejection — consume the waiter (the reply failed
        // permanently; the browser gets a correlated 413, not a retry).
        retire(auth.waiter, "cancelled");

        return { kind: "rejected", reason: "too_large" };
      }
      retire(auth.waiter, "replied");

      return { kind: "ok" };
    },

    authenticateForFailure(correlation, nowMs) {
      const auth = authenticate(correlation, nowMs);
      if ("kind" in auth) {
        return auth;
      }
      // Correlation authenticated; the body was rejected (over-cap) or
      // malformed. Consume the waiter and report ok so the hub fails the
      // (correct) browser waiter with a correlated 413 / 502.
      retire(auth.waiter, "cancelled");

      return { kind: "ok" };
    },

    waiterState(workId) {
      return waiters.get(workId)?.state;
    },

    sweepExpired(nowMs) {
      const removedSessions: string[] = [];
      const failedWaiters: string[] = [];
      const droppedMutations: string[] = [];

      for (const [sessionId, session] of sessions) {
        // An open/last poll counts as liveness; only absence of any poll
        // beyond expiryMs makes a session expire (plan §3).
        if (nowMs - session.lastSeen <= config.expiryMs) {
          continue;
        }

        removedSessions.push(sessionId);
        sessions.delete(sessionId);
        expireSessionWaiters(sessionId, failedWaiters, droppedMutations);
      }

      return { removedSessions, failedWaiters, droppedMutations };
    },

    abort(workId) {
      const waiter = waiters.get(workId);
      if (waiter === undefined) {
        return { droppedMutation: false };
      }
      if (waiter.state === "queued") {
        // Undelivered: cancel it. A queued mutation is a dropped mutation.
        const dropped = isMutation(waiter.type);
        retire(waiter, "cancelled");

        return { droppedMutation: dropped };
      }
      // Already delivered (or terminal): the CLI DB owns its retry safety.
      return { droppedMutation: false };
    },
  };

  return relay;
}
