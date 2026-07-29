/**
 * P14 (plan §4, NEW for v2) — PROMPTS ARE NATIVE `<dialog>` MODALS.
 *
 * Peter's literal requirement: the inline PromptPanel is replaced by a
 * native `<dialog>` prompt modal (showModal, focus containment, backdrop)
 * for all three prompt kinds. Deterministic NON-Claude coverage comes from
 * the hidden `prompt-probe` command, which emits a choice → clarify →
 * freetext sequence through the real answer collector and reads answers
 * from stdin (plan §4).
 *
 * Asserts: dialog modality + initial focus containment; all three kinds
 * answered in order (each on its own `data-prompt-id`); a FAILED answer RPC
 * keeps the dialog open with a visible error; and Escape does NOT answer —
 * the dialog stays until the prompt is answered or the run ends.
 */

import { expect, test } from "@playwright/test";

import { newRunToken } from "./helpers/api-client";
import { clickChoice, submitClarifyNumber, submitFreetext, waitForPromptKind } from "./helpers/fix-loop";
import { ProtocolEnv } from "./helpers/protocol-env";
import { TID, gotoRun, promptDialog } from "./helpers/ui";

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

test("P14: prompt-probe drives a native <dialog> through choice → clarify → freetext", async ({ page }) => {
  env = await ProtocolEnv.start("p14-prompt-dialog");
  const e = env;

  const fixture = await e.materialize("bare-host");
  const remote = await e.startRemote(fixture.target);
  const session = await e.api.waitForSession((candidate) => candidate.pid === remote.pid);
  e.note({ sessionId: session.session_id });

  const { runId } = await e.api.dispatchRun(session.session_id, {
    command: "prompt-probe",
    fix: false,
    client_token: newRunToken(),
  });
  e.note({ runId });

  await gotoRun(page, e.server.baseURL, session.session_id, runId);

  // ── choice: the dialog opens as a native modal with focus contained ──
  const choice = await waitForPromptKind(e.api, session.session_id, runId, "choice");
  const choiceId = choice.prompt.prompt_id;
  const choiceDialog = promptDialog(page, choiceId);
  await expect(choiceDialog).toBeVisible({ timeout: 15_000 });
  await expect(choiceDialog).toHaveAttribute("data-kind", "choice");

  const focusInside = await page.evaluate(() => {
    const dialog = document.querySelector('[data-testid="prompt-dialog"]');
    return !!(dialog && document.activeElement && dialog.contains(document.activeElement));
  });
  expect(focusInside).toBe(true);

  // Escape does NOT answer — the dialog stays and the prompt is unchanged.
  await page.keyboard.press("Escape");
  await expect(choiceDialog).toBeVisible();
  const afterEscape = await e.api.getRun(session.session_id, runId);
  expect(afterEscape.pending_prompt?.prompt_id).toBe(choiceId);

  await clickChoice(page, choiceId, "apply");

  // ── clarify: a FAILED RPC (empty answer) keeps the dialog open + errors ──
  const clarify = await waitForPromptKind(e.api, session.session_id, runId, "clarify", { notPromptId: choiceId });
  const clarifyId = clarify.prompt.prompt_id;
  const clarifyDialog = promptDialog(page, clarifyId);
  await expect(clarifyDialog).toHaveAttribute("data-kind", "clarify");

  // Submit an EMPTY clarify answer: the RPC is rejected and the dialog stays
  // open with a visible error (plan §4).
  await clarifyDialog.getByTestId(TID.promptAnswerInput).fill("");
  await clarifyDialog.getByTestId(TID.promptAnswerSubmit).click();
  await expect(clarifyDialog.getByTestId(TID.promptDialogError)).toBeVisible({ timeout: 15_000 });
  await expect(clarifyDialog).toBeVisible();

  // A valid answer clears the dialog and advances.
  await submitClarifyNumber(page, clarifyId, 1);

  // ── freetext: the multiline textarea answers the last prompt ──
  const freetext = await waitForPromptKind(e.api, session.session_id, runId, "freetext", { notPromptId: clarifyId });
  const freetextId = freetext.prompt.prompt_id;
  await expect(promptDialog(page, freetextId)).toHaveAttribute("data-kind", "freetext");
  await submitFreetext(page, freetextId, "final probe feedback\n");

  // All three answered: the run leaves the freetext prompt (terminal or a
  // cleared prompt) — the dialog for the answered prompt is gone.
  await expect(promptDialog(page, freetextId)).toHaveCount(0, { timeout: 30_000 });
});
