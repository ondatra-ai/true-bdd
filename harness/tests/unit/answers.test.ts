/**
 * Real-SQLite: browser answers (plan §3.3/§3.2 handoff d / §4.6).
 *   - first-wins durable; exact retry ⇒ accepted; conflicting ⇒ conflict;
 *   - validation: choice ∈ {apply,refine,exit,1,2,3}; clarify single-line
 *     (bounded); freetext ≤ 64KB; unknown run/prompt handled;
 *   - a consumed answer is no longer re-delivered on poll.
 */

import { afterEach, describe, expect, it } from "vitest";

import { appendEvents, dispatchRun, pollSession, submitAnswer } from "../../app/lib/store";
import { cleanup, makeTempDb, seedSession, type TempDb } from "./support/temp-db";

let temp: TempDb;

afterEach(() => cleanup(temp));

/** Queues a run and publishes one prompt of `kind`; returns ids + seqs. */
function runWithPrompt(db: TempDb["db"], sessionId: string, kind: string, token: string): { run: string; prompt: string } {
  const created = dispatchRun(db, sessionId, { command: "us-refine", story_id: "1.1", fix: true, client_token: token });
  if (created.kind !== "created") {
    throw new Error(`expected created, got ${created.kind}`);
  }
  const run = created.run_id;
  const prompt = `prompt-${token}`;
  appendEvents(db, run, [
    { seq: 1, type: "output", data: "..." },
    { seq: 2, type: "prompt", prompt_id: prompt, kind, payload: { question: "q", options: ["one", "two", "three"] } },
  ]);

  return { run, prompt };
}

