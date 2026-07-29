/**
 * Component/view-model tables (plan §4, REWRITTEN for v2): the control-
 * disable rule and the sibling-warning rule — the two session-detail
 * behaviors P5 gates on. Pure predicates; no DOM.
 *
 * v2 model (critique §2/§13):
 *   - There is no `reachability` (sessions are gone on disconnect — a
 *     readable session is connected) and no `inventory_generation` freshness
 *     (every read is a fresh scan). So `controlsDisabled` gates ONLY on the
 *     session's OWN active run, taken from the `session_status` view's
 *     `active_run` (not a flat `active_run_id` on the registry summary).
 *   - The sibling warning is driven by `session_status.active_owners`
 *     (same-project active owners surfaced by the CLI), NOT by scanning the
 *     registry list for same-folder sessions — the relay does no joins.
 *
 * The v2 signatures do not exist yet; each is accessed through a pinned cast
 * so the file typechecks and every table is RED until the view model is
 * rebuilt.
 */

import { describe, expect, it } from "vitest";

import * as sessionVM from "../../app/lib/view-model/session";

// ── Pinned v2 target surfaces ──

interface ActiveRunRef {
  run_id: string;
}

interface OwnerRef {
  owner_id: string;
}

interface StatusView {
  session_id: string;
  owner_id: string;
  active_run: ActiveRunRef | null;
  active_owners: OwnerRef[];
}

interface TargetSessionVM {
  controlsDisabled: (status: StatusView | null | undefined) => boolean;
  siblingWarningVisible: (status: StatusView | null | undefined) => boolean;
}

const vm = sessionVM as unknown as TargetSessionVM;

function status(overrides: Partial<StatusView> = {}): StatusView {
  return {
    session_id: "s1",
    owner_id: "owner-1",
    active_run: null,
    active_owners: [],
    ...overrides,
  };
}

describe("controlsDisabled (v2 — own active run only)", () => {
  it("enabled: a connected session with no active run", () => {
    expect(vm.controlsDisabled(status())).toBe(false);
  });

  it("disabled: the session's OWN active run", () => {
    expect(vm.controlsDisabled(status({ active_run: { run_id: "r1" } }))).toBe(true);
  });

  it("enabled even when a SIBLING owner is active (no own run)", () => {
    // A same-project sibling owner does not disable controls (it only warns);
    // the folder flock decides at dispatch time.
    expect(vm.controlsDisabled(status({ active_owners: [{ owner_id: "owner-2" }] }))).toBe(false);
  });

  it("disabled: no session loaded yet", () => {
    expect(vm.controlsDisabled(null)).toBe(true);
    expect(vm.controlsDisabled(undefined)).toBe(true);
  });
});

describe("siblingWarningVisible (v2 — from active_owners)", () => {
  it("warns when a DIFFERENT owner is active in the same project", () => {
    expect(vm.siblingWarningVisible(status({ active_owners: [{ owner_id: "owner-2" }] }))).toBe(true);
  });

  it("does NOT warn for the session's OWN owner being active", () => {
    expect(
      vm.siblingWarningVisible(status({ active_run: { run_id: "r1" }, active_owners: [{ owner_id: "owner-1" }] })),
    ).toBe(false);
  });

  it("does NOT warn when no other owner is active", () => {
    expect(vm.siblingWarningVisible(status({ active_owners: [] }))).toBe(false);
  });

  it("is hidden when the status is missing", () => {
    expect(vm.siblingWarningVisible(null)).toBe(false);
    expect(vm.siblingWarningVisible(undefined)).toBe(false);
  });
});
