/**
 * Component/view-model tables (plan §4.6): the control-disable rule and
 * the sibling-warning rule — the two session-detail behaviors P5 gates on.
 * Pure predicates; no DOM.
 */

import { describe, expect, it } from "vitest";

import {
  INVENTORY_STALE_AFTER_MS,
  controlsDisabled,
  inventoryFreshness,
  siblingWarningVisible,
} from "../../app/lib/view-model/session";
import type { SessionSummary } from "../../app/lib/view-model/types";

function session(overrides: Partial<SessionSummary> = {}): SessionSummary {
  return {
    id: "s1",
    folder: "/host/proj",
    pid: 1000,
    reachability: "connected",
    active_run_id: null,
    inventory_generation: 1,
    inventory_updated_at: null,
    ...overrides,
  };
}

describe("controlsDisabled", () => {
  it("enabled: connected session with no active run", () => {
    expect(controlsDisabled(session())).toBe(false);
  });

  it("disabled: the session's own active run", () => {
    expect(controlsDisabled(session({ active_run_id: "r1" }))).toBe(true);
  });

  it("disabled: an unreachable session (even with no active run)", () => {
    expect(controlsDisabled(session({ reachability: "unreachable" }))).toBe(true);
  });

  it("disabled: an unreachable session with an active run", () => {
    expect(controlsDisabled(session({ reachability: "unreachable", active_run_id: "r1" }))).toBe(true);
  });

  it("disabled: no session loaded yet", () => {
    expect(controlsDisabled(null)).toBe(true);
    expect(controlsDisabled(undefined)).toBe(true);
  });
});

describe("siblingWarningVisible", () => {
  const me = { id: "s1", folder: "/host/proj" };

  it("warns when a DIFFERENT session on the SAME folder has an active run", () => {
    const sessions = [session({ id: "s1" }), session({ id: "s2", active_run_id: "r9" })];
    expect(siblingWarningVisible(sessions, me)).toBe(true);
  });

  it("does NOT warn for the session's OWN active run", () => {
    const sessions = [session({ id: "s1", active_run_id: "r1" })];
    expect(siblingWarningVisible(sessions, me)).toBe(false);
  });

  it("does NOT warn for a sibling on a DIFFERENT folder", () => {
    const sessions = [session({ id: "s1" }), session({ id: "s2", folder: "/host/other", active_run_id: "r9" })];
    expect(siblingWarningVisible(sessions, me)).toBe(false);
  });

  it("does NOT warn for a same-folder sibling that is idle", () => {
    const sessions = [session({ id: "s1" }), session({ id: "s2", active_run_id: null })];
    expect(siblingWarningVisible(sessions, me)).toBe(false);
  });

  it("is hidden when the session list or self is missing", () => {
    expect(siblingWarningVisible(undefined, me)).toBe(false);
    expect(siblingWarningVisible([session()], null)).toBe(false);
  });
});

describe("inventoryFreshness (finding 8; clock-injected)", () => {
  const clockNow = 1_000_000;

  it("is stale with null age when no inventory has been promoted", () => {
    expect(inventoryFreshness(session({ inventory_updated_at: null }), clockNow)).toEqual({
      updatedAt: null,
      ageMs: null,
      stale: true,
    });
  });

  it("reports a fresh age below the threshold as not stale", () => {
    const updatedAt = clockNow - 5_000;
    expect(inventoryFreshness(session({ inventory_updated_at: updatedAt }), clockNow)).toEqual({
      updatedAt,
      ageMs: 5_000,
      stale: false,
    });
  });

  it("flags an age past the threshold as stale", () => {
    const updatedAt = clockNow - (INVENTORY_STALE_AFTER_MS + 1);
    const freshness = inventoryFreshness(session({ inventory_updated_at: updatedAt }), clockNow);
    expect(freshness.ageMs).toBe(INVENTORY_STALE_AFTER_MS + 1);
    expect(freshness.stale).toBe(true);
  });

  it("clamps a future timestamp to a zero, non-stale age", () => {
    expect(inventoryFreshness(session({ inventory_updated_at: clockNow + 10_000 }), clockNow)).toEqual({
      updatedAt: clockNow + 10_000,
      ageMs: 0,
      stale: false,
    });
  });
});
