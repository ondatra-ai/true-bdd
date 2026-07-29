/**
 * Real-SQLite: folder-global scan epochs + ticket-bound inventory
 * promotion (plan §3.3 handoff c / finding 2 / §4.6).
 *   - epochs are issued from a FOLDER-level monotonic counter (register
 *     and want_inventory polls), so two sessions share one ordering;
 *   - an upload must carry a TICKET the server issued to THIS session for
 *     THIS folder (an unissued epoch is rejected);
 *   - promotion only on a strictly newer epoch than the promoted one — an
 *     equal-epoch new token does NOT promote, a delayed stale one never
 *     demotes;
 *   - an EXACT retry (same token, epoch, payload) returns its RECORDED
 *     generation even after a newer upload; a mismatched retry is rejected.
 *
 * NOTE (conflict flagged in the fix report): the previous version of this
 * suite uploaded arbitrary epochs with NO session/ticket binding — it
 * pinned the pre-finding-2 behavior the review rejected. It is rewritten
 * here to issue real tickets and assert the ordered, bound contract.
 */

import { afterEach, describe, expect, it } from "vitest";

import { pollSession, registerSession, requestRefresh, upsertInventory } from "../../app/lib/store";
import { cleanup, makeTempDb, type TempDb } from "./support/temp-db";

let temp: TempDb;

afterEach(() => cleanup(temp));

/** Registers a session on `folder` and returns its id + first ticket epoch. */
function register(db: TempDb["db"], folder: string, incarnation: string): { session_id: string; scan_epoch: number } {
  return registerSession(db, { incarnation_id: incarnation, folder, canonical_folder: folder, pid: 1, version: "v" });
}

/** Issues the next want_inventory ticket for a session (fresh epoch). */
function nextTicket(db: TempDb["db"], sessionId: string): number {
  requestRefresh(db, sessionId);
  const poll = pollSession(db, sessionId, undefined);
  if (!poll || poll.scan_epoch <= 0) {
    throw new Error("expected a fresh want_inventory ticket");
  }

  return poll.scan_epoch;
}

