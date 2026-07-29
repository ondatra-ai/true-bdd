/**
 * Real-SQLite: the run state machine (plan §3.3/§4.6).
 *   - full queued→claimed→running⇄prompt_published→answer_accepted→
 *     answer_consumed→terminal walk;
 *   - terminal transition guarded exactly-once;
 *   - event dedup incl. gap rows;
 *   - gap-aware contiguous acked_through;
 *   - answers_consumed_through echo;
 *   - flock-proof abandonment (older folder runs) + folder auto-refresh;
 *   - a folder_locked run abandons nothing.
 */

import { afterEach, describe, expect, it } from "vitest";

import {
  appendEvents,
  dispatchRun,
  pollSession,
  runDetail,
  sessionDetail,
  submitAnswer,
} from "../../app/lib/store";
import { cleanup, makeTempDb, seedSession, type TempDb } from "./support/temp-db";

let temp: TempDb;

afterEach(() => cleanup(temp));

function queueRun(db: TempDb["db"], sessionId: string, token: string, command = "us-refine"): string {
  const result = dispatchRun(db, sessionId, { command, story_id: "1.1", fix: true, client_token: token });
  if (result.kind !== "created") {
    throw new Error(`expected created, got ${result.kind}`);
  }

  return result.run_id;
}

function stateOf(db: TempDb["db"], runId: string): string {
  return (db.prepare("SELECT state FROM runs WHERE id = ?").get(runId) as { state: string }).state;
}

