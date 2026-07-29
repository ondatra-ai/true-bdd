/**
 * Real-SQLite: dispatch admission (plan §3.3/§4.6).
 *   - typed-enum validation (invalid);
 *   - same client_token dedup (same run id);
 *   - two REAL connections racing distinct tokens ⇒ exactly one run
 *     (the partial unique index + BEGIN IMMEDIATE);
 *   - unreachable target session ⇒ conflict;
 *   - ANY non-terminal run on the target session ⇒ conflict;
 *   - a same-folder SIBLING session is never blocked.
 */

import { afterEach, describe, expect, it } from "vitest";

import { REACHABILITY_THRESHOLD_MS, dispatchRun } from "../../app/lib/store";
import { cleanup, makeTempDb, openSecondConnection, seedSession, type TempDb } from "./support/temp-db";

let temp: TempDb;

afterEach(() => cleanup(temp));

function backdatePoll(db: TempDb["db"], sessionId: string, ms: number): void {
  db.prepare("UPDATE sessions SET last_poll_at = ? WHERE id = ?").run(Date.now() - ms, sessionId);
}

describe("dispatch admission", () => {
  it("rejects a command outside the typed allowlist", () => {
    temp = makeTempDb();
    const session = seedSession(temp.db);

    expect(dispatchRun(temp.db, session, { command: "version; echo pwned", fix: false, client_token: "t" }).kind).toBe(
      "invalid",
    );
    expect(dispatchRun(temp.db, session, { command: "us-refine", fix: false, client_token: "" }).kind).toBe("invalid");
    expect(dispatchRun(temp.db, session, { command: 42, fix: false, client_token: "t" }).kind).toBe("invalid");
  });

  it("returns not_found for an unknown session", () => {
    temp = makeTempDb();
    expect(dispatchRun(temp.db, "nope", { command: "version", fix: false, client_token: "t" }).kind).toBe("not_found");
  });

  it("dedups a repeated client_token to the SAME run id", () => {
    temp = makeTempDb();
    const session = seedSession(temp.db);

    const first = dispatchRun(temp.db, session, { command: "version", fix: false, client_token: "dup" });
    expect(first.kind).toBe("created");
    const second = dispatchRun(temp.db, session, { command: "version", fix: false, client_token: "dup" });
    expect(second.kind).toBe("deduped");
    expect(second.kind === "deduped" && second.run_id).toBe(first.kind === "created" && first.run_id);
  });

  it("two connections racing DISTINCT tokens yield exactly one run", () => {
    temp = makeTempDb();
    const session = seedSession(temp.db);
    const other = openSecondConnection(temp);

    try {
      const a = dispatchRun(temp.db, session, { command: "version", fix: false, client_token: "race-a" });
      const b = dispatchRun(other, session, { command: "version", fix: false, client_token: "race-b" });

      const kinds = [a.kind, b.kind].sort();
      expect(kinds).toEqual(["conflict", "created"]);

      const count = temp.db.prepare("SELECT COUNT(*) AS n FROM runs WHERE session_id = ?").get(session) as { n: number };
      expect(count.n).toBe(1);
    } finally {
      other.close();
    }
  });

  it("conflicts when the target session is unreachable", () => {
    temp = makeTempDb();
    const session = seedSession(temp.db);
    backdatePoll(temp.db, session, REACHABILITY_THRESHOLD_MS + 5_000);

    expect(dispatchRun(temp.db, session, { command: "version", fix: false, client_token: "t" }).kind).toBe("conflict");
  });

  it("conflicts on ANY non-terminal run on the same session (distinct token)", () => {
    temp = makeTempDb();
    const session = seedSession(temp.db);

    expect(dispatchRun(temp.db, session, { command: "version", fix: false, client_token: "t1" }).kind).toBe("created");
    expect(dispatchRun(temp.db, session, { command: "us-refine", story_id: "1.1", fix: true, client_token: "t2" }).kind).toBe(
      "conflict",
    );
  });

  it("does NOT block a sibling session on the same folder", () => {
    temp = makeTempDb();
    const folder = "/host/shared";
    const sessionA = seedSession(temp.db, folder);
    const sessionB = seedSession(temp.db, folder);

    expect(dispatchRun(temp.db, sessionA, { command: "version", fix: false, client_token: "a" }).kind).toBe("created");
    // The sibling accepts its own dispatch despite folder-mate activity.
    expect(dispatchRun(temp.db, sessionB, { command: "version", fix: false, client_token: "b" }).kind).toBe("created");
  });
});
