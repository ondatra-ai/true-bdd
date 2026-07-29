/**
 * The harness data store (plan §3.3): every SQL operation the agent and
 * browser routes need, expressed as pure functions over a better-sqlite3
 * connection passed in by the caller. Keeping the connection a parameter
 * (rather than reaching for a module singleton) is what lets the vitest
 * suites drive the REAL state machine against a temp-file DB — including
 * two independent connections racing a dispatch — without a running Next
 * server (plan §4.6, "thin route → handler pattern"). The route handlers
 * stay thin: parse → call one of these with `db()` → map the typed result
 * to a NextResponse.
 *
 * Run state machine (plan §3.3):
 *   queued → claimed → running ⇄ prompt_published →
 *   answer_accepted → answer_consumed → terminal{ok|converged|not_fixed|
 *   user_exit|max_attempts|interrupted|abandoned|error}
 *
 * - claimed: the remote posted its first event for a queued run.
 * - running / prompt_published: proof the child actually executes (it
 *   acquired the host flock — plan §3.7); the FIRST such transition
 *   ABANDONS older non-terminal runs of the same folder.
 * - answer_accepted: the browser's answer is durably recorded (first
 *   wins). answer_consumed: the remote confirmed it wrote the answer to
 *   the child's stdin (answers_consumed watermark).
 * - terminal: applied exactly once (guarded on state != 'terminal'); the
 *   first terminal transition of any folder run flags want_inventory for
 *   ALL sessions of that folder (folder-wide auto-refresh).
 */

import { createHash, randomUUID } from "node:crypto";

import type Database from "better-sqlite3";

import { retentionConfig } from "./retention";

/** A session flips unreachable after this long without an agent poll. */
export const REACHABILITY_THRESHOLD_MS = 10_000;

/** Server-side answer size caps (plan §3.2/§3.3, mirroring the remote). */
const MAX_CLARIFY_BYTES = 4096;
const MAX_FREETEXT_BYTES = 64 * 1024;

/** The choice-answer enum: apply/refine/exit or their 1/2/3 equivalents. */
const CHOICE_VALUES = new Set(["apply", "refine", "exit", "1", "2", "3"]);

export type Reachability = "connected" | "unreachable";
export type RunState =
  | "queued"
  | "claimed"
  | "running"
  | "prompt_published"
  | "answer_accepted"
  | "answer_consumed"
  | "terminal";

type Db = Database.Database;

interface SessionRow {
  id: string;
  incarnation_id: string;
  folder: string;
  canonical_folder: string;
  pid: number;
  version: string | null;
  created_at: number;
  last_poll_at: number;
  scan_epoch: number;
  want_inventory: number;
}

interface RunRow {
  id: string;
  session_id: string;
  command: string;
  story_id: string | null;
  fix: number;
  state: string;
  outcome: string | null;
  error_detail: string | null;
  client_token: string;
  answer_seq: number;
  answers_consumed: number;
  applied_through: number;
  engine_outcome: string | null;
  finalization_ok: number | null;
  exit_code: number | null;
  signal: string | null;
  created_at: number;
  updated_at: number;
}

// ── Wire shapes returned to callers ──

export interface SessionSummary {
  id: string;
  folder: string;
  pid: number;
  reachability: Reachability;
  active_run_id: string | null;
  inventory_generation: number;
  /** Wall-clock ms of the last promoted inventory, null until one exists. */
  inventory_updated_at: number | null;
}

/** The full remote-synthesized terminal envelope (plan §3.2 / finding 7). */
export interface TerminalEnvelope {
  /** The final classification — this is what the run-outcome badge shows. */
  classification: string | null;
  /** The engine's own stop-reason outcome, before classification. */
  engine_outcome: string | null;
  /** Whether the post-walk finalization (e.g. story write) succeeded. */
  finalization_ok: boolean | null;
  /** The child process exit code, when it exited normally. */
  exit_code: number | null;
  /** The signal that killed the child, when it died on one. */
  signal: string | null;
}

export interface RunSummary {
  id: string;
  command: string;
  story_id: string | null;
  fix: boolean;
  state: string;
  outcome: string | null;
  error_detail: string | null;
  /** The full terminal envelope (finding 7); fields null until terminal. */
  envelope: TerminalEnvelope;
  /** Wall-clock ms the run was created (dispatched). */
  created_at: number;
  /** Wall-clock ms of the run's last state change (last activity). */
  updated_at: number;
}

export interface PendingPrompt {
  prompt_id: string;
  kind: string;
  payload: unknown;
}

export interface RunEvent {
  seq: number;
  type: string;
  [key: string]: unknown;
}

/** Monotonic wall-clock milliseconds for created/updated stamps. */
export function now(): number {
  return Date.now();
}

// ── Reachability ──

function reachabilityOf(session: SessionRow): Reachability {
  return now() - session.last_poll_at <= REACHABILITY_THRESHOLD_MS ? "connected" : "unreachable";
}

// ── Row lookups ──

function sessionRow(db: Db, id: string): SessionRow | undefined {
  return db.prepare("SELECT * FROM sessions WHERE id = ?").get(id) as SessionRow | undefined;
}

function runRow(db: Db, id: string): RunRow | undefined {
  return db.prepare("SELECT * FROM runs WHERE id = ?").get(id) as RunRow | undefined;
}

function activeRunId(db: Db, sessionId: string): string | null {
  const row = db
    .prepare("SELECT id FROM runs WHERE session_id = ? AND state != 'terminal' ORDER BY created_at DESC LIMIT 1")
    .get(sessionId) as { id: string } | undefined;

  return row?.id ?? null;
}

function inventoryForFolder(db: Db, folder: string): { generation: number; updated_at: number | null } {
  const row = db
    .prepare("SELECT generation, updated_at FROM inventory WHERE canonical_folder = ?")
    .get(folder) as { generation: number; updated_at: number } | undefined;

  return { generation: row?.generation ?? 0, updated_at: row?.updated_at ?? null };
}