describe("run state machine", () => {
  it("walks the full lifecycle including answer_consumed", () => {
    temp = makeTempDb();
    const session = seedSession(temp.db);
    const run = queueRun(temp.db, session, "life");

    expect(stateOf(temp.db, run)).toBe("queued");

    // First output ⇒ claimed→running (proof of execution).
    appendEvents(temp.db, run, [{ seq: 1, type: "output", stream: "stdout", data: "working...\n" }]);
    expect(stateOf(temp.db, run)).toBe("running");

    // A prompt ⇒ prompt_published, pending prompt exposed.
    appendEvents(temp.db, run, [
      { seq: 2, type: "prompt", prompt_id: "p1", kind: "choice", payload: { question: "?" } },
    ]);
    expect(stateOf(temp.db, run)).toBe("prompt_published");
    expect(runDetail(temp.db, run, 0)?.pending_prompt?.prompt_id).toBe("p1");

    // Browser answers ⇒ answer_accepted (first wins).
    expect(submitAnswer(temp.db, run, "p1", "exit")).toBe("accepted");
    expect(stateOf(temp.db, run)).toBe("answer_accepted");

    // The remote confirms consumption (answers_consumed watermark) with
    // no new output ⇒ answer_consumed observable.
    const ack = appendEvents(temp.db, run, [], 1);
    expect(ack.answers_consumed_through).toBe(1);
    expect(stateOf(temp.db, run)).toBe("answer_consumed");

    // More output ⇒ back to running (the ⇄ cycle), then terminal.
    appendEvents(temp.db, run, [{ seq: 3, type: "output", stream: "stdout", data: "bye\n" }]);
    expect(stateOf(temp.db, run)).toBe("running");
    appendEvents(temp.db, run, [{ seq: 4, type: "terminal", outcome: "user_exit" }]);
    expect(stateOf(temp.db, run)).toBe("terminal");
    expect(runDetail(temp.db, run, 0)?.outcome).toBe("user_exit");
    expect(runDetail(temp.db, run, 0)?.pending_prompt).toBeNull();
  });

  it("applies the terminal transition exactly once (first terminal wins)", () => {
    temp = makeTempDb();
    const session = seedSession(temp.db);
    const run = queueRun(temp.db, session, "term");

    appendEvents(temp.db, run, [
      { seq: 1, type: "output", data: "x" },
      { seq: 2, type: "terminal", outcome: "converged" },
    ]);
    expect(runDetail(temp.db, run, 0)?.outcome).toBe("converged");

    // Redelivery of the same terminal (dedup) and a conflicting later
    // terminal both leave the first outcome untouched.
    appendEvents(temp.db, run, [{ seq: 2, type: "terminal", outcome: "converged" }]);
    appendEvents(temp.db, run, [{ seq: 3, type: "terminal", outcome: "error", error_detail: "spawn" }]);

    const detail = runDetail(temp.db, run, 0);
    expect(detail?.outcome).toBe("converged");
    expect(detail?.error_detail).toBeNull();

    // Event dedup: seq 1 and 2 are stored once each despite redelivery.
    const count = temp.db.prepare("SELECT COUNT(*) AS n FROM events WHERE run_id = ? AND seq IN (1,2)").get(run) as {
      n: number;
    };
    expect(count.n).toBe(2);
  });

  it("advances acked_through across a gap tombstone and dedups gap rows", () => {
    temp = makeTempDb();
    const session = seedSession(temp.db);
    const run = queueRun(temp.db, session, "gap");

    // A hole (seq 2 missing) stalls the watermark at 1.
    let ack = appendEvents(temp.db, run, [
      { seq: 1, type: "output", data: "a" },
      { seq: 3, type: "output", data: "c" },
    ]);
    expect(ack.acked_through).toBe(1);

    // A gap covering [2, 90] plus a contiguous terminal at 91 lets the
    // watermark reach the terminal seq.
    ack = appendEvents(temp.db, run, [
      { seq: 2, type: "gap", through_seq: 90, dropped_bytes: 4096 },
      { seq: 91, type: "terminal", outcome: "ok" },
    ]);
    expect(ack.acked_through).toBe(91);
    expect(stateOf(temp.db, run)).toBe("terminal");

    // Re-posting the gap is deduped by (run_id, seq).
    appendEvents(temp.db, run, [{ seq: 2, type: "gap", through_seq: 90, dropped_bytes: 4096 }]);
    const gaps = temp.db.prepare("SELECT COUNT(*) AS n FROM events WHERE run_id = ? AND type = 'gap'").get(run) as {
      n: number;
    };
    expect(gaps.n).toBe(1);
  });

  it("abandons older folder runs on a later run's lock_acquired PROOF; folder_locked abandons nothing", () => {
    // Finding 5: the authoritative flock proof is an explicit
    // `lock_acquired` event emitted immediately after AcquireFolderLock and
    // BEFORE spawn — output/prompt no longer stand in for lock proof, so a
    // run that took the lock and then failed spawn still abandons a stale
    // older run.
    temp = makeTempDb();
    const folder = "/host/exclusive";
    const sessionA = seedSession(temp.db, folder);
    const sessionB = seedSession(temp.db, folder);

    // runA on A proves lock acquisition (no older runs yet), then streams.
    const runA = queueRun(temp.db, sessionA, "A");
    appendEvents(temp.db, runA, [
      { seq: 1, type: "lock_acquired" },
      { seq: 2, type: "output", data: "holding lock\n" },
    ]);
    expect(stateOf(temp.db, runA)).toBe("running");

    // runB on B fails to acquire the flock: only a terminal(error,
    // folder_locked) event, NO lock_acquired ⇒ runA is NOT abandoned.
    const runB = queueRun(temp.db, sessionB, "B");
    appendEvents(temp.db, runB, [{ seq: 1, type: "terminal", outcome: "error", error_detail: "folder_locked" }]);
    expect(stateOf(temp.db, runB)).toBe("terminal");
    expect(stateOf(temp.db, runA)).toBe("running");

    // Plain output alone never proves the lock: a stray output on a fresh
    // run on B does not abandon runA.
    const runOutputOnly = queueRun(temp.db, sessionB, "OUT");
    appendEvents(temp.db, runOutputOnly, [{ seq: 1, type: "output", data: "no lock proof\n" }]);
    expect(stateOf(temp.db, runA)).toBe("running");
    // Terminate it so B's session slot frees for runC.
    appendEvents(temp.db, runOutputOnly, [{ seq: 2, type: "terminal", outcome: "ok" }]);

    // runC on B acquires the freed flock (lock_acquired) even though it then
    // fails spawn (terminal error) ⇒ the stale older runA flips abandoned.
    const runC = queueRun(temp.db, sessionB, "C");
    appendEvents(temp.db, runC, [
      { seq: 1, type: "lock_acquired" },
      { seq: 2, type: "terminal", outcome: "error", error_detail: "spawn" },
    ]);
    expect(runDetail(temp.db, runC, 0)?.outcome).toBe("error");

    const abandoned = runDetail(temp.db, runA, 0);
    expect(abandoned?.state).toBe("terminal");
    expect(abandoned?.outcome).toBe("abandoned");
  });

  it("flags folder-wide want_inventory when any folder run reaches terminal", () => {
    temp = makeTempDb();
    const folder = "/host/autorefresh";
    const sessionA = seedSession(temp.db, folder);
    const sessionB = seedSession(temp.db, folder);
    const run = queueRun(temp.db, sessionA, "R", "version");

    appendEvents(temp.db, run, [
      { seq: 1, type: "output", data: "true-bdd version 1.0\n" },
      { seq: 2, type: "terminal", outcome: "ok" },
    ]);

    // The idle sibling B's next poll is told to re-inventory.
    const pollB = pollSession(temp.db, sessionB, undefined);
    expect(pollB?.want_inventory).toBe(true);
    expect(pollB?.scan_epoch).toBeGreaterThan(0);

    // Sanity: A's run history shows the terminal ok run.
    expect(sessionDetail(temp.db, sessionA)?.runs.find((r) => r.id === run)?.outcome).toBe("ok");
  });
});
