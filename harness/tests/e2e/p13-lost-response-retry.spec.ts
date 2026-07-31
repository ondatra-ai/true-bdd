/**
 * P13 (plan §1.4, NEW for v2) — lost HTTP response ⇒ CLI-side exact-retry.
 *
 * A browser mutation whose HTTP RESPONSE is lost (network blip after the
 * CLI committed) is retried by the client. The CLI store's idempotency
 * (dispatch receipts keyed by (owner_id, client_token); first-wins answers)
 * makes the retry EXACT — the same run id, the answer applied once — so a
 * lost response never double-dispatches or double-answers (critique §6).
 *
 * A genuinely-dropped response is indistinguishable, at the client, from a
 * re-submitted identical request; this spec proves the exact-retry contract
 * by re-submitting the identical dispatch and answer.
 *
 * Uses `prompt-probe` (deterministic prompts, no `claude`).
 */

import { expect, test } from "@playwright/test";

import { newRunToken, STATUS } from "./helpers/api-client";
import { ProtocolEnv } from "./helpers/protocol-env";

let env: ProtocolEnv | undefined;

test.afterEach(async () => {
  const info = test.info();
  info.setTimeout(info.timeout + 60_000);
  const current = env;
  env = undefined;

  if (current !== undefined) {
    await current.teardown(test.info());
  }
});

test("P13: a re-submitted dispatch and answer are EXACT retries (idempotent CLI store)", async () => {
  env = await ProtocolEnv.start("p13-lost-response");
  const e = env;

  const fixture = await e.materialize("bare-host");
  const remote = await e.startRemote(fixture.target);
  const session = await e.api.waitForSession((candidate) => candidate.pid === remote.pid);
  e.note({ sessionId: session.session_id });

  // ── Lost dispatch response ⇒ exact retry returns the ORIGINAL run ──
  const token = newRunToken();
  const first = await e.api.dispatchRun(session.session_id, {
    command: "prompt-probe",
    fix: false,
    client_token: token,
  });
  expect(first.status).toBe(STATUS.created);
  e.note({ runId: first.runId });

  // The "lost response" retry: same token → the SAME run id, status 200
  // (dedup) — never a second run.
  const retry = await e.api.dispatchRun(session.session_id, {
    command: "prompt-probe",
    fix: false,
    client_token: token,
  });
  expect(retry.runId).toBe(first.runId);
  expect(retry.status).toBe(STATUS.ok);

  // Reach the first (choice) prompt.
  const atPrompt = await e.api.waitForRun(session.session_id, first.runId, (run) => run.pending_prompt !== null, {
    timeoutMs: 30_000,
  });
  const promptId = atPrompt.pending_prompt?.prompt_id as string;

  // ── Lost answer response ⇒ exact retry stays accepted; conflict rejected ──
  const answer1 = await e.api.answerRunResponse(session.session_id, first.runId, { prompt_id: promptId, value: "1" });
  expect(answer1.status()).toBe(STATUS.ok);

  // The "lost response" retry of the SAME answer is accepted (first-wins,
  // exact retry) — not a new insert, not a conflict.
  const answerRetry = await e.api.answerRunResponse(session.session_id, first.runId, { prompt_id: promptId, value: "1" });
  expect(answerRetry.status()).toBe(STATUS.ok);

  // A CONFLICTING answer to the already-answered prompt is a 409.
  const conflicting = await e.api.answerRunResponse(session.session_id, first.runId, { prompt_id: promptId, value: "2" });
  expect(conflicting.status()).toBe(STATUS.conflict);

  // The answer was consumed EXACTLY ONCE: the run advanced past the first
  // prompt to the next one (a distinct prompt id).
  const advanced = await e.api.waitForRun(
    session.session_id,
    first.runId,
    (run) => run.pending_prompt !== null && run.pending_prompt.prompt_id !== promptId,
    { timeoutMs: 30_000 },
  );
  expect(advanced.pending_prompt?.prompt_id).not.toBe(promptId);
});
