/**
 * Out-of-band inventory-unavailable table (plan §1a / finding 1). When the
 * server's inventory limit is below the minimum viable request, no inventory
 * upload can ever be accepted, so the remote reports the terminal cannot-fit
 * state on its poll heartbeat instead. The store records that folder-/
 * session-scoped state and surfaces it on the session summary so the page can
 * render it honestly even though no snapshot exists.
 *
 * These assertions drive the REAL store + migrations against a temp-file DB.
 */

import { afterEach, describe, expect, it } from "vitest";

import { listSessions, pollSession, sessionStatus } from "../../app/lib/store";
import { cleanup, makeTempDb, seedSession, type TempDb } from "./support/temp-db";

let temp: TempDb;

afterEach(() => cleanup(temp));

describe("out-of-band inventory unavailable", () => {
  it("defaults to null and a normal poll leaves it null", () => {
    temp = makeTempDb();
    const sessionId = seedSession(temp.db, "/host/ok");

    // A heartbeat with no unavailable signal must not set the state.
    pollSession(temp.db, sessionId, undefined);

    const summary = listSessions(temp.db).find((session) => session.id === sessionId);
    expect(summary?.inventory_unavailable ?? null).toBeNull();
  });

  it("records the reported reason and surfaces it on the summary + status view", () => {
    temp = makeTempDb();
    const sessionId = seedSession(temp.db, "/host/tiny-cap");

    pollSession(temp.db, sessionId, undefined, "limit_too_small");

    const summary = listSessions(temp.db).find((session) => session.id === sessionId);
    expect(summary?.inventory_unavailable).toBe("limit_too_small");

    const status = sessionStatus(temp.db, sessionId);
    expect(status?.inventory_unavailable).toBe("limit_too_small");
  });

  it("is terminal: a later heartbeat without the signal does not clear it", () => {
    temp = makeTempDb();
    const sessionId = seedSession(temp.db, "/host/terminal");

    pollSession(temp.db, sessionId, undefined, "limit_too_small");
    pollSession(temp.db, sessionId, undefined); // ordinary heartbeat

    const summary = listSessions(temp.db).find((session) => session.id === sessionId);
    expect(summary?.inventory_unavailable).toBe("limit_too_small");
  });
});