function summaryOf(db: Db, session: SessionRow): SessionSummary {
  const inventory = inventoryForFolder(db, session.canonical_folder);

  return {
    id: session.id,
    folder: session.canonical_folder,
    pid: session.pid,
    reachability: reachabilityOf(session),
    active_run_id: activeRunId(db, session.id),
    inventory_generation: inventory.generation,
    inventory_updated_at: inventory.updated_at,
  };
}

// ── Folder-global scan epochs (plan §3.3 handoff) ──
//
// The epoch that orders inventory scans is a FOLDER-level monotonic
// counter, not a per-session one: two sessions on the same folder draw
// from the same sequence, so a slow scan carrying an older epoch is
// correctly rejected against a fresher one uploaded by EITHER session.
// An epoch is issued only when a scan is about to happen — on register
// and on a want_inventory poll — "before the remote scans".

function issueEpoch(db: Db, folder: string, sessionId: string): number {
  db.prepare(
    `INSERT INTO folder_epochs (canonical_folder, issued_epoch) VALUES (?, 1)
     ON CONFLICT(canonical_folder) DO UPDATE SET issued_epoch = issued_epoch + 1`,
  ).run(folder);
  const row = db
    .prepare("SELECT issued_epoch FROM folder_epochs WHERE canonical_folder = ?")
    .get(folder) as { issued_epoch: number };

  // Persist the ticket so an inventory upload can be proven to carry an
  // epoch THIS session was granted for THIS folder (plan §3.3 / finding 2).
  db.prepare(
    "INSERT OR REPLACE INTO inventory_tickets (canonical_folder, session_id, scan_epoch, issued_at) VALUES (?, ?, ?, ?)",
  ).run(folder, sessionId, row.issued_epoch, now());

  return row.issued_epoch;
}

// ── Agent: register ──

export function registerSession(
  db: Db,
  input: {
    incarnation_id: string;
    folder: string;
    canonical_folder: string;
    pid: number;
    version: string;
  },
): { session_id: string; scan_epoch: number } {
  const timestamp = now();
  const existing = db
    .prepare("SELECT * FROM sessions WHERE incarnation_id = ?")
    .get(input.incarnation_id) as SessionRow | undefined;

  if (existing) {
    // Same incarnation ⇒ same session (plan §3.3), issuing a fresh ticket.
    const epoch = issueEpoch(db, input.canonical_folder, existing.id);
    db.prepare(
      `UPDATE sessions SET folder = ?, canonical_folder = ?, pid = ?, version = ?,
       last_poll_at = ?, scan_epoch = ? WHERE id = ?`,
    ).run(input.folder, input.canonical_folder, input.pid, input.version, timestamp, epoch, existing.id);

    return { session_id: existing.id, scan_epoch: epoch };
  }

  const id = randomUUID();
  const epoch = issueEpoch(db, input.canonical_folder, id);
  db.prepare(
    `INSERT INTO sessions
     (id, incarnation_id, folder, canonical_folder, pid, version, created_at, last_poll_at, scan_epoch, want_inventory)
     VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 0)`,
  ).run(id, input.incarnation_id, input.folder, input.canonical_folder, input.pid, input.version, timestamp, timestamp, epoch);

  return { session_id: id, scan_epoch: epoch };
}

// ── Agent: poll (heartbeat) ──

export interface PollResult {
  run?: { run_id: string; command: string; story_id?: string; fix: boolean };
  answer?: { prompt_id: string; value: string; answer_seq: number };
  want_inventory: boolean;
  scan_epoch: number;
}

export function pollSession(db: Db, sessionId: string, activeRun: string | undefined): PollResult | undefined {
  const session = sessionRow(db, sessionId);
  if (!session) {
    return undefined;
  }

  // Deliver (and consume) a want_inventory request only to an IDLE remote:
  // the remote re-scans only when it has no active run, so clearing the
  // flag while it is busy would silently drop the request. A fresh scan
  // epoch is issued ONLY when we actually tell the remote to re-scan —
  // "before the remote scans"; an ordinary heartbeat returns 0 (omitted),
  // leaving the remote's monotonic epoch untouched.
  const deliverInventory = session.want_inventory === 1 && activeRun === undefined;
  const epoch = deliverInventory ? issueEpoch(db, session.canonical_folder, session.id) : 0;
  if (deliverInventory) {
    db.prepare("UPDATE sessions SET last_poll_at = ?, want_inventory = 0, scan_epoch = ? WHERE id = ?").run(now(), epoch, sessionId);
  } else {
    db.prepare("UPDATE sessions SET last_poll_at = ? WHERE id = ?").run(now(), sessionId);
  }

  const queued = db
    .prepare("SELECT id, command, story_id, fix FROM runs WHERE session_id = ? AND state = 'queued' ORDER BY created_at LIMIT 1")
    .get(sessionId) as { id: string; command: string; story_id: string | null; fix: number } | undefined;

  const result: PollResult = { want_inventory: deliverInventory, scan_epoch: epoch };

  if (queued) {
    result.run = {
      run_id: queued.id,
      command: queued.command,
      story_id: queued.story_id ?? undefined,
      fix: queued.fix === 1,
    };
  }

  if (activeRun) {
    const answer = pendingAnswer(db, activeRun);
    if (answer) {
      result.answer = answer;
    }
  }

  return result;
}

/**
 * The highest-seq answer the remote has NOT yet consumed (answer_seq
 * beyond the run's answers_consumed watermark). Once consumed, an answer
 * is never re-delivered — redelivery on the remote is acknowledgement
 * only (plan §3.3 handoff b).
 */
function pendingAnswer(db: Db, runId: string): { prompt_id: string; value: string; answer_seq: number } | undefined {
  const run = runRow(db, runId);
  if (!run || run.state === "terminal") {
    return undefined;
  }

  return db
    .prepare(
      "SELECT prompt_id, value, answer_seq FROM answers WHERE run_id = ? AND answer_seq > ? ORDER BY answer_seq DESC LIMIT 1",
    )
    .get(runId, run.answers_consumed) as { prompt_id: string; value: string; answer_seq: number } | undefined;
}

// ── Agent: events (the remote-owned (run_id, seq) stream) ──

