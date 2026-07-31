/**
 * v2 STATELESS RELAY HUB (plan §2/§3) — the async, process-wide layer over
 * the pure registry core (`createRelay`, registry.ts). The relay holds NO
 * database: it is a register / poll / reply message bus. This module adds the
 * Promise/timer plumbing the route handlers need on top of the pure core:
 *   - browser reads/mutations `request()` a work item and AWAIT the CLI reply
 *     (bounded by an operation deadline → 504 cli_timeout);
 *   - the agent `poll()`s (held ≤5s; an open poll counts as liveness) and
 *     `replyOk()`s, which resolves the awaiting browser Promise by work_id;
 *   - session metadata for the registry-only `GET /api/sessions`;
 *   - atomic expiry (10s absence-of-poll) that removes the session and fails
 *     its waiters — never store-and-forward.
 *
 * The three agent routes + the browser routes share ONE instance via an
 * explicit globalThis singleton (critique §14): Next.js may re-evaluate route
 * modules, but the singleton survives on the long-lived `next start` process.
 */

import { createRelay, type Relay, type WorkType } from "./registry";

export const RELAY_CONFIG = {
  expiryMs: 10_000,
  perSessionQueueMax: 64,
  globalQueueMax: 256,
  replyBudgetBytes: 1_048_576,
} as const;

/** Operation deadlines the browser routes bound each work item by (plan §3). */
export const DEADLINE_MS = {
  status: 5_000,
  runDetail: 5_000,
  mutation: 10_000,
  inventory: 30_000,
} as const;

/** Registry-only session summary (plan §3, critique §2). */
export interface SessionMeta {
  session_id: string;
  folder: string;
  canonical_folder: string;
  pid: number;
  start_identity: string;
  version: string;
  connected_at: number;
}

/** A CLI reply body: an HTTP status + the JSON payload the route returns. */
export interface CliReply {
  status: number;
  body: unknown;
}

interface Waiter {
  resolve: (reply: CliReply) => void;
  timer: ReturnType<typeof setTimeout>;
}

export interface Correlation {
  sessionId: string;
  epoch: number;
  token: string;
  workId: string;
}

/** Result of the register route mapping. */
export interface HubRegisterResult {
  connection_epoch: number;
  reply_budget_bytes: number;
  capability_token: string;
}

/** Background expiry sweep cadence (plan §3): a session with no poll for
 * `expiryMs` is removed even if nothing is currently accessing it. */
const SWEEP_INTERVAL_MS = 2_000;

class RelayHub {
  private readonly relay: Relay = createRelay(RELAY_CONFIG);
  private readonly meta = new Map<string, SessionMeta>();
  private readonly waiters = new Map<string, Waiter>();
  private readonly sweepTimer: ReturnType<typeof setInterval>;

  constructor() {
    // A background sweep removes expired sessions and fails their waiters even
    // when no request is currently touching the registry (plan §3, finding 2).
    // `unref` keeps the timer from holding the process open.
    this.sweepTimer = setInterval(() => this.sweep(), SWEEP_INTERVAL_MS);
    if (typeof this.sweepTimer.unref === "function") {
      this.sweepTimer.unref();
    }
  }

  register(body: SessionMeta): HubRegisterResult {
    const result = this.relay.register(body.session_id, { pid: body.pid, version: body.version }, Date.now());
    this.meta.set(body.session_id, { ...body, connected_at: Date.now() });

    // Re-registration invalidates the previous epoch: fail any browser waiter
    // whose work the new registration just expired (no store-and-forward).
    for (const workId of result.expiredWaiters) {
      this.deliverToWaiter(workId, { status: 404, body: { error: "session_gone" } });
    }

    return {
      connection_epoch: result.connection_epoch,
      reply_budget_bytes: result.reply_budget_bytes,
      capability_token: result.capability_token,
    };
  }

  /** Registry-only session list; every entry is connected by definition. */
  listSessions(): SessionMeta[] {
    this.sweep();

    return [...this.meta.values()];
  }

  hasSession(sessionId: string): boolean {
    this.sweep();

    return this.meta.has(sessionId);
  }

  /**
   * Enqueues a work item and awaits the CLI reply, bounded by `deadlineMs`.
   * An unknown session → 404 session_gone; a full queue → 503 capacity; a
   * silent CLI → 504 cli_timeout at the deadline (the undelivered work is
   * aborted so it is never delivered late).
   */
  request(
    sessionId: string,
    type: WorkType,
    payload: unknown,
    deadlineMs: number,
    signal?: AbortSignal,
  ): Promise<CliReply> {
    this.sweep();

    // A browser that already aborted must never enqueue work — a queued
    // mutation delivered after the abort is exactly the forbidden
    // store-and-forward (plan §3, finding 2).
    if (signal?.aborted) {
      return Promise.resolve({ status: 499, body: { error: "client_aborted" } });
    }

    const enqueue = this.relay.enqueue(sessionId, { type, payload }, Date.now());
    if (!enqueue.ok) {
      if (enqueue.reason === "unknown_session") {
        return Promise.resolve({ status: 404, body: { error: "session_gone" } });
      }

      return Promise.resolve({ status: 503, body: { error: "capacity" } });
    }

    const workId = enqueue.work_id as string;

    return new Promise<CliReply>((resolve) => {
      const timer = setTimeout(() => {
        this.relay.abort(workId);
        this.waiters.delete(workId);
        resolve({ status: 504, body: { error: "cli_timeout" } });
      }, deadlineMs);

      const settle = (reply: CliReply): void => {
        clearTimeout(timer);
        signal?.removeEventListener("abort", onAbort);
        this.waiters.delete(workId);
        resolve(reply);
      };

      // Browser cancellation cancels an UNDELIVERED operation (read OR
      // mutation). An already-delivered mutation is NOT dropped — its retry
      // safety is the CLI DB (plan §3). `relay.abort` reflects that.
      const onAbort = (): void => {
        this.relay.abort(workId);
        settle({ status: 499, body: { error: "client_aborted" } });
      };
      signal?.addEventListener("abort", onAbort, { once: true });

      this.waiters.set(workId, { resolve: settle, timer });
    });
  }

