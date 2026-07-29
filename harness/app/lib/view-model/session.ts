/**
 * Pure view-model for the session-list + session-detail control surfaces
 * (plan §3.5). The control-disable rule and the sibling-warning rule are
 * the two behaviors the protocol suite (P5) gates on; keeping them as pure
 * predicates lets the vitest tables (§4.6) cover every case without a
 * browser.
 */

import type { SessionDetail, SessionSummary } from "./types";

/** After this long without a fresh promoted inventory a session is stale. */
export const INVENTORY_STALE_AFTER_MS = 60_000;

export interface InventoryFreshnessView {
  /** ms of the last promoted inventory, or null when none exists. */
  updatedAt: number | null;
  /** ms since the last promoted inventory, or null when none exists. */
  ageMs: number | null;
  /** Explicit staleness: no inventory yet, or older than the threshold. */
  stale: boolean;
}

/**
 * Inventory freshness for the sessions + detail views (plan §3.3/§3.5 /
 * finding 8): the promoted-inventory age and an EXPLICIT staleness flag,
 * computed against an injectable clock so the view-model tables are
 * deterministic. A session with no promoted inventory is stale by
 * definition; otherwise staleness is age past `staleAfterMs`.
 */
export function inventoryFreshness(
  session: Pick<SessionSummary, "inventory_updated_at"> | null | undefined,
  clockNow: number = Date.now(),
  staleAfterMs: number = INVENTORY_STALE_AFTER_MS,
): InventoryFreshnessView {
  const updatedAt = typeof session?.inventory_updated_at === "number" ? session.inventory_updated_at : null;
  if (updatedAt === null) {
    return { updatedAt: null, ageMs: null, stale: true };
  }

  const ageMs = Math.max(0, clockNow - updatedAt);

  return { updatedAt, ageMs, stale: ageMs > staleAfterMs };
}

/**
 * Whether a session's action controls (Create/Refine/Apply, build tests /
 * build code) are disabled. Disabled ONLY for the session's OWN active run
 * or an unreachable session — sibling-session folder activity never
 * disables (it warns; see siblingWarningVisible). README-testids / P5.
 */
export function controlsDisabled(session: Pick<SessionSummary, "active_run_id" | "reachability"> | null | undefined): boolean {
  if (!session) {
    return true;
  }

  return session.active_run_id !== null || session.reachability === "unreachable";
}

/**
 * Whether the folder-warning banner shows: true when a DIFFERENT session
 * on the SAME canonical folder has a non-terminal run (its active_run_id
 * is set). Derived from GET /api/sessions (P5). The session's own active
 * run does NOT trigger the banner — that is the disable rule above.
 */
export function siblingWarningVisible(
  sessions: SessionSummary[] | undefined,
  me: Pick<SessionDetail, "id" | "folder"> | null | undefined,
): boolean {
  if (!sessions || !me) {
    return false;
  }

  return sessions.some(
    (candidate) => candidate.id !== me.id && candidate.folder === me.folder && candidate.active_run_id !== null,
  );
}