/** The event types the remote may post (plan §3.2/§3.6 / findings 1, 5). */
const EVENT_TYPES = new Set(["output", "prompt", "terminal", "gap", "lock_acquired"]);

/** Server-side prompt/answer payload byte caps (plan §3.2, NICE-2). */
const MAX_PROMPT_PAYLOAD_BYTES = 64 * 1024;

/**
 * Appends a contiguous batch of remote-owned run events (plan §3.3 /
 * finding 1). The pipeline is effect-idempotent:
 *
 * - Structurally invalid batches (non-positive/out-of-order seq, unknown
 *   type, oversized payload) are rejected WITHOUT mutating state.
 * - An event at an already-occupied seq is a no-op when its canonical
 *   payload matches (a genuine redelivery) and is REJECTED (skipped, never
 *   re-applied) when it conflicts — a forged terminal at an output's seq
 *   can no longer terminalize the run.
 * - State transitions run exactly once, in contiguous seq order, driven by
 *   an `applied_through` cursor, so a redelivered prompt can never regress
 *   answer_accepted / answer_consumed back to prompt_published.
 * - Range tombstone coverage is stored separately (gap_coverage), so a
 *   tombstone colliding with a committed seq still advances the ack.
 * - A terminal run records a compact receipt that survives run pruning, so
 *   a late terminal replay of a pruned run is still acknowledged.
 */
export function appendEvents(
  db: Db,
  runId: string,
  events: RunEvent[],
  answersConsumed = 0,
): { acked_through: number; answers_consumed_through: number; rejected?: boolean } {
  const run = runRow(db, runId);
  if (!run) {
    // Unknown run: it may have been pruned after going terminal. A retained
    // terminal receipt still acknowledges a late replay so the remote's
    // spool can finally delete (finding 1).
    const receipt = terminalReceipt(db, runId);
    if (receipt) {
      return {
        acked_through: receipt.acked_through,
        answers_consumed_through: receipt.answers_consumed_through,
      };
    }

    return { acked_through: 0, answers_consumed_through: 0 };
  }

  if (!validEventBatch(events)) {
    return {
      acked_through: contiguousAck(db, runId),
      answers_consumed_through: run.answers_consumed,
      rejected: true,
    };
  }

  const apply = db.transaction((batch: RunEvent[]) => {
    // Claim: the first event on a queued run takes ownership. Flock-proof
    // abandonment is deferred to the explicit `lock_acquired` event so a
    // folder_locked run — which never emits it — abandons nothing.
    if (batch.length > 0) {
      db.prepare("UPDATE runs SET state = 'claimed', updated_at = ? WHERE id = ? AND state = 'queued'").run(now(), runId);
    }

    for (const event of batch) {
      storeEvent(db, runId, event);
    }

    advanceTransitions(db, runId);
    consumeAnswers(db, runId, answersConsumed);
    enforceByteBudget(db, runId);
    recordTerminalReceipt(db, runId);
  });
  apply(events);

  const after = runRow(db, runId);

  return {
    acked_through: contiguousAck(db, runId),
    answers_consumed_through: after?.answers_consumed ?? 0,
  };
}

/**
 * Validates a batch's structure BEFORE any state access (finding 1): every
 * seq is a positive integer, the batch is strictly ascending, every type is
 * known, and prompt/gap payloads are within bounds. A structurally invalid
 * batch is rejected whole so a malformed post never half-applies.
 */
function validEventBatch(events: RunEvent[]): boolean {
  let previous = 0;
  for (const event of events) {
    if (!Number.isInteger(event.seq) || event.seq <= 0 || event.seq <= previous) {
      return false;
    }
    previous = event.seq;

    if (typeof event.type !== "string" || !EVENT_TYPES.has(event.type)) {
      return false;
    }

    if (event.type === "gap") {
      const through = event.through_seq;
      if (typeof through !== "number" || !Number.isInteger(through) || through < event.seq) {
        return false;
      }
    }

    if (payloadBytes(event) > MAX_PROMPT_PAYLOAD_BYTES) {
      return false;
    }
  }

  return true;
}

/** The serialized byte size of an event's non-key payload fields. */
function payloadBytes(event: RunEvent): number {
  const rest: Record<string, unknown> = {};
  for (const key of Object.keys(event)) {
    if (key !== "seq" && key !== "type") {
      rest[key] = event[key];
    }
  }

  return Buffer.byteLength(JSON.stringify(rest), "utf8");
}

/**
 * Stores one event, deduping against any already-committed row at the same
 * seq. An exact-payload redelivery is a no-op; a CONFLICTING payload at an
 * occupied seq is skipped entirely (never inserted, never transitioned). A
 * gap also records its coverage interval separately so a collision with a
 * committed event row cannot strand the contiguous ack.
 */
function storeEvent(db: Db, runId: string, event: RunEvent): void {
  const { seq, type, ...rest } = event;
  const data = JSON.stringify(rest);

  const existing = db
    .prepare("SELECT type, data FROM events WHERE run_id = ? AND seq = ?")
    .get(runId, seq) as { type: string; data: string } | undefined;

  const isGap = type === "gap";
  const gapThrough = isGap && typeof event.through_seq === "number" ? event.through_seq : seq;

  if (existing) {
    // Occupied seq: dedup an exact redelivery, reject a conflicting payload.
    // Either way the event row is untouched. A gap still merges its range,
    // so a tombstone colliding with a committed event still covers 3..N.
    if (isGap) {
      recordGapCoverage(db, runId, seq, gapThrough);
    }

    return;
  }

  db.prepare("INSERT INTO events (run_id, seq, type, data, created_at) VALUES (?, ?, ?, ?, ?)").run(
    runId,
    seq,
    type,
    data,
    now(),
  );

  if (isGap) {
    recordGapCoverage(db, runId, seq, gapThrough);
  }
}

/** Records (merging) a range tombstone's inclusive coverage [from, to]. */
function recordGapCoverage(db: Db, runId: string, fromSeq: number, toSeq: number): void {
  if (toSeq < fromSeq) {
    return;
  }

  db.prepare(
    `INSERT INTO gap_coverage (run_id, from_seq, to_seq) VALUES (?, ?, ?)
     ON CONFLICT(run_id, from_seq) DO UPDATE SET to_seq = MAX(to_seq, excluded.to_seq)`,
  ).run(runId, fromSeq, toSeq);
}