  /**
   * Long-polls for a work item on behalf of a CLI. Held ≤5s unless `wait`
   * is false (the zero-wait raw-protocol variant). Each internal poll renews
   * liveness. Returns the work item, an empty hold (→ 204), or a rejection
   * (unknown session / stale epoch / bad token → 4xx).
   */
  async poll(
    sessionId: string,
    epoch: number,
    token: string,
    wait: boolean,
  ): Promise<
    | { kind: "work"; work_id: string; type: WorkType; payload: unknown }
    | { kind: "empty" }
    | { kind: "rejected"; reason: string }
  > {
    const holdUntil = Date.now() + 5_000;
    for (;;) {
      const result = this.relay.poll(sessionId, epoch, token, Date.now());
      if (result.kind === "work") {
        return { kind: "work", work_id: result.item.work_id, type: result.item.type, payload: result.item.payload };
      }
      if (result.kind === "rejected") {
        return { kind: "rejected", reason: result.reason };
      }
      if (!wait || Date.now() >= holdUntil) {
        return { kind: "empty" };
      }
      await sleep(50);
    }
  }

  /**
   * Validates a CLI reply's correlation (epoch/token/work_id) via the pure
   * core and resolves the awaiting browser Promise. Returns the pure result so
   * the route can map a rejection to a status.
   */
  reply(correlation: Correlation, byteLength: number, replyBody: CliReply): { ok: boolean; reason?: string } {
    const result = this.relay.reply(
      { sessionId: correlation.sessionId, epoch: correlation.epoch, token: correlation.token, workId: correlation.workId },
      byteLength,
      Date.now(),
    );
    if (result.kind !== "ok") {
      return { ok: false, reason: result.reason };
    }

    this.deliverToWaiter(correlation.workId, replyBody);

    return { ok: true };
  }

  /**
   * Authenticates a failed reply's correlation (session/epoch/token +
   * DELIVERED waiter ownership) and, only if authenticated, fails the CORRECT
   * browser waiter with the given status (finding 1: never an unauthenticated
   * fail). Used for an over-cap (413) or malformed (502) reply. Returns whether
   * the correlation was authenticated.
   */
  private failAuthenticated(correlation: Correlation, status: number, error: string): { ok: boolean; reason?: string } {
    const result = this.relay.authenticateForFailure(
      { sessionId: correlation.sessionId, epoch: correlation.epoch, token: correlation.token, workId: correlation.workId },
      Date.now(),
    );
    if (result.kind !== "ok") {
      return { ok: false, reason: result.reason };
    }

    this.deliverToWaiter(correlation.workId, { status, body: { error } });

    return { ok: true };
  }

  /** Fails the correlated browser waiter with 413 for an over-cap reply. */
  failOverCap(correlation: Correlation): { ok: boolean; reason?: string } {
    return this.failAuthenticated(correlation, 413, "reply_too_large");
  }

  /** Fails the correlated browser waiter with 502 for a malformed reply. */
  failInvalid(correlation: Correlation): { ok: boolean; reason?: string } {
    return this.failAuthenticated(correlation, 502, "invalid_cli_reply");
  }

  private deliverToWaiter(workId: string, reply: CliReply): void {
    const waiter = this.waiters.get(workId);
    if (waiter !== undefined) {
      waiter.resolve(reply);
    }
  }

  /** Removes expired sessions and fails their waiters (atomic, plan §3). */
  private sweep(): void {
    const result = this.relay.sweepExpired(Date.now());
    for (const sessionId of result.removedSessions) {
      this.meta.delete(sessionId);
    }
    for (const workId of result.failedWaiters) {
      this.deliverToWaiter(workId, { status: 404, body: { error: "session_gone" } });
    }
  }
}

const sleep = (ms: number): Promise<void> => new Promise((resolve) => setTimeout(resolve, ms));

// ── The globalThis singleton (critique §14) ──

const GLOBAL_KEY = "__trueBddRelayHub__";

interface GlobalWithHub {
  [GLOBAL_KEY]?: RelayHub;
}

/** Returns the process-wide relay hub, creating it on first access. */
export function relayHub(): RelayHub {
  const store = globalThis as unknown as GlobalWithHub;
  if (store[GLOBAL_KEY] === undefined) {
    store[GLOBAL_KEY] = new RelayHub();
  }

  return store[GLOBAL_KEY];
}

export type { RelayHub };
