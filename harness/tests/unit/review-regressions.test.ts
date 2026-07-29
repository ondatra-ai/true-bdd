/**
 * Real-SQLite regression suite (finding 1 + 6): the five finished-solution
 * review reproductions, INVERTED to assert the CORRECT behavior now that
 * the defects are fixed. The originals (in tmp/codex-solution-review-repros
 * .test.ts) asserted the BROKEN behavior and passed; these assert the fixed
 * behavior and would fail against the old code.
 *
 * (The inventory-token repro is inverted in scan-epoch.test.ts; the agent
 * `Origin: null` repro is inverted in origin.test.ts.)
 */

import { afterEach, describe, expect, it } from "vitest";

import { appendEvents, dispatchRun, pruneTerminalRuns, submitAnswer } from "../../app/lib/store";
import { cleanup, makeTempDb, seedSession, type TempDb } from "./support/temp-db";

let temp: TempDb;

afterEach(() => cleanup(temp));

function queueRun(db: TempDb["db"], sessionId: string, token: string): string {
  const result = dispatchRun(db, sessionId, { command: "us-refine", story_id: "1.1", fix: true, client_token: token });
  if (result.kind !== "created") {
    throw new Error(`expected created, got ${result.kind}`);
  }

  return result.run_id;
}

function stateOf(db: TempDb["db"], runId: string): string {
  return (db.prepare("SELECT state FROM runs WHERE id = ?").get(runId) as { state: string }).state;
}

describe("finished-solution review reproductions — INVERTED to the fixed contract", () => {
  it("a range tombstone colliding with a committed seq now advances the contiguous ACK to terminal", () => {
    temp = makeTempDb();
    const run = queueRun(temp.db, seedSession(temp.db), "gap");

    expect(
      appendEvents(temp.db, run, [
        { seq: 1, type: "output", data: "one" },
        { seq: 2, type: "output", data: "two" },
      ]).acked_through,
    ).toBe(2);

    // A terminal at seq 5 arrives before the 3–4 hole is covered: it must
    // NOT terminalize out of contiguous order.
    expect(appendEvents(temp.db, run, [{ seq: 5, type: "terminal", outcome: "ok" }]).acked_through).toBe(2);
    expect(stateOf(temp.db, run)).not.toBe("terminal");

    // The remote's retained tombstone [2,4] collides with the committed
    // seq-2 output. Its coverage is recorded SEPARATELY, so 3–4 is covered
    // and the watermark reaches the terminal seq 5.
    const ack = appendEvents(temp.db, run, [{ seq: 2, type: "gap", through_seq: 4, dropped_bytes: 10 }]);
    expect(ack.acked_through).toBe(5);
    expect(stateOf(temp.db, run)).toBe("terminal");
  });

  it("a redelivered prompt never regresses answer state, and a forged terminal at an occupied seq is ignored", () => {
    temp = makeTempDb();
    const run = queueRun(temp.db, seedSession(temp.db), "dup");

    appendEvents(temp.db, run, [
      { seq: 1, type: "prompt", prompt_id: "p1", kind: "choice", payload: { actions: ["exit"] } },
    ]);
    expect(submitAnswer(temp.db, run, "p1", "exit")).toBe("accepted");

    // Redelivering seq 1 (exact dup) with answers_consumed=1 must not roll
    // the prompt transition back: the answer advances to consumed.
    appendEvents(
      temp.db,
      run,
      [{ seq: 1, type: "prompt", prompt_id: "p1", kind: "choice", payload: { actions: ["exit"] } }],
      1,
    );
    const progressed = temp.db.prepare("SELECT state, answers_consumed FROM runs WHERE id = ?").get(run) as {
      state: string;
      answers_consumed: number;
    };
    expect(progressed).toEqual({ state: "answer_consumed", answers_consumed: 1 });

    // A CONFLICTING payload (terminal) at the already-occupied seq 1 is
    // rejected — it can no longer terminalize the run.
    appendEvents(temp.db, run, [{ seq: 1, type: "terminal", outcome: "error", error_detail: "forged" }]);
    const stillLive = temp.db.prepare("SELECT state, outcome FROM runs WHERE id = ?").get(run) as {
      state: string;
      outcome: string | null;
    };
    expect(stillLive.state).toBe("answer_consumed");
    expect(stillLive.outcome).toBeNull();
  });

  it("a brand-new answer is REJECTED (no insert) once the run is terminal", () => {
    temp = makeTempDb();
    const run = queueRun(temp.db, seedSession(temp.db), "term");
    appendEvents(temp.db, run, [
      { seq: 1, type: "prompt", prompt_id: "p1", kind: "choice", payload: { actions: ["exit"] } },
      { seq: 2, type: "terminal", outcome: "ok" },
    ]);

    expect(submitAnswer(temp.db, run, "p1", "exit")).toBe("conflict");
    const count = temp.db.prepare("SELECT COUNT(*) AS n FROM answers WHERE run_id = ?").get(run) as { n: number };
    expect(count.n).toBe(0);
  });

  it("a late terminal replay of a PRUNED run is still acknowledged via its retained receipt", () => {
    temp = makeTempDb();
    const run = queueRun(temp.db, seedSession(temp.db), "receipt");
    appendEvents(temp.db, run, [
      { seq: 1, type: "output", data: "true-bdd version 1.0\n" },
      { seq: 2, type: "terminal", outcome: "ok" },
    ]);

    // Prune every terminal run for the folder (keep 0): the run row is gone.
    pruneTerminalRuns(temp.db, 0);
    expect(temp.db.prepare("SELECT COUNT(*) AS n FROM runs WHERE id = ?").get(run)).toEqual({ n: 0 });

    // A replay of the terminal (its commit response was lost) is still
    // acknowledged through the retained receipt — the spool can finally
    // delete rather than replay forever.
    const ack = appendEvents(temp.db, run, [{ seq: 2, type: "terminal", outcome: "ok" }]);
    expect(ack.acked_through).toBe(2);
  });

  it("rejects a structurally invalid batch without mutating state", () => {
    temp = makeTempDb();
    const run = queueRun(temp.db, seedSession(temp.db), "invalid");
    appendEvents(temp.db, run, [{ seq: 1, type: "output", data: "a" }]);

    // Non-positive seq, unknown type, and out-of-order seq are all rejected.
    expect(appendEvents(temp.db, run, [{ seq: 0, type: "output", data: "x" }]).rejected).toBe(true);
    expect(appendEvents(temp.db, run, [{ seq: 2, type: "bogus" }]).rejected).toBe(true);
    expect(
      appendEvents(temp.db, run, [
        { seq: 3, type: "output", data: "c" },
        { seq: 2, type: "output", data: "b" },
      ]).rejected,
    ).toBe(true);

    // Only the valid seq 1 was ever stored.
    const seqs = temp.db.prepare("SELECT seq FROM events WHERE run_id = ? ORDER BY seq").all(run) as { seq: number }[];
    expect(seqs.map((row) => row.seq)).toEqual([1]);
  });
});