/**
 * Advances the `applied_through` cursor across the newly-contiguous span
 * and applies each event row's state transition exactly once, in seq order
 * (finding 1). Only event rows carry transitions; gap coverage advances the
 * cursor without one. Because transitions run once and in order, a
 * redelivered or out-of-order event never re-drives the state machine.
 */
function advanceTransitions(db: Db, runId: string): void {
  const run = runRow(db, runId);
  if (!run) {
    return;
  }

  const watermark = contiguousAck(db, runId);
  if (watermark <= run.applied_through) {
    return;
  }

  const rows = db
    .prepare("SELECT seq, type, data FROM events WHERE run_id = ? AND seq > ? AND seq <= ? ORDER BY seq")
    .all(runId, run.applied_through, watermark) as { seq: number; type: string; data: string }[];

  for (const row of rows) {
    applyTransition(db, runId, { seq: row.seq, type: row.type, ...(JSON.parse(row.data) as object) });
  }

  db.prepare("UPDATE runs SET applied_through = ? WHERE id = ?").run(watermark, runId);
}

function applyTransition(db: Db, runId: string, event: RunEvent): void {
  const timestamp = now();

  // The explicit flock-acquired proof (plan §3.7 / finding 5): the ONLY
  // event that abandons older folder runs. Output/prompt move presentation
  // state but never stand in for lock proof, so a run that fails argument
  // construction or spawn after taking the lock still abandons a stale
  // older run of the same folder.
  if (event.type === "lock_acquired") {
    db.prepare("UPDATE runs SET state = 'running', updated_at = ? WHERE id = ? AND state IN ('queued', 'claimed')").run(
      timestamp,
      runId,
    );
    abandonOlderFolderRuns(db, runId);

    return;
  }

  if (event.type === "output") {
    db.prepare(
      "UPDATE runs SET state = 'running', updated_at = ? WHERE id = ? AND state IN ('queued', 'claimed', 'answer_accepted', 'answer_consumed')",
    ).run(timestamp, runId);

    return;
  }

  if (event.type === "prompt") {
    db.prepare(
      "INSERT OR IGNORE INTO prompts (run_id, prompt_id, kind, payload, seq, created_at) VALUES (?, ?, ?, ?, ?, ?)",
    ).run(runId, String(event.prompt_id), String(event.kind), JSON.stringify(event.payload ?? null), event.seq, timestamp);

    db.prepare(
      "UPDATE runs SET state = 'prompt_published', updated_at = ? WHERE id = ? AND state IN ('queued', 'claimed', 'running', 'answer_accepted', 'answer_consumed')",
    ).run(timestamp, runId);

    return;
  }

  if (event.type === "terminal") {
    const res = db
      .prepare(
        `UPDATE runs SET state = 'terminal', outcome = ?, error_detail = ?, engine_outcome = ?,
         finalization_ok = ?, exit_code = ?, signal = ?, updated_at = ? WHERE id = ? AND state != 'terminal'`,
      )
      .run(
        String(event.outcome ?? "error"),
        (event.error_detail as string) ?? null,
        typeof event.engine_outcome === "string" ? event.engine_outcome : null,
        finalizationFlag(event.finalization_ok),
        typeof event.exit_code === "number" ? event.exit_code : null,
        typeof event.signal === "string" ? event.signal : null,
        timestamp,
        runId,
      );
    if (res.changes > 0) {
      autoRefreshFolder(db, runId);
    }
  }

  // "gap" carries no transition; its coverage advances the cursor only.
}

/** Maps an optional terminal `finalization_ok` boolean to a 0/1/null flag. */
function finalizationFlag(value: unknown): number | null {
  if (value === true) {
    return 1;
  }
  if (value === false) {
    return 0;
  }

  return null;
}

/**
 * Records a compact terminal receipt once a run is terminal (finding 1).
 * The receipt outlives run pruning, so a late replay of a terminal whose
 * commit response was lost is still acknowledged and the remote's spool can
 * delete rather than replay forever.
 */
function recordTerminalReceipt(db: Db, runId: string): void {
  const run = runRow(db, runId);
  if (!run || run.state !== "terminal") {
    return;
  }

  db.prepare(
    `INSERT INTO terminal_receipts (run_id, acked_through, answers_consumed_through, updated_at)
     VALUES (?, ?, ?, ?)
     ON CONFLICT(run_id) DO UPDATE SET acked_through = MAX(acked_through, excluded.acked_through),
       answers_consumed_through = MAX(answers_consumed_through, excluded.answers_consumed_through),
       updated_at = excluded.updated_at`,
  ).run(runId, contiguousAck(db, runId), run.answers_consumed, now());
}

/** The retained terminal receipt for a (possibly pruned) run, if any. */
function terminalReceipt(
  db: Db,
  runId: string,
): { acked_through: number; answers_consumed_through: number } | undefined {
  return db
    .prepare("SELECT acked_through, answers_consumed_through FROM terminal_receipts WHERE run_id = ?")
    .get(runId) as { acked_through: number; answers_consumed_through: number } | undefined;
}

/**
 * Advances answer_accepted → answer_consumed once the remote confirms it
 * has written every accepted answer to the child's stdin. Idempotent:
 * the watermark only ever moves forward.
 */
function consumeAnswers(db: Db, runId: string, answersConsumed: number): void {
  if (answersConsumed <= 0) {
    return;
  }

  const run = runRow(db, runId);
  if (!run || answersConsumed <= run.answers_consumed) {
    return;
  }

  db.prepare("UPDATE runs SET answers_consumed = ?, updated_at = ? WHERE id = ?").run(answersConsumed, now(), runId);
  if (run.state === "answer_accepted" && answersConsumed >= run.answer_seq) {
    db.prepare("UPDATE runs SET state = 'answer_consumed', updated_at = ? WHERE id = ? AND state = 'answer_accepted'").run(
      now(),
      runId,
    );
  }
}

