/**
 * P16 (plan §1.1, NEW for v2) — cross-owner answer guard. Two remotes share
 * one project (one CLI store). Project-wide run READS are allowed through
 * any live session, but answer acceptance/delivery requires
 * `run.owner_id == this CLI's owner_id` AND a matching local active
 * executor/prompt. Other owners' runs are READ-ONLY through this session.
 *
 * Asserts: B's run is readable via A (answerable=false); an answer via A
 * changes no row and no stdin (rejected); the same answer via B succeeds
 * (answerable=true) and advances the run.
 *
 * Uses `prompt-probe` (deterministic prompts, no `claude`).
 */

import { expect, test } from "@playwright/test";

import { newRunToken, STATUS } from "./helpers/api-client";
import { ProtocolEnv } from "./helpers/protocol-env";

let env: ProtocolEnv | undefined;

test.afterEach(async () => {
  const info = test.info();
  info.setTimeout(info.timeout + 90_000);
  const current = env;
  env = undefined;

  if (current !== undefined) {
    await current.teardown(test.info());
  }
});

test("P16: B's run is read-only via A; answer-via-A rejected changing nothing; answer-via-B succeeds", async () => {
  env = await ProtocolEnv.start("p16-cross-owner-answer");
  const e = env;

  const fixture = await e.materialize("bare-host");

  const remoteA = await e.startRemote(fixture.target);
  const sessionA = await e.api.waitForSession((candidate) => candidate.pid === remoteA.pid);
  const remoteB = await e.startRemote(fixture.target);
  const sessionB = await e.api.waitForSession((candidate) => candidate.pid === remoteB.pid);
  expect(sessionB.session_id).not.toBe(sessionA.session_id);
  e.note({ sessionId: sessionB.session_id });

  // Owner B holds a prompt-probe run blocked at its first prompt.
  const runB = await e.api.dispatchRun(sessionB.session_id, {
    command: "prompt-probe",
    fix: false,
    client_token: newRunToken(),
  });
  e.note({ runId: runB.runId });
  const atPrompt = await e.api.waitForRun(sessionB.session_id, runB.runId, (run) => run.pending_prompt !== null, {
    timeoutMs: 30_000,
  });
  const promptId = atPrompt.pending_prompt?.prompt_id as string;

  // B's run is READABLE through A (project-wide reads), but NOT answerable
  // there — the cross-owner guard marks it read-only for A.
  const viaA = await e.api.getRun(sessionA.session_id, runB.runId);
  expect(viaA.run_id).toBe(runB.runId);
  expect(viaA.answerable).toBe(false);

  const viaB = await e.api.getRun(sessionB.session_id, runB.runId);
  expect(viaB.answerable).toBe(true);

  // An answer routed through A is REJECTED (read-only through this session)
  // and changes nothing: the run stays at the same pending prompt.
  const answerViaA = await e.api.answerRunResponse(sessionA.session_id, runB.runId, {
    prompt_id: promptId,
    value: "1",
  });
  expect(answerViaA.status()).toBe(STATUS.conflict);

  const unchanged = await e.api.getRun(sessionB.session_id, runB.runId);
  expect(unchanged.pending_prompt?.prompt_id).toBe(promptId);
  expect(unchanged.state).not.toBe("terminal");

  // The same answer routed through the OWNER (B) succeeds and advances.
  const answerViaB = await e.api.answerRunResponse(sessionB.session_id, runB.runId, {
    prompt_id: promptId,
    value: "1",
  });
  expect(answerViaB.status()).toBe(STATUS.ok);

  const advanced = await e.api.waitForRun(
    sessionB.session_id,
    runB.runId,
    (run) => run.state === "terminal" || (run.pending_prompt !== null && run.pending_prompt.prompt_id !== promptId),
    { timeoutMs: 30_000 },
  );
  expect(advanced.pending_prompt?.prompt_id ?? "terminal").not.toBe(promptId);
});
