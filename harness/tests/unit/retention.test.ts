/**
 * Real-SQLite: retention & bounds (plan §3.6 server side / §4.6).
 *   - per-run byte budget ⇒ HEAD + rolling TAIL, middle collapsed into a
 *     synthesized gap marker; contiguous acked_through still reaches the
 *     terminal seq;
 *   - prune terminal runs beyond last N/folder (the 50-run cap);
 *   - startup + periodic pruning via startRetention;
 *   - the budget/keep/interval knobs are env-tunable.
 */

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { retentionConfig } from "../../app/lib/retention";
import { startRetention } from "../../app/lib/retention-runtime";
import { appendEvents, dispatchRun, enforceByteBudget, pruneTerminalRuns } from "../../app/lib/store";
import { cleanup, makeTempDb, seedSession, type TempDb } from "./support/temp-db";

let temp: TempDb;

afterEach(() => cleanup(temp));

function terminalRun(db: TempDb["db"], session: string, token: string): string {
  const created = dispatchRun(db, session, { command: "version", fix: false, client_token: token });
  if (created.kind !== "created") {
    throw new Error(`expected created, got ${created.kind}`);
  }
  appendEvents(db, created.run_id, [
    { seq: 1, type: "output", data: "true-bdd version 1.0\n" },
    { seq: 2, type: "terminal", outcome: "ok" },
  ]);

  return created.run_id;
}

describe("per-run byte budget", () => {
  it("collapses the middle into a gap marker while keeping head, tail, and acked_through", () => {
    temp = makeTempDb();
    const session = seedSession(temp.db);
    const created = dispatchRun(temp.db, session, { command: "us-refine", story_id: "1.1", fix: true, client_token: "bb" });
    const run = created.kind === "created" ? created.run_id : "";

    // 20 large contiguous output events (seq 1..20) + terminal at 21 ⇒
    // acked_through 21, well over a tiny byte budget.
    const events = Array.from({ length: 20 }, (_, index) => ({
      seq: index + 1,
      type: "output",
      stream: "stdout",
      data: "x".repeat(200),
    }));
    events.push({ seq: 21, type: "terminal", stream: "", data: "" });
    const ack = appendEvents(temp.db, run, events);
    expect(ack.acked_through).toBe(21);

    enforceByteBudget(temp.db, run, { byteBudget: 300, head: 2, tail: 2 });

    const rows = temp.db.prepare("SELECT seq, type, data FROM events WHERE run_id = ? ORDER BY seq").all(run) as {
      seq: number;
      type: string;
      data: string;
    }[];
    const gaps = rows.filter((row) => row.type === "gap");
    expect(gaps).toHaveLength(1);

    // Head (seq 1,2) and tail (seq 20,21) survive; the gap covers the
    // deleted middle [3, 19].
    expect(rows.map((row) => row.seq)).toContain(1);
    expect(rows.map((row) => row.seq)).toContain(2);
    expect(rows.map((row) => row.seq)).toContain(20);
    expect(rows.map((row) => row.seq)).toContain(21);
    expect(JSON.parse(gaps[0].data).through_seq).toBe(19);
    // The middle output rows are gone.
    expect(rows.some((row) => row.seq > 3 && row.seq < 20)).toBe(false);

    // The synthesized gap keeps the watermark whole: acked_through == 21.
    const detail = appendEvents(temp.db, run, []);
    expect(detail.acked_through).toBe(21);
  });
});

describe("terminal-run pruning (per-folder cap)", () => {
  it("keeps only the newest N terminal runs per folder and drops their events", () => {
    temp = makeTempDb();
    const folder = "/host/cap";
    const session = seedSession(temp.db, folder);

    const runs: string[] = [];
    for (let index = 0; index < 55; index += 1) {
      runs.push(terminalRun(temp.db, session, `k${index}`));
    }

    pruneTerminalRuns(temp.db, 50);

    const remaining = temp.db
      .prepare(
        "SELECT COUNT(*) AS n FROM runs r JOIN sessions s ON s.id = r.session_id WHERE s.canonical_folder = ?",
      )
      .get(folder) as { n: number };
    expect(remaining.n).toBe(50);

    // The 5 oldest runs (and their events) are gone.
    const oldest = runs[0];
    expect(temp.db.prepare("SELECT COUNT(*) AS n FROM runs WHERE id = ?").get(oldest)).toEqual({ n: 0 });
    expect(temp.db.prepare("SELECT COUNT(*) AS n FROM events WHERE run_id = ?").get(oldest)).toEqual({ n: 0 });
    // The newest run survives.
    expect(temp.db.prepare("SELECT COUNT(*) AS n FROM runs WHERE id = ?").get(runs[54])).toEqual({ n: 1 });
  });
});

describe("startRetention (startup + periodic) and env-tunable knobs", () => {
  const savedEnv = { ...process.env };

  beforeEach(() => {
    process.env.TRUE_BDD_HARNESS_KEEP_RUNS = "2";
    process.env.TRUE_BDD_HARNESS_PRUNE_INTERVAL_MS = "1000";
    process.env.TRUE_BDD_HARNESS_BYTE_BUDGET = "4096";
  });

  afterEach(() => {
    process.env = { ...savedEnv };
    vi.useRealTimers();
  });

  it("reads its knobs from the environment", () => {
    const cfg = retentionConfig();
    expect(cfg.keepPerFolder).toBe(2);
    expect(cfg.pruneIntervalMs).toBe(1000);
    expect(cfg.byteBudget).toBe(4096);
  });

  it("prunes at startup and again on the periodic timer", () => {
    vi.useFakeTimers();
    temp = makeTempDb();
    const folder = "/host/timer";
    const session = seedSession(temp.db, folder);

    for (let index = 0; index < 4; index += 1) {
      terminalRun(temp.db, session, `s${index}`);
    }

    // Startup prune keeps 2.
    startRetention(temp.db);
    const afterStartup = temp.db
      .prepare("SELECT COUNT(*) AS n FROM runs r JOIN sessions s ON s.id = r.session_id WHERE s.canonical_folder = ?")
      .get(folder) as { n: number };
    expect(afterStartup.n).toBe(2);

    // Two more terminal runs, then the periodic timer fires and re-prunes.
    terminalRun(temp.db, session, "s4");
    terminalRun(temp.db, session, "s5");
    vi.advanceTimersByTime(1000);
    const afterTimer = temp.db
      .prepare("SELECT COUNT(*) AS n FROM runs r JOIN sessions s ON s.id = r.session_id WHERE s.canonical_folder = ?")
      .get(folder) as { n: number };
    expect(afterTimer.n).toBe(2);
  });
});