/**
 * Flock-proof abandonment (plan §3.3/§3.7): when a run first proves it is
 * executing (its first output/prompt event — i.e. it acquired the host
 * folder flock), every OLDER non-terminal run of the SAME folder becomes
 * `abandoned`. A later run taking the folder is the ONLY proof that
 * resolves an earlier run whose terminal event was lost (e.g. its remote
 * was killed at a prompt — A7).
 */
function abandonOlderFolderRuns(db: Db, runId: string): void {
  const self = db.prepare("SELECT rowid AS rid, session_id FROM runs WHERE id = ?").get(runId) as
    | { rid: number; session_id: string }
    | undefined;
  if (!self) {
    return;
  }

  const session = sessionRow(db, self.session_id);
  if (!session) {
    return;
  }

  // "Older" is INSERTION order via the monotonic rowid — robust even when
  // sibling runs share a created_at millisecond. `rowid < self` also
  // excludes the claiming run itself and any strictly-newer queued run.
  const victims = db
    .prepare(
      `SELECT r.id AS id FROM runs r JOIN sessions s ON s.id = r.session_id
       WHERE s.canonical_folder = ? AND r.state != 'terminal' AND r.rowid < ?`,
    )
    .all(session.canonical_folder, self.rid) as { id: string }[];

  for (const victim of victims) {
    const res = db
      .prepare("UPDATE runs SET state = 'terminal', outcome = 'abandoned', updated_at = ? WHERE id = ? AND state != 'terminal'")
      .run(now(), victim.id);
    if (res.changes > 0) {
      autoRefreshFolder(db, victim.id);
    }
  }
}

/** Flags want_inventory for every session on the run's folder (plan §3.3). */
function autoRefreshFolder(db: Db, runId: string): void {
  const run = runRow(db, runId);
  if (!run) {
    return;
  }

  const session = sessionRow(db, run.session_id);
  if (!session) {
    return;
  }

  db.prepare("UPDATE sessions SET want_inventory = 1 WHERE canonical_folder = ?").run(session.canonical_folder);
}

/**
 * The contiguous acked watermark, computed over the union of stored event
 * rows and the separately-recorded gap-coverage intervals (plan §3.6
 * handoff a / finding 1). A `gap` tombstone covers the inclusive range
 * [seq, through_seq]; because its coverage is recorded independently of the
 * events primary key, a tombstone whose first seq collides with an
 * already-committed event still advances the watermark across the
 * collapsed span and can reach the terminal seq.
 */
function contiguousAck(db: Db, runId: string): number {
  const intervals = coverageIntervals(db, runId);
  intervals.sort((a, b) => a[0] - b[0]);

  let watermark = 0;
  for (const [start, end] of intervals) {
    if (start > watermark + 1) {
      break;
    }
    watermark = Math.max(watermark, end);
  }

  return watermark;
}

/** The [start, end] coverage intervals from events + gap_coverage. */
function coverageIntervals(db: Db, runId: string): [number, number][] {
  const eventRows = db.prepare("SELECT seq, type, data FROM events WHERE run_id = ?").all(runId) as {
    seq: number;
    type: string;
    data: string;
  }[];
  const gapRows = db.prepare("SELECT from_seq, to_seq FROM gap_coverage WHERE run_id = ?").all(runId) as {
    from_seq: number;
    to_seq: number;
  }[];

  const intervals: [number, number][] = eventRows.map((row) => [row.seq, spanEnd(row)]);
  for (const gap of gapRows) {
    intervals.push([gap.from_seq, gap.to_seq]);
  }

  return intervals;
}

/** The highest seq an event row covers: through_seq for a gap, else seq. */
function spanEnd(row: { seq: number; type: string; data: string }): number {
  if (row.type === "gap") {
    const parsed = JSON.parse(row.data) as { through_seq?: number };
    if (typeof parsed.through_seq === "number" && parsed.through_seq > row.seq) {
      return parsed.through_seq;
    }
  }

  return row.seq;
}

// ── Retention: per-run byte budget (plan §3.6, server side) ──
//
// A long run's stored event rows are bounded to a byte budget by keeping
// a HEAD and a rolling TAIL and collapsing the middle into ONE synthesized
// `gap` marker event that covers the deleted seq span. Only rows the
// remote will never re-send (seq ≤ the contiguous acked watermark) are
// collapsed, so INSERT OR IGNORE redelivery can never resurrect a middle
// row and un-collapse the range.

export function enforceByteBudget(db: Db, runId: string, override?: Partial<{ byteBudget: number; head: number; tail: number }>): void {
  const cfg = { ...retentionConfig(), ...override };
  const rows = db.prepare("SELECT seq, type, data FROM events WHERE run_id = ? ORDER BY seq").all(runId) as {
    seq: number;
    type: string;
    data: string;
  }[];

  if (rows.length <= cfg.head + cfg.tail + 1 || totalBytes(rows) <= cfg.byteBudget) {
    return;
  }

  const acked = contiguousAck(db, runId);
  const middle = rows.slice(cfg.head, rows.length - cfg.tail).filter((row) => spanEnd(row) <= acked);
  if (middle.length < 2) {
    return;
  }

  const firstSeq = middle[0].seq;
  const lastSeq = spanEnd(middle[middle.length - 1]);
  const dropped = totalBytes(middle);

  db.prepare("DELETE FROM events WHERE run_id = ? AND seq >= ? AND seq <= ?").run(runId, firstSeq, lastSeq);
  db.prepare("INSERT OR REPLACE INTO events (run_id, seq, type, data, created_at) VALUES (?, ?, 'gap', ?, ?)").run(
    runId,
    firstSeq,
    JSON.stringify({ through_seq: lastSeq, dropped_bytes: dropped, retention: true }),
    now(),
  );
}

function totalBytes(rows: { data: string }[]): number {
  let total = 0;
  for (const row of rows) {
    total += Buffer.byteLength(row.data, "utf8");
    const parsed = safeParse(row.data);
    if (parsed && typeof parsed.dropped_bytes === "number") {
      total += parsed.dropped_bytes;
    }
  }

  return total;
}