describe("browser answers", () => {
  it("is first-wins: exact retry accepted, conflicting rejected, value durable", () => {
    temp = makeTempDb();
    const session = seedSession(temp.db);
    const { run, prompt } = runWithPrompt(temp.db, session, "choice", "fw");

    expect(submitAnswer(temp.db, run, prompt, "exit")).toBe("accepted");
    expect(submitAnswer(temp.db, run, prompt, "exit")).toBe("accepted"); // exact retry
    expect(submitAnswer(temp.db, run, prompt, "apply")).toBe("conflict"); // conflicting

    const stored = temp.db.prepare("SELECT value FROM answers WHERE run_id = ? AND prompt_id = ?").get(run, prompt) as {
      value: string;
    };
    expect(stored.value).toBe("exit");
  });

  it("validates the choice enum (apply/refine/exit or 1/2/3)", () => {
    temp = makeTempDb();
    const session = seedSession(temp.db);
    const { run, prompt } = runWithPrompt(temp.db, session, "choice", "ce");

    expect(submitAnswer(temp.db, run, prompt, "2")).toBe("accepted"); // numeric equivalent
    // A conflicting-but-VALID second answer is a conflict, not invalid.
    expect(submitAnswer(temp.db, run, prompt, "refine")).toBe("conflict");
    // Invalid values never reach conflict/accept (fresh session — the
    // first run is still non-terminal, so it holds this session's slot).
    const session2 = seedSession(temp.db);
    const { run: r2, prompt: p2 } = runWithPrompt(temp.db, session2, "choice", "ce2");
    expect(submitAnswer(temp.db, r2, p2, "bogus")).toBe("invalid");
    expect(submitAnswer(temp.db, r2, p2, "exit\n")).toBe("invalid");
  });

  it("validates a single-line clarify answer", () => {
    temp = makeTempDb();
    const session = seedSession(temp.db);
    const { run, prompt } = runWithPrompt(temp.db, session, "clarify", "cl");

    expect(submitAnswer(temp.db, run, prompt, "with\nnewline")).toBe("invalid");
    expect(submitAnswer(temp.db, run, prompt, "")).toBe("invalid");
    expect(submitAnswer(temp.db, run, prompt, "x".repeat(4097))).toBe("invalid");
    expect(submitAnswer(temp.db, run, prompt, "2")).toBe("accepted");
  });

  it("validates freetext size (≤ 64KB, non-empty; newlines allowed)", () => {
    temp = makeTempDb();
    const session = seedSession(temp.db);
    const { run, prompt } = runWithPrompt(temp.db, session, "freetext", "ft");

    expect(submitAnswer(temp.db, run, prompt, "")).toBe("invalid");
    expect(submitAnswer(temp.db, run, prompt, "x".repeat(64 * 1024 + 1))).toBe("invalid");
    expect(submitAnswer(temp.db, run, prompt, "multi\nline\nfeedback\n")).toBe("accepted");
  });

  it("handles unknown run and unknown prompt", () => {
    temp = makeTempDb();
    const session = seedSession(temp.db);
    const { run } = runWithPrompt(temp.db, session, "choice", "unk");

    expect(submitAnswer(temp.db, "no-such-run", "p", "exit")).toBe("not_found");
    expect(submitAnswer(temp.db, run, "no-such-prompt", "exit")).toBe("invalid");
    expect(submitAnswer(temp.db, run, "p", 42)).toBe("invalid");
  });

  it("preserves an EXACT retry of an already-accepted answer after the run goes terminal (finding 6)", () => {
    temp = makeTempDb();
    const session = seedSession(temp.db);
    const { run, prompt } = runWithPrompt(temp.db, session, "choice", "exact");

    expect(submitAnswer(temp.db, run, prompt, "exit")).toBe("accepted");
    // The run reaches terminal after the answer was durably accepted.
    appendEvents(temp.db, run, [{ seq: 3, type: "terminal", outcome: "user_exit" }]);

    // An EXACT retry of the already-accepted answer stays 200/accepted …
    expect(submitAnswer(temp.db, run, prompt, "exit")).toBe("accepted");
    // … while a CONFLICTING retry is a conflict, never a new insert.
    expect(submitAnswer(temp.db, run, prompt, "apply")).toBe("conflict");
    const count = temp.db.prepare("SELECT COUNT(*) AS n FROM answers WHERE run_id = ?").get(run) as { n: number };
    expect(count.n).toBe(1);
  });

  it("rejects an answer to a STALE (superseded) pending prompt (finding 6)", () => {
    temp = makeTempDb();
    const session = seedSession(temp.db);
    const { run } = runWithPrompt(temp.db, session, "choice", "stale");

    // A second prompt supersedes the first as the current pending prompt.
    appendEvents(temp.db, run, [
      { seq: 3, type: "prompt", prompt_id: "p2", kind: "choice", payload: { actions: ["exit"] } },
    ]);

    // Answering the OLD prompt (prompt-stale) is a conflict; the current one
    // is accepted.
    expect(submitAnswer(temp.db, run, "prompt-stale", "exit")).toBe("conflict");
    expect(submitAnswer(temp.db, run, "p2", "exit")).toBe("accepted");
    const stored = temp.db.prepare("SELECT prompt_id FROM answers WHERE run_id = ?").all(run) as { prompt_id: string }[];
    expect(stored.map((row) => row.prompt_id)).toEqual(["p2"]);
  });

  it("stops re-delivering an answer once the remote reports it consumed", () => {
    temp = makeTempDb();
    const session = seedSession(temp.db);
    const { run, prompt } = runWithPrompt(temp.db, session, "choice", "cons");

    expect(submitAnswer(temp.db, run, prompt, "exit")).toBe("accepted");

    // Before consumption the poll delivers the accepted answer.
    const before = pollSession(temp.db, session, run);
    expect(before?.answer?.prompt_id).toBe(prompt);
    expect(before?.answer?.answer_seq).toBe(1);

    // The remote confirms consumption (answers_consumed = 1).
    const ack = appendEvents(temp.db, run, [], 1);
    expect(ack.answers_consumed_through).toBe(1);

    // Now the poll delivers NO answer (redelivery is ack-only on the remote).
    const after = pollSession(temp.db, session, run);
    expect(after?.answer).toBeUndefined();
  });
});
