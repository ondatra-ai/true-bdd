/**
 * A3 (plan §4.3) — clarify round, engineered. Two obviously-duplicate
 * malformed acceptance criteria + the rule-based-description prompt: the
 * shipped fix-generator system prompt forces a clarifying question
 * before merging duplicates. The clarify prompt renders NUMBERED
 * options; A3 answers by NUMBER and asserts the recorded answer equals
 * that OPTION'S TEXT (the engine collector maps the number to the option
 * string and echoes "Recorded: <text>"). The run ends deterministically
 * with Exit.
 *
 * Oracle (plan §4.5): NO canonical change — Exit applies nothing.
 */

import { expect, test } from "@playwright/test";

import { newRunToken } from "./helpers/api-client";
import { ClaudeCallBudget } from "./helpers/claude-budget";
import { skipUnlessClaudeAvailable } from "./helpers/claude-gate";
import { AI_TERMINAL_TIMEOUT_MS, clickChoice, submitClarifyNumber, waitForPromptKind } from "./helpers/fix-loop";
import { ProtocolEnv } from "./helpers/protocol-env";
import { TID, clarifyOption, gotoRun } from "./helpers/ui";
import { expectNoCanonicalChange } from "./helpers/tree-hash";

const AI_CALL_BUDGET = 8;

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

test("A3: duplicate ACs force a clarify prompt; answer by number records the option text", async ({ page }) => {
  skipUnlessClaudeAvailable(test);

  env = await ProtocolEnv.start("a3-clarify");
  const e = env;

  const fixture = await e.materialize("a3-clarify");
  const remote = await e.startRemote(fixture.target);
  const session = await e.api.waitForSession((candidate) => candidate.pid === remote.pid);
  e.note({ sessionId: session.id });
  await e.api.waitForGeneration(session.id, 0);

  const budget = new ClaudeCallBudget(remote.pid).start();

  const { runId } = await e.api.dispatchRun(session.id, {
    command: "us-refine",
    story_id: "44.1",
    fix: true,
    client_token: newRunToken(),
  });
  e.note({ runId });

  await gotoRun(page, e.server.baseURL, runId);

  // The fix-generator asks a clarifying question with numbered options.
  const clarify = await waitForPromptKind(e.api, runId, "clarify");
  const clarifyId = clarify.prompt.prompt_id;

  const optionOne = clarifyOption(page, 1);
  await expect(optionOne).toBeVisible({ timeout: 15_000 });
  const optionText = ((await optionOne.textContent()) ?? "").trim();
  expect(optionText.length).toBeGreaterThan(0);

  // Answer by NUMBER (1). The collector maps it to the option's text.
  await submitClarifyNumber(page, clarifyId, 1);

  // The recorded answer is the OPTION'S TEXT (echoed to the output tail).
  await expect(page.getByTestId(TID.runOutput)).toContainText(optionText, {
    timeout: AI_TERMINAL_TIMEOUT_MS,
  });

  // With the merge decision made, the generator produces a fix and the
  // choice prompt appears; end deterministically with Exit.
  const choice = await waitForPromptKind(e.api, runId, "choice", { notPromptId: clarifyId });
  await clickChoice(page, choice.prompt.prompt_id, "exit");

  const terminal = await e.api.waitForRunTerminal(runId, { timeoutMs: AI_TERMINAL_TIMEOUT_MS });
  expect(terminal.outcome).toBe("user_exit");

  budget.stop();
  budget.assertWithinBudget(AI_CALL_BUDGET, "A3");

  await expectNoCanonicalChange(fixture.baseline, fixture.target);
});
