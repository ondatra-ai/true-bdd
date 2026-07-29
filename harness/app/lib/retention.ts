/**
 * Retention & bounds configuration (plan §3.6, server side). Every knob
 * is env-tunable so the vitest suites and BDD fixtures can drive the
 * pruning paths with small values instead of the production defaults:
 *
 * - TRUE_BDD_HARNESS_BYTE_BUDGET     per-run stored-event byte budget
 * - TRUE_BDD_HARNESS_RETENTION_HEAD  event rows kept at the head
 * - TRUE_BDD_HARNESS_RETENTION_TAIL  event rows kept at the tail
 * - TRUE_BDD_HARNESS_KEEP_RUNS       terminal runs kept per folder
 * - TRUE_BDD_HARNESS_PRUNE_INTERVAL_MS   hourly-prune timer interval
 *
 * This module has NO other imports so it can be a leaf both store.ts and
 * db.ts depend on without an import cycle.
 */

export interface RetentionConfig {
  /** Per-run stored-event byte budget before HEAD+TAIL collapse. */
  byteBudget: number;
  /** Event rows retained at the head of a run's stream. */
  head: number;
  /** Event rows retained at the tail of a run's stream. */
  tail: number;
  /** Terminal runs retained per folder (older ones pruned). */
  keepPerFolder: number;
  /** Interval for the periodic prune timer (ms). */
  pruneIntervalMs: number;
}

const DEFAULTS: RetentionConfig = {
  byteBudget: 256 * 1024,
  head: 16,
  tail: 16,
  keepPerFolder: 50,
  pruneIntervalMs: 60 * 60 * 1000,
};

/** Reads a positive-integer env override, else the provided default. */
function intEnv(name: string, fallback: number): number {
  const raw = process.env[name];
  if (raw === undefined || raw.trim() === "") {
    return fallback;
  }

  const parsed = Number.parseInt(raw, 10);

  return Number.isFinite(parsed) && parsed >= 0 ? parsed : fallback;
}

/** The live retention configuration, re-read from the environment. */
export function retentionConfig(): RetentionConfig {
  return {
    byteBudget: intEnv("TRUE_BDD_HARNESS_BYTE_BUDGET", DEFAULTS.byteBudget),
    head: intEnv("TRUE_BDD_HARNESS_RETENTION_HEAD", DEFAULTS.head),
    tail: intEnv("TRUE_BDD_HARNESS_RETENTION_TAIL", DEFAULTS.tail),
    keepPerFolder: intEnv("TRUE_BDD_HARNESS_KEEP_RUNS", DEFAULTS.keepPerFolder),
    pruneIntervalMs: intEnv("TRUE_BDD_HARNESS_PRUNE_INTERVAL_MS", DEFAULTS.pruneIntervalMs),
  };
}
