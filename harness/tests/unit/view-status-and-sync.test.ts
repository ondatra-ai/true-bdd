/**
 * Transport §2/§2a tables (plan §4.6): the lightweight `?view=status`
 * session read, the atomic single-read inventory pair, and the client-side
 * retry-until-match generation logic.
 *
 *   - `sessionStatus(db, id)` — reachability + active run + run history +
 *     generation, and NO inventory snapshot (the session page polls this 1 Hz
 *     instead of re-shipping the full detail every second).
 *   - `sessionInventory(db, id)` — the generation AND the snapshot from ONE
 *     consistent read (fixing the non-atomic snapshot-then-summaryOf pair):
 *     an N+1 generation paired with an N snapshot is impossible at the store.
 *   - `inventoryStale` / `reconcileInventoryGeneration` — the client tracks
 *     TARGET (from status polls) vs LOADED generation; while they differ the
 *     inventory refetches, and reordered consecutive-generation responses
 *     never regress the loaded generation.
 *
 * None of these functions exist yet; each is accessed through a pinned cast
 * so the file typechecks and every table is RED until the transport is built.
 */

import type Database from "better-sqlite3";
import { afterEach, describe, expect, it } from "vitest";

import { dispatchRun, registerSession, upsertInventory } from "../../app/lib/store";
import * as store from "../../app/lib/store";
import * as inventoryVM from "../../app/lib/view-model/inventory";
import { cleanup, makeTempDb, type TempDb } from "./support/temp-db";

let temp: TempDb;

afterEach(() => cleanup(temp));

// ── Pinned target surfaces ──

interface SessionStatusView {
  id: string;
  reachability: string;
  active_run_id: string | null;
  inventory_generation: number;
  inventory_updated_at: number | null;
  runs: { id: string; command: string; state: string }[];
}

interface SessionInventoryRead {
  generation: number;
  updated_at: number | null;
  snapshot: unknown;
}

interface TargetStore {
  sessionStatus: (db: Database.Database, id: string) => SessionStatusView | undefined;
  sessionInventory: (db: Database.Database, id: string) => SessionInventoryRead | null;
}

interface TargetSync {
  inventoryStale: (targetGeneration: number, loadedGeneration: number) => boolean;
  reconcileInventoryGeneration: (loaded: number, incoming: number) => number;
}

const targetStore = store as unknown as TargetStore;
const sync = inventoryVM as unknown as TargetSync;

/** Registers a reachable session and returns its id + first ticket epoch. */
function register(db: Database.Database, folder: string): { session_id: string; scan_epoch: number } {
  return registerSession(db, { incarnation_id: `inc-${folder}`, folder, canonical_folder: folder, pid: 1, version: "v" });
}

describe("sessionStatus (?view=status)", () => {
  it("returns reachability, active run, runs, and generation but NO inventory", () => {
    temp = makeTempDb();
    const folder = "/host/status";
    const registered = register(temp.db, folder);

    // Seed a promoted inventory on the register ticket (generation 1).
    upsertInventory(temp.db, {
      session_id: registered.session_id,
      canonical_folder: folder,
      scan_epoch: registered.scan_epoch,
      snapshot: { epics: [], documents: {} },
      client_token: "inv-1",
    });

    // A queued run so active_run_id + runs are populated.
    const dispatched = dispatchRun(temp.db, registered.session_id, {
      command: "version",
      fix: false,
      client_token: "run-1",
    });
    expect(dispatched.kind).toBe("created");

    const status = targetStore.sessionStatus(temp.db, registered.session_id);
    expect(status).toBeDefined();
    if (status !== undefined) {
      expect(status.reachability).toBe("connected");
      expect(status.inventory_generation).toBe(1);
      expect(status.runs.length).toBe(1);
      expect(status.active_run_id).not.toBeNull();
      // The whole point of the lightweight view: it omits the snapshot.
      expect("inventory" in (status as unknown as Record<string, unknown>)).toBe(false);
    }
  });

  it("returns undefined for an unknown session", () => {
    temp = makeTempDb();
    expect(targetStore.sessionStatus(temp.db, "nope")).toBeUndefined();
  });
});

describe("sessionInventory (atomic generation+snapshot pair)", () => {
  it("returns the generation AND the snapshot promoted TOGETHER (single read)", () => {
    temp = makeTempDb();
    const folder = "/host/atomic";
    const registered = register(temp.db, folder);

    const snapshotV1 = { epics: [], documents: { config: "present" }, marker: "v1" };
    upsertInventory(temp.db, {
      session_id: registered.session_id,
      canonical_folder: folder,
      scan_epoch: registered.scan_epoch,
      snapshot: snapshotV1,
      client_token: "inv-1",
    });

    const read = targetStore.sessionInventory(temp.db, registered.session_id);
    expect(read).not.toBeNull();
    if (read !== null) {
      expect(read.generation).toBe(1);
      // The snapshot MUST be the one promoted at THIS generation — never a
      // newer generation number paired with an older snapshot.
      expect(read.snapshot).toEqual(snapshotV1);
    }
  });

  it("returns null when no inventory has been promoted", () => {
    temp = makeTempDb();
    const registered = register(temp.db, "/host/empty");
    expect(targetStore.sessionInventory(temp.db, registered.session_id)).toBeNull();
  });
});

describe("client retry-until-match generation logic", () => {
  it("inventoryStale is true only while the target generation is ahead of loaded", () => {
    expect(sync.inventoryStale(5, 3)).toBe(true);
    expect(sync.inventoryStale(3, 3)).toBe(false);
    expect(sync.inventoryStale(3, 5)).toBe(false); // loaded already ahead — do not refetch
    expect(sync.inventoryStale(1, 0)).toBe(true); // first load
  });

  it("reconcileInventoryGeneration never regresses on a reordered/older response", () => {
    expect(sync.reconcileInventoryGeneration(3, 5)).toBe(5); // catch up
    expect(sync.reconcileInventoryGeneration(5, 3)).toBe(5); // stale/reordered — keep loaded
    expect(sync.reconcileInventoryGeneration(5, 5)).toBe(5);
    expect(sync.reconcileInventoryGeneration(0, 1)).toBe(1);
  });
});
