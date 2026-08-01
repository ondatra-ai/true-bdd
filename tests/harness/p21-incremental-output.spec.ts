/**
 * P21 (plan: connect-cli-to-vercel-harness) — output streams before completion.
 *
 * Browser reads pinned to A, CLI poll/reply pinned to B (real remote on B),
 * sharing one Redis. Uses `prompt-probe`'s OWN prompt as the deterministic
 * barrier (Codex r2 #8 — a sleep-based barrier flakes; the published prompt is
 * a real sync point): while the run is still ACTIVE (non-terminal, a pending
 * prompt present) the pre-answer output is already visible in the run detail on
 * A; after the answer, further output appears and the run advances.
 *
 * Under the in-process relay the cross-instance rendezvous never happens (the
 * dispatch on A 404s — the remote registered on B is invisible on A), so the
 * test fails before any output could be served. Under Redis, run_detail reads
 * stream incremental output from the CLI store across the instance boundary.
 */

import { expect, test } from "@playwright/test";

import { newRunToken, pollUntil, STATUS, type RunDetail } from "./helpers/api-client";
import { CrossInstanceEnv } from "./helpers/cross-instance-env";

let env: CrossInstanceEnv | undefined;

test.afterEach(async () => {
  const info = test.info();
  info.setTimeout(info.timeout + 60_000);
  const current = env;
  env = undefined;
  if (current !== undefined) {
    await current.teardown(test.info());
  }
});

test("P21: run detail on A shows active output before the prompt is answered (cross-instance)", async () => {
  env = await CrossInstanceEnv.start("p21-incremental-output");
  const e = env;
  const fixture = await e.materialize("bare-host");

  const remote = await e.startRemoteOnB(fixture.target);
  const session = await pollUntil(
    async () => (await e.apiA.listSessions()).find((candidate) => candidate.pid === remote.pid),
    { timeoutMs: 30_000, what: "the B-registered remote to appear in GET /api/sessions on A" },
  );
  const sessionId = session.session_id;

  const token = newRunToken();
  const dispatch = await e.apiA.dispatchRun(sessionId, { command: "prompt-probe", fix: false, client_token: token });
  expect(dispatch.status).toBe(STATUS.created);
  const runId = dispatch.runId;

  // ── Read 1: the run is served on A WHILE STILL ACTIVE ──
  // The published prompt is the deterministic barrier; the run_detail read
  // resolves with a NON-terminal run carrying a pending prompt AND an output
  // event already present — i.e. output is streamed before completion, not
  // buffered until terminal. The output condition is part of the poll
  // predicate (Codex review r2 #4): the prompt and output streams are served
  // by independent readers, so checking output AFTER the poll races — under
  // load the prompt could land before the output event and the post-poll
  // assertion would observe zero output. Requiring both in the predicate makes
  // the barrier self-stabilizing.
  const active = await pollUntil(
    async () => {
      const run = (await e.apiA.getRun(sessionId, runId)) as RunDetail;
      if (run.state === "terminal" || run.pending_prompt === null) {
        return undefined;
      }
      const hasOutput = run.events.some((event) => event.type === "output");
      return hasOutput ? run : undefined;
    },
    { timeoutMs: 30_000, what: `run ${runId} to be served ACTIVE with a pending prompt AND an output event on A` },
  );
  expect(active.state).not.toBe("terminal");
  expect(active.pending_prompt).not.toBeNull();
  const promptBefore = active.pending_prompt?.prompt_id as string;

  // Output is streamed BEFORE completion: an explicit output event (chunk A) is
  // already present while the run is still ACTIVE — not just any event (a prompt
  // or state-transition event would not prove streaming). The CLI emits output
  // as type === "output" records (run_executor.go); assert that exact type.
  const outputBefore = active.events.filter((event) => event.type === "output");
  expect(outputBefore.length, "pre-prompt output chunk streamed while the run is ACTIVE").toBeGreaterThan(0);
  const maxOutputSeqBefore = Math.max(...outputBefore.map((event) => event.seq));

  // ── Answer the prompt; the run advances and MORE output appears ──
  const answer = await e.apiA.answerRunResponse(sessionId, runId, { prompt_id: promptBefore, value: "1" });
  expect(answer.status()).toBe(STATUS.ok);

  const advanced = await pollUntil(
    async () => {
      const run = (await e.apiA.getRun(sessionId, runId)) as RunDetail;
      // Either the prompt advanced (new pending prompt) or the run went terminal;
      // either way the post-answer output chunk must have landed. Require BOTH
      // the prompt-advance AND a new output seq above the pre-answer max in the
      // SAME poll (Codex review r2 #4): the prompt and output streams are
      // independent, so checking output after the poll-return races.
      const promptChanged = run.pending_prompt === null || run.pending_prompt.prompt_id !== promptBefore;
      if (!promptChanged) return undefined;
      const hasNewOutput = run.events.some(
        (event) => event.type === "output" && event.seq > maxOutputSeqBefore,
      );
      return hasNewOutput ? run : undefined;
    },
    { timeoutMs: 30_000, what: `run ${runId} to advance past the answered prompt AND surface a new output event on A` },
  );

  // The post-answer read observed a NEW output event (chunk B) with a seq above
  // the pre-answer max — proving the relay streams run_detail mid-flight, cross-
  // instance (not merely that the event window grew on a non-output event).
  const newOutput = advanced.events.filter((event) => event.type === "output" && event.seq > maxOutputSeqBefore);
  expect(newOutput.length, "post-answer output chunk streamed after the prompt barrier").toBeGreaterThan(0);
});
