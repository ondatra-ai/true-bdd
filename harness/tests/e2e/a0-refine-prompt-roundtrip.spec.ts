/**
 * A0 (plan §4.3) — prompt round-trip + exit (spike): the shared
 * one-defect us-refine fixture, driven through the harness `us refine
 * --fix` prompt loop. The single filtered rule-based-description check
 * fails, the fix loop publishes a CHOICE prompt, the browser clicks
 * Exit, and the run terminates `user_exit`.
 *
 * LIVE-OUTPUT anchor (plan §4.3): while the run is durably blocked at
 * the prompt, initial output is already rendered; after Exit the event
 * sequence advances and the terminal outcome renders. Session is
 * reusable afterwards (connected, no active run).
 *
 * Oracle (plan §4.5): NO canonical change — Exit applies nothing.
 * Budget (plan §4.3): a single-check refine reaching one choice prompt
 * spawns ~2 claude calls (evaluate + fix-generate); bound 6 catches a
 * filter that walked the full us-refine checklist (≥12 evaluations).
 */

import { expect, test } from "@playwright/test";

import { newRunToken } from "./helpers/api-client";
import { ClaudeCallBudget } from "./helpers/claude-budget";
import { skipUnlessClaudeAvailable } from "./helpers/claude-gate";
import { AI_TERMINAL_TIMEOUT_MS, clickChoice, waitForPromptKind } from "./helpers/fix-loop";
import { ProtocolEnv } from "./helpers/protocol-env";
import { TID, gotoRun } from "./helpers/ui";
import { expectNoCanonicalChange } from "./helpers/tree-hash";

const AI_CALL_BUDGET = 6;

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

function maxSeq(events: { seq: number }[]): number {
  return events.reduce((max, event) => Math.max(max, event.seq), 0);
}

test("A0: refine prompt round-trip, Exit yields user_exit, session reusable", async ({ page }) => {
  skipUnlessClaudeAvailable(test);

  env = await ProtocolEnv.start("a0-refine-prompt-roundtrip");
  const e = env;

  const fixture = await e.materialize("ai-refine-one-defect");
  const remote = await e.startRemote(fixture.target); // default CLAUDECODE sentinel (production-strip proof)
  const session = await e.api.waitForSession((candidate) => candidate.pid === remote.pid);
  e.note({ sessionId: session.id });
  await e.api.waitForGeneration(session.id, 0);

  const budget = new ClaudeCallBudget(remote.pid).start();

  const { runId } = await e.api.dispatchRun(session.id, {
    command: "us-refine",
    story_id: "30.1",
    fix: true,
    client_token: newRunToken(),
  });
  e.note({ runId });

  await gotoRun(page, e.server.baseURL, runId);

  // Durably blocked at the CHOICE prompt: initial output is already
  // rendered (live-output anchor), and we snapshot the sequence.
  const atPrompt = await waitForPromptKind(e.api, runId, "choice");
  const promptId = atPrompt.prompt.prompt_id;
  const seqAtPrompt = maxSeq(atPrompt.run.events);

  await expect(page.getByTestId(TID.runOutput)).toContainText(/\S/, { timeout: 30_000 });
  await expect(page.locator(`[data-testid="${TID.promptPanel}"][data-prompt-id="${promptId}"]`)).toHaveAttribute(
    "data-kind",
    "choice",
  );

  // Exit through the real UI control.
  await clickChoice(page, promptId, "exit");

  const terminal = await e.api.waitForRunTerminal(runId, { timeoutMs: AI_TERMINAL_TIMEOUT_MS });
  expect(terminal.outcome).toBe("user_exit");

  // After Exit: higher sequence + terminal outcome rendered.
  expect(maxSeq(terminal.events)).toBeGreaterThan(seqAtPrompt);
  await expect(page.getByTestId(TID.runOutcome)).toHaveText("user_exit", { timeout: 15_000 });

  budget.stop();
  budget.assertWithinBudget(AI_CALL_BUDGET, "A0");

  // No canonical change — Exit applied nothing.
  await expectNoCanonicalChange(fixture.baseline, fixture.target);

  // Session reusable: still connected, no lingering active run.
  const after = await e.api.getSession(session.id);
  expect(after.reachability).toBe("connected");
  expect(after.active_run_id).toBeNull();
});
