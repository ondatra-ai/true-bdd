/**
 * Pure view-model for the session-detail control surfaces (plan §4,
 * REWRITTEN for v2). v2 has no `reachability` (a readable session is
 * connected — sessions are GONE on disconnect) and no
 * `inventory_generation` (every read is a fresh scan). So:
 *   - `controlsDisabled` gates ONLY on the session's OWN active run, taken
 *     from the `session_status` view's `active_run`.
 *   - `siblingWarningVisible` is driven by `session_status.active_owners`
 *     (same-project active owners surfaced by the CLI), NOT by scanning the
 *     registry list — the relay does no joins.
 */

/** A same-project active owner (subset used by the warning rule). */
export interface OwnerRef {
  owner_id: string;
}

/** The active run reference (subset used by the disable rule). */
export interface ActiveRunRef {
  run_id: string;
}

/** The `session_status` fields the control surfaces depend on. */
export interface SessionStatusView {
  session_id: string;
  owner_id: string;
  active_run: ActiveRunRef | null;
  active_owners: OwnerRef[];
}

/**
 * Whether a session's action controls (Create/Refine/Apply, build tests /
 * build code) are disabled. Disabled ONLY for the session's OWN active run
 * (plan §4 / P5): a same-project SIBLING owner does not disable — it warns,
 * and the folder flock decides at dispatch time. A not-yet-loaded status is
 * disabled by default.
 */
export function controlsDisabled(status: SessionStatusView | null | undefined): boolean {
  if (!status) {
    return true;
  }

  return status.active_run !== null;
}

/**
 * Whether the folder-warning banner shows: true when a DIFFERENT owner is
 * active in the same project (plan §4 / P5). The session's OWN owner being
 * active never warns. Driven by `session_status.active_owners`.
 */
export function siblingWarningVisible(status: SessionStatusView | null | undefined): boolean {
  if (!status) {
    return false;
  }

  return status.active_owners.some((owner) => owner.owner_id !== status.owner_id);
}