function safeParse(data: string): { dropped_bytes?: number } | undefined {
  try {
    return JSON.parse(data) as { dropped_bytes?: number };
  } catch {
    return undefined;
  }
}

// ── Retention: prune terminal runs beyond last N/folder (plan §3.6) ──

export function pruneTerminalRuns(db: Db, keepPerFolder = retentionConfig().keepPerFolder): void {
  const folders = db.prepare("SELECT DISTINCT canonical_folder FROM sessions").all() as { canonical_folder: string }[];

  const pruneFolder = db.transaction((folder: string) => {
    const doomed = db
      .prepare(
        `SELECT r.id AS id FROM runs r JOIN sessions s ON s.id = r.session_id
         WHERE s.canonical_folder = ? AND r.state = 'terminal'
         ORDER BY r.created_at DESC, r.rowid DESC LIMIT -1 OFFSET ?`,
      )
      .all(folder, keepPerFolder) as { id: string }[];

    for (const run of doomed) {
      db.prepare("DELETE FROM events WHERE run_id = ?").run(run.id);
      db.prepare("DELETE FROM prompts WHERE run_id = ?").run(run.id);
      db.prepare("DELETE FROM answers WHERE run_id = ?").run(run.id);
      db.prepare("DELETE FROM gap_coverage WHERE run_id = ?").run(run.id);
      // terminal_receipts are DELIBERATELY retained (compact, finding 1):
      // a late terminal replay of a pruned run must still be acknowledged.
      db.prepare("DELETE FROM runs WHERE id = ?").run(run.id);
    }
  });

  for (const folder of folders) {
    pruneFolder(folder.canonical_folder);
  }
}

/** A session with no runs is pruned after this long without a poll. */
export const SESSION_PRUNE_AFTER_MS = 24 * 60 * 60 * 1000;

/**
 * Prunes long-unreachable sessions that have NO retained runs (NICE-2):
 * a remote that connected once, uploaded nothing durable, and vanished
 * would otherwise accumulate a session row (and its issued tickets)
 * forever. A session with any run — even a pruned-to-receipt terminal one
 * whose run row survives — is kept; only truly orphaned sessions are
 * removed. Folder-scoped inventory is unaffected.
 */
export function pruneUnreachableSessions(db: Db, olderThanMs = SESSION_PRUNE_AFTER_MS): void {
  const cutoff = now() - olderThanMs;
  const doomed = db
    .prepare(
      `SELECT s.id AS id, s.canonical_folder AS folder FROM sessions s
       WHERE s.last_poll_at < ?
         AND NOT EXISTS (SELECT 1 FROM runs r WHERE r.session_id = s.id)`,
    )
    .all(cutoff) as { id: string; folder: string }[];

  for (const session of doomed) {
    db.prepare("DELETE FROM inventory_tickets WHERE session_id = ?").run(session.id);
    db.prepare("DELETE FROM sessions WHERE id = ?").run(session.id);
  }
}

// ── Agent: inventory ──

export type InventoryResult = { generation: number } | { rejected: "unknown_ticket" | "mismatched_retry" };

/**
 * Records an inventory upload (plan §3.3 / finding 2). Ordering and
 * idempotency are enforced through server-issued tickets and an upload
 * ledger, not the client's word:
 *
 * - EXACT retry (same folder+token, same epoch+payload) returns the
 *   RECORDED generation, so a lost response never loses the assignment;
 * - a MISMATCHED retry (same token, different epoch/payload) is rejected;
 * - a NEW token must carry a ticket the server issued to THIS session for
 *   THIS folder, and must be strictly newer than the promoted epoch to
 *   promote — a delayed stale scan or a forged epoch never demotes.
 */
export function upsertInventory(
  db: Db,
  input: {
    session_id: string;
    canonical_folder: string;
    scan_epoch: number;
    snapshot: unknown;
    client_token: string;
  },
): InventoryResult {
  const snapshot = JSON.stringify(input.snapshot);
  const payloadHash = createHash("sha256").update(snapshot).digest("hex");
  const timestamp = now();

  const record = db.transaction((): InventoryResult => {
    const ledger = db
      .prepare("SELECT scan_epoch, generation, payload_hash FROM inventory_uploads WHERE canonical_folder = ? AND client_token = ?")
      .get(input.canonical_folder, input.client_token) as
      | { scan_epoch: number; generation: number; payload_hash: string }
      | undefined;

    if (ledger) {
      // Known token: only an exact retry (same epoch + payload) replays the
      // recorded generation. A mismatched retry is a protocol error.
      if (ledger.scan_epoch === input.scan_epoch && ledger.payload_hash === payloadHash) {
        return { generation: ledger.generation };
      }

      return { rejected: "mismatched_retry" };
    }

    // New token: prove the (session, folder, epoch) ticket was issued.
    const ticket = db
      .prepare("SELECT 1 FROM inventory_tickets WHERE canonical_folder = ? AND session_id = ? AND scan_epoch = ?")
      .get(input.canonical_folder, input.session_id, input.scan_epoch);
    if (!ticket) {
      return { rejected: "unknown_ticket" };
    }

    const existing = db
      .prepare("SELECT generation, scan_epoch FROM inventory WHERE canonical_folder = ?")
      .get(input.canonical_folder) as { generation: number; scan_epoch: number } | undefined;

    if (!existing) {
      db.prepare(
        "INSERT INTO inventory (canonical_folder, generation, scan_epoch, snapshot, client_token, updated_at) VALUES (?, 1, ?, ?, ?, ?)",
      ).run(input.canonical_folder, input.scan_epoch, snapshot, input.client_token, timestamp);
      recordUpload(db, input.canonical_folder, input.client_token, input.scan_epoch, 1, payloadHash, timestamp);

      return { generation: 1 };
    }

    // A new token must be strictly newer than the promoted epoch to
    // promote; an equal or older epoch keeps the current generation but is
    // still ledgered so its own retry is idempotent.
    if (input.scan_epoch <= existing.scan_epoch) {
      recordUpload(db, input.canonical_folder, input.client_token, input.scan_epoch, existing.generation, payloadHash, timestamp);

      return { generation: existing.generation };
    }

    const generation = existing.generation + 1;
    db.prepare(
      "UPDATE inventory SET generation = ?, scan_epoch = ?, snapshot = ?, client_token = ?, updated_at = ? WHERE canonical_folder = ?",
    ).run(generation, input.scan_epoch, snapshot, input.client_token, timestamp, input.canonical_folder);
    recordUpload(db, input.canonical_folder, input.client_token, input.scan_epoch, generation, payloadHash, timestamp);

    return { generation };
  });

  return record.immediate();
}

