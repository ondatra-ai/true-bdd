/**
 * Retention runtime (plan §3.6): arms the terminal-run pruning for the
 * process-wide singleton connection — once at startup, then on a periodic
 * timer. The interval is env-tunable (TRUE_BDD_HARNESS_PRUNE_INTERVAL_MS)
 * so tests can shrink it; the timer is `unref`'d so it never keeps the
 * process alive, and it is armed at most once per connection.
 *
 * Imported only by db.ts (db → retention-runtime → store → retention),
 * so the config leaf (retention.ts) and the store never depend back on
 * this module — no import cycle.
 */

import type Database from "better-sqlite3";

import { retentionConfig } from "./retention";
import { pruneTerminalRuns, pruneUnreachableSessions } from "./store";

const armed = new WeakSet<Database.Database>();

/** Runs a startup prune and schedules the periodic prune (idempotent). */
export function startRetention(db: Database.Database): void {
  if (armed.has(db)) {
    return;
  }
  armed.add(db);

  safePrune(db);

  const interval = retentionConfig().pruneIntervalMs;
  if (interval <= 0) {
    return;
  }

  const timer = setInterval(() => safePrune(db), interval);
  // Never let the prune timer hold the event loop open.
  if (typeof timer.unref === "function") {
    timer.unref();
  }
}

function safePrune(db: Database.Database): void {
  try {
    pruneTerminalRuns(db);
    pruneUnreachableSessions(db);
  } catch {
    // Best-effort hygiene: a prune failure must never crash a request.
  }
}