describe("scan epochs + ticket-bound inventory promotion", () => {
  it("issues a folder-global monotonic epoch across register and want_inventory polls", () => {
    temp = makeTempDb();
    const folder = "/host/epochs";
    const a = register(temp.db, folder, "inc-a");
    const b = register(temp.db, folder, "inc-b");

    // Two sessions on ONE folder draw from the same counter.
    expect(b.scan_epoch).toBeGreaterThan(a.scan_epoch);

    // An ordinary heartbeat issues no new epoch (0 ⇒ remote keeps its own).
    const idle = pollSession(temp.db, a.session_id, undefined);
    expect(idle?.scan_epoch).toBe(0);
    expect(idle?.want_inventory).toBe(false);

    // A want_inventory poll issues the next epoch, strictly higher.
    const refreshed = nextTicket(temp.db, a.session_id);
    expect(refreshed).toBeGreaterThan(b.scan_epoch);
  });

  it("rejects an upload whose epoch was never issued to the session/folder", () => {
    temp = makeTempDb();
    const folder = "/host/ticket";
    const a = register(temp.db, folder, "inc-a");

    // An epoch the server never handed out ⇒ unknown_ticket, no promotion.
    expect(
      upsertInventory(temp.db, {
        session_id: a.session_id,
        canonical_folder: folder,
        scan_epoch: a.scan_epoch + 999,
        snapshot: { forged: true },
        client_token: "forged",
      }),
    ).toEqual({ rejected: "unknown_ticket" });

    // A different session cannot spend THIS session's ticket epoch either.
    const b = register(temp.db, folder, "inc-b");
    expect(
      upsertInventory(temp.db, {
        session_id: b.session_id,
        canonical_folder: folder,
        scan_epoch: a.scan_epoch,
        snapshot: { v: 1 },
        client_token: "b-steals-a",
      }),
    ).toEqual({ rejected: "unknown_ticket" });
  });

  it("promotes on a strictly newer ticket and never demotes on a delayed stale one", () => {
    temp = makeTempDb();
    const folder = "/host/promote";
    const a = register(temp.db, folder, "inc-a");

    // First snapshot on the register ticket ⇒ generation 1.
    expect(
      upsertInventory(temp.db, {
        session_id: a.session_id,
        canonical_folder: folder,
        scan_epoch: a.scan_epoch,
        snapshot: { v: 1 },
        client_token: "t1",
      }),
    ).toEqual({ generation: 1 });

    // A strictly newer ticket ⇒ generation 2.
    const newer = nextTicket(temp.db, a.session_id);
    expect(
      upsertInventory(temp.db, {
        session_id: a.session_id,
        canonical_folder: folder,
        scan_epoch: newer,
        snapshot: { v: 2 },
        client_token: "t2",
      }),
    ).toEqual({ generation: 2 });

    // A DELAYED stale scan carrying the OLD (still-valid) ticket with a NEW
    // token must NOT demote: generation stays 2, snapshot unchanged.
    expect(
      upsertInventory(temp.db, {
        session_id: a.session_id,
        canonical_folder: folder,
        scan_epoch: a.scan_epoch,
        snapshot: { v: 99 },
        client_token: "t-stale",
      }),
    ).toEqual({ generation: 2 });
    const row = temp.db.prepare("SELECT generation, snapshot FROM inventory WHERE canonical_folder = ?").get(folder) as {
      generation: number;
      snapshot: string;
    };
    expect(row.generation).toBe(2);
    expect(JSON.parse(row.snapshot)).toEqual({ v: 2 });
  });

  it("does NOT promote an equal-epoch new token (slow-scan/concurrent-poll race)", () => {
    temp = makeTempDb();
    const folder = "/host/equal";
    const a = register(temp.db, folder, "inc-a");

    expect(
      upsertInventory(temp.db, {
        session_id: a.session_id,
        canonical_folder: folder,
        scan_epoch: a.scan_epoch,
        snapshot: { v: 1 },
        client_token: "first",
      }),
    ).toEqual({ generation: 1 });

    // A DIFFERENT token at the SAME epoch is not strictly newer ⇒ no promote.
    expect(
      upsertInventory(temp.db, {
        session_id: a.session_id,
        canonical_folder: folder,
        scan_epoch: a.scan_epoch,
        snapshot: { v: 2 },
        client_token: "second-equal",
      }),
    ).toEqual({ generation: 1 });
  });

  it("never demotes when the stale delivery is the FIRST after a fresher promotion", () => {
    temp = makeTempDb();
    const folder = "/host/first-stale";
    const a = register(temp.db, folder, "inc-a");
    const older = a.scan_epoch;
    const newer = nextTicket(temp.db, a.session_id);

    // The fresher ticket is promoted first ⇒ generation 1.
    expect(
      upsertInventory(temp.db, {
        session_id: a.session_id,
        canonical_folder: folder,
        scan_epoch: newer,
        snapshot: { fresh: true },
        client_token: "hi",
      }),
    ).toEqual({ generation: 1 });

    // The older ticket arrives late ⇒ rejected as a demotion, generation 1.
    expect(
      upsertInventory(temp.db, {
        session_id: a.session_id,
        canonical_folder: folder,
        scan_epoch: older,
        snapshot: { fresh: false },
        client_token: "lo",
      }),
    ).toEqual({ generation: 1 });
  });

  it("an EXACT retry replays its recorded generation; a mismatched retry is rejected", () => {
    temp = makeTempDb();
    const folder = "/host/token";
    const a = register(temp.db, folder, "inc-a");

    const first = upsertInventory(temp.db, {
      session_id: a.session_id,
      canonical_folder: folder,
      scan_epoch: a.scan_epoch,
      snapshot: { a: 1 },
      client_token: "same",
    });
    expect(first).toEqual({ generation: 1 });

    // A newer, distinct-token upload advances to generation 2 …
    const newer = nextTicket(temp.db, a.session_id);
    expect(
      upsertInventory(temp.db, {
        session_id: a.session_id,
        canonical_folder: folder,
        scan_epoch: newer,
        snapshot: { a: 2 },
        client_token: "new",
      }),
    ).toEqual({ generation: 2 });

    // … but an EXACT retry of the ORIGINAL token returns ITS recorded
    // generation 1 — it does not forget its assignment (inverted repro 3).
    expect(
      upsertInventory(temp.db, {
        session_id: a.session_id,
        canonical_folder: folder,
        scan_epoch: a.scan_epoch,
        snapshot: { a: 1 },
        client_token: "same",
      }),
    ).toEqual({ generation: 1 });

    // A MISMATCHED retry (same token, different epoch/payload) is rejected.
    expect(
      upsertInventory(temp.db, {
        session_id: a.session_id,
        canonical_folder: folder,
        scan_epoch: newer,
        snapshot: { a: 999 },
        client_token: "same",
      }),
    ).toEqual({ rejected: "mismatched_retry" });
  });
});
