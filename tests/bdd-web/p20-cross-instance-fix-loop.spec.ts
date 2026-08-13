/**
 * P20 (plan: connect-cli-to-vercel-harness) — the --fix workflow across instances.
 *
 * Browser reads/mutations pinned to A, CLI poll/reply pinned to B (the real
 * remote runs against server B), sharing one Redis. p17 only exercises the
 * `version` command; this spec covers the Product --fix requirement cross-
 * instance by driving the hidden deterministic `prompt-probe` driver
 * (choice → clarify → freetext, no `claude`). The real-CLI fix-loop end-to-end
 * behavior stays in the `ai` suite.
 *
 * Consume-once oracle: each answer advances the run to the NEXT prompt kind
 * exactly once, and a re-submitted identical answer is accepted first-wins
 * (P13-style idempotency) — observable through the run state on A. Under the
 * in-process relay the cross-instance rendezvous NEVER happens (the remote
 * registered on B is invisible on A; the dispatch on A 404s), so every
 * assertion fails until the Redis-backed relay lands.
 */

import { expect, test } from "@playwright/test";

import { newRunToken, pollUntil, STATUS, type RunDetail, type PendingPrompt } from "./helpers/api-client";
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

const PROMPT_TIMEOUT_MS = 30_000;

/** Waits for a pending prompt of the given kind (optionally distinct from `notId`). */
async function waitForPrompt(
  api: CrossInstanceEnv["apiA"],
  sessionId: string,
  runId: string,
  kind: PendingPrompt["kind"],
  notId?: string,
): Promise<PendingPrompt> {
  const run = await pollUntil(
    async () => {
      const candidate = await api.getRun(sessionId, runId);
      if (candidate.state === "terminal") {
        throw new Error(`run ${runId} went terminal before a ${kind} prompt appeared`);
      }
      const pending = candidate.pending_prompt;
      if (pending === null || pending.prompt_id === notId) {
        return undefined;
      }
      return candidate;
    },
    { timeoutMs: PROMPT_TIMEOUT_MS, what: `run ${runId} to publish a ${kind} prompt` },
  );
  if (run.pending_prompt?.kind !== kind) {
    throw new Error(`expected a ${kind} prompt, got ${run.pending_prompt?.kind}`);
  }
  return run.pending_prompt;
}

test("P20: a prompt-probe fix loop drives choice → clarify → freetext across instances", async () => {
  env = await CrossInstanceEnv.start("p20-fix-loop");
  const e = env;
  const fixture = await e.materialize("bare-host");

  // The remote registers against server B. It is visible on A only via Redis.
  const remote = await e.startRemoteOnB(fixture.target);
  const session = await pollUntil(
    async () => (await e.apiA.listSessions()).find((candidate) => candidate.pid === remote.pid),
    { timeoutMs: 30_000, what: "the B-registered remote to appear in GET /api/sessions on A" },
  );
  const sessionId = session.session_id;

  const token = newRunToken();
  // fix: true — this spec covers the Product --fix requirement cross-instance
  // (Codex test-author r1): the run genuinely represents a fix run, not just a
  // prompt-probe dispatch. Assert the run record carries fix === true.
  const dispatch = await e.apiA.dispatchRun(sessionId, { command: "prompt-probe", fix: true, client_token: token });
  expect(dispatch.status).toBe(STATUS.created);
  const runId = dispatch.runId;
  const fixRun = await e.apiA.getRun(sessionId, runId);
  expect(fixRun.fix).toBe(true);

  // ── choice ──
  const choice = await waitForPrompt(e.apiA, sessionId, runId, "choice");
  const choiceAnswer = await e.apiA.answerRunResponse(sessionId, runId, { prompt_id: choice.prompt_id, value: "1" });
  expect(choiceAnswer.status()).toBe(STATUS.ok);

  // A re-submitted IDENTICAL answer is accepted first-wins (consume-once).
  const choiceRedo = await e.apiA.answerRunResponse(sessionId, runId, { prompt_id: choice.prompt_id, value: "1" });
  expect(choiceRedo.status()).toBe(STATUS.ok);

  // ── clarify (advanced exactly once past the choice prompt) ──
  const clarify = await waitForPrompt(e.apiA, sessionId, runId, "clarify", choice.prompt_id);
  const clarifyAnswer = await e.apiA.answerRunResponse(sessionId, runId, { prompt_id: clarify.prompt_id, value: "1" });
  expect(clarifyAnswer.status()).toBe(STATUS.ok);

  // ── freetext ──
  const freetext = await waitForPrompt(e.apiA, sessionId, runId, "freetext", clarify.prompt_id);
  const freetextAnswer = await e.apiA.answerRunResponse(sessionId, runId, {
    prompt_id: freetext.prompt_id,
    value: "cross-instance feedback\n",
  });
  expect(freetextAnswer.status()).toBe(STATUS.ok);

  // The run leaves the freetext prompt (terminal or cleared) — consume-once
  // across all three kinds, every step served cross-instance via Redis.
  await pollUntil(
    async () => {
      const run = (await e.apiA.getRun(sessionId, runId)) as RunDetail;
      return run.pending_prompt?.prompt_id === freetext.prompt_id ? undefined : run;
    },
    { timeoutMs: PROMPT_TIMEOUT_MS, what: `run ${runId} to leave the freetext prompt` },
  );

  // Strong terminal assertion (Codex review r1 #7): leaving the freetext prompt
  // is necessary but not sufficient — a run that errored out or dropped the
  // prompt without processing the answer would also satisfy the predicate
  // above. Pin the SUCCESS outcome: the run reaches `terminal` with `outcome
  // === "ok"`, proving all three answers were accepted and the fix loop drove
  // the run to a clean completion cross-instance via Redis.
  const terminal = await pollUntil(
    async () => {
      const run = (await e.apiA.getRun(sessionId, runId)) as RunDetail;
      return run.state === "terminal" ? run : undefined;
    },
    { timeoutMs: PROMPT_TIMEOUT_MS, what: `run ${runId} to reach terminal after the full fix loop` },
  );
  expect(terminal.outcome, "the full fix loop must complete ok cross-instance").toBe("ok");
});