/** Writes the (folder, token) upload ledger row. */
function recordUpload(
  db: Db,
  folder: string,
  token: string,
  epoch: number,
  generation: number,
  payloadHash: string,
  timestamp: number,
): void {
  db.prepare(
    `INSERT OR REPLACE INTO inventory_uploads (canonical_folder, client_token, scan_epoch, generation, payload_hash, updated_at)
     VALUES (?, ?, ?, ?, ?, ?)`,
  ).run(folder, token, epoch, generation, payloadHash, timestamp);
}

// ── Browser: sessions ──

export function listSessions(db: Db): SessionSummary[] {
  const rows = db.prepare("SELECT * FROM sessions ORDER BY created_at DESC").all() as SessionRow[];

  return rows.map((session) => summaryOf(db, session));
}

export interface SessionDetail extends SessionSummary {
  runs: RunSummary[];
  inventory: unknown;
}

export function sessionDetail(db: Db, id: string): SessionDetail | undefined {
  const session = sessionRow(db, id);
  if (!session) {
    return undefined;
  }

  const runs = db.prepare("SELECT * FROM runs WHERE session_id = ? ORDER BY created_at DESC").all(id) as RunRow[];
  const inventory = db
    .prepare("SELECT snapshot FROM inventory WHERE canonical_folder = ?")
    .get(session.canonical_folder) as { snapshot: string } | undefined;

  return {
    ...summaryOf(db, session),
    runs: runs.map(runSummaryOf),
    inventory: inventory ? JSON.parse(inventory.snapshot) : null,
  };
}

// ── Browser: dispatch ──

const COMMANDS = new Set(["version", "us-create", "us-refine", "us-apply", "build-tests", "build-code"]);

export type DispatchResult =
  | { kind: "created"; run_id: string }
  | { kind: "deduped"; run_id: string }
  | { kind: "invalid" }
  | { kind: "not_found" }
  | { kind: "conflict" };

/**
 * Admits (or rejects) a browser dispatch. Typed-enum validation is a 400;
 * a repeated client_token returns the SAME run (200 dedup); an unreachable
 * target session or ANY non-terminal run on it (including `queued`) is a
 * 409. Dedup, reachability, and the `UNIQUE(session_id) WHERE state !=
 * 'terminal'` insert run in ONE `BEGIN IMMEDIATE` transaction so two
 * connections racing distinct tokens yield exactly one run — the loser's
 * insert violates the partial unique index and maps to a 409. Sibling
 * sessions on the same folder are never blocked here (the flock decides).
 */
export function dispatchRun(
  db: Db,
  sessionId: string,
  body: { command?: unknown; story_id?: unknown; fix?: unknown; client_token?: unknown },
): DispatchResult {
  if (
    typeof body.command !== "string" ||
    !COMMANDS.has(body.command) ||
    typeof body.client_token !== "string" ||
    body.client_token.length === 0
  ) {
    return { kind: "invalid" };
  }

  const command = body.command;
  const token = body.client_token;
  const storyId = typeof body.story_id === "string" ? body.story_id : null;
  const fix = body.fix === true ? 1 : 0;

  const admit = db.transaction((): DispatchResult => {
    const session = sessionRow(db, sessionId);
    if (!session) {
      return { kind: "not_found" };
    }

    const dedup = runByToken(db, token);
    if (dedup) {
      return { kind: "deduped", run_id: dedup.id };
    }

    if (reachabilityOf(session) === "unreachable") {
      return { kind: "conflict" };
    }

    const runId = randomUUID();
    const timestamp = now();
    try {
      db.prepare(
        `INSERT INTO runs (id, session_id, command, story_id, fix, state, client_token, answer_seq, answers_consumed, created_at, updated_at)
         VALUES (?, ?, ?, ?, ?, 'queued', ?, 0, 0, ?, ?)`,
      ).run(runId, sessionId, command, storyId, fix, token, timestamp, timestamp);
    } catch {
      const raced = runByToken(db, token);
      if (raced) {
        return { kind: "deduped", run_id: raced.id };
      }

      return { kind: "conflict" };
    }

    return { kind: "created", run_id: runId };
  });

  return admit.immediate();
}

function runByToken(db: Db, token: string): { id: string } | undefined {
  return db.prepare("SELECT id FROM runs WHERE client_token = ?").get(token) as { id: string } | undefined;
}

// ── Browser: refresh ──

export function requestRefresh(db: Db, sessionId: string): boolean {
  const session = sessionRow(db, sessionId);
  if (!session) {
    return false;
  }

  db.prepare("UPDATE sessions SET want_inventory = 1 WHERE id = ?").run(sessionId);

  return true;
}

// ── Browser: run detail ──

export interface RunDetail extends RunSummary {
  session_id: string;
  events: RunEvent[];
  pending_prompt: PendingPrompt | null;
}

export function runDetail(db: Db, runId: string, afterSeq: number): RunDetail | undefined {
  const run = runRow(db, runId);
  if (!run) {
    return undefined;
  }

  const eventRows = db
    .prepare("SELECT seq, type, data FROM events WHERE run_id = ? AND seq > ? ORDER BY seq")
    .all(runId, afterSeq) as { seq: number; type: string; data: string }[];
  const events: RunEvent[] = eventRows.map((row) => ({ seq: row.seq, type: row.type, ...JSON.parse(row.data) }));

  return {
    ...runSummaryOf(run),
    session_id: run.session_id,
    events,
    pending_prompt: pendingPrompt(db, run),
  };
}

function pendingPrompt(db: Db, run: RunRow): PendingPrompt | null {
  if (run.state === "terminal") {
    return null;
  }

  const row = db
    .prepare(
      `SELECT p.prompt_id, p.kind, p.payload FROM prompts p
       LEFT JOIN answers a ON a.run_id = p.run_id AND a.prompt_id = p.prompt_id
       WHERE p.run_id = ? AND a.prompt_id IS NULL
       ORDER BY p.seq DESC LIMIT 1`,
    )
    .get(run.id) as { prompt_id: string; kind: string; payload: string } | undefined;

  if (!row) {
    return null;
  }

  return { prompt_id: row.prompt_id, kind: row.kind, payload: JSON.parse(row.payload) };
}

// ── Browser: answer (first-wins, durable) ──

export type AnswerResult = "accepted" | "conflict" | "invalid" | "not_found";

/**
 * Records a browser answer (plan §3.3 / finding 6). Every check runs inside
 * ONE BEGIN IMMEDIATE transaction so the terminal/pending-prompt state a
 * decision is based on cannot change between check and insert:
 *
 * - unknown run/prompt or a malformed value → invalid, no insert;
 * - an EXACT retry of an already-accepted answer → accepted (idempotent);
 * - a conflicting retry of an already-accepted prompt → conflict;
 * - a NEW answer is accepted only when the run is nonterminal AND the
 *   prompt is the current unanswered pending prompt, and only when the
 *   conditional state update changes exactly one row; otherwise it is a
 *   409-class conflict with NO answer row inserted (so a brand-new answer
 *   can never be durably accepted after the run went terminal).
 */
export function submitAnswer(db: Db, runId: string, promptId: unknown, value: unknown): AnswerResult {
  if (typeof promptId !== "string" || typeof value !== "string") {
    return runRow(db, runId) ? "invalid" : "not_found";
  }

  const accept = db.transaction((): AnswerResult => {
    const run = runRow(db, runId);
    if (!run) {
      return "not_found";
    }

    const prompt = db
      .prepare("SELECT kind FROM prompts WHERE run_id = ? AND prompt_id = ?")
      .get(runId, promptId) as { kind: string } | undefined;
    if (!prompt || !validAnswer(prompt.kind, value)) {
      return "invalid";
    }

    const existing = db
      .prepare("SELECT value FROM answers WHERE run_id = ? AND prompt_id = ?")
      .get(runId, promptId) as { value: string } | undefined;
    if (existing) {
      // First accepted answer wins durably: an EXACT retry is idempotent
      // (accepted), a different value is a conflict.
      return existing.value === value ? "accepted" : "conflict";
    }

    // A new answer is only legal for the current unanswered pending prompt
    // on a nonterminal run.
    const pending = pendingPromptId(db, run);
    if (pending !== promptId) {
      return "conflict";
    }

    // Acceptance is CONDITIONAL on a state update that flips exactly one
    // nonterminal run to answer_accepted. If the run went terminal between
    // the pending-prompt check and here, zero rows change ⇒ conflict, and
    // no answer row is ever inserted.
    const timestamp = now();
    const answerSeq = run.answer_seq + 1;
    const updated = db
      .prepare(
        "UPDATE runs SET answer_seq = ?, state = 'answer_accepted', updated_at = ? WHERE id = ? AND state != 'terminal'",
      )
      .run(answerSeq, timestamp, runId);
    if (updated.changes !== 1) {
      return "conflict";
    }

    db.prepare("INSERT INTO answers (run_id, prompt_id, value, answer_seq, created_at) VALUES (?, ?, ?, ?, ?)").run(
      runId,
      promptId,
      value,
      answerSeq,
      timestamp,
    );

    return "accepted";
  });

  return accept.immediate();
}

/** The prompt_id of the current unanswered pending prompt, or null. */
function pendingPromptId(db: Db, run: RunRow): string | null {
  return pendingPrompt(db, run)?.prompt_id ?? null;
}

function validAnswer(kind: string, value: string): boolean {
  if (kind === "choice") {
    return CHOICE_VALUES.has(value);
  }

  if (kind === "clarify") {
    return value.length > 0 && !/[\r\n]/.test(value) && Buffer.byteLength(value, "utf8") <= MAX_CLARIFY_BYTES;
  }

  if (kind === "freetext") {
    return value.length > 0 && Buffer.byteLength(value, "utf8") <= MAX_FREETEXT_BYTES;
  }

  return false;
}

// ── Browser: mark abandoned ──

export type AbandonResult = "abandoned" | "conflict" | "not_found";

/**
 * Explicit abandonment for a run whose session is unreachable (plan
 * §3.3): only eligible when the owning session is unreachable AND the run
 * is non-terminal — otherwise a 409. The terminal transition is guarded
 * exactly-once and triggers the folder-wide auto-refresh.
 */
export function markAbandoned(db: Db, runId: string): AbandonResult {
  const run = runRow(db, runId);
  if (!run) {
    return "not_found";
  }

  const session = sessionRow(db, run.session_id);
  if (!session || run.state === "terminal" || reachabilityOf(session) === "connected") {
    return "conflict";
  }

  const res = db
    .prepare("UPDATE runs SET state = 'terminal', outcome = 'abandoned', updated_at = ? WHERE id = ? AND state != 'terminal'")
    .run(now(), runId);
  if (res.changes === 0) {
    return "conflict";
  }

  autoRefreshFolder(db, runId);

  return "abandoned";
}

// ── Shared row helpers ──

function runSummaryOf(run: RunRow): RunSummary {
  return {
    id: run.id,
    command: run.command,
    story_id: run.story_id,
    fix: run.fix === 1,
    state: run.state,
    outcome: run.outcome,
    error_detail: run.error_detail,
    envelope: {
      classification: run.outcome,
      engine_outcome: run.engine_outcome ?? null,
      finalization_ok: run.finalization_ok === null ? null : run.finalization_ok === 1,
      exit_code: run.exit_code ?? null,
      signal: run.signal ?? null,
    },
    created_at: run.created_at,
    updated_at: run.updated_at,
  };
}
