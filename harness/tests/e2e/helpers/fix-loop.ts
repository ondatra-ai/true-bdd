/**
 * Browser-driven `--fix` loop helpers for the AI specs (plan §4.3
 * A0–A9). The A-tests drive the real interactive fix loop THROUGH the
 * run view's prompt panel; these helpers encapsulate the prompt-panel
 * vocabulary (choice / clarify / freetext) and the bounded
 * apply-until-terminal driver.
 *
 * Every control is scoped to a specific `data-prompt-id` so a click can
 * never land on a stale panel: if the API already advanced to a new
 * prompt id but the live-polling UI has not re-rendered yet, the scoped
 * locator simply auto-waits until it does.
 *
 * Poll ceilings are AI-scale (real Claude calls take minutes; the `ai`
 * project has a 30-minute test timeout), deliberately much larger than
 * the protocol client's 60-second ceilings.
 */

import type { Page } from "@playwright/test";

import { pollUntil, type HarnessApi, type PendingPrompt, type RunDetail } from "./api-client";
import { TID } from "./ui";

/** Time to reach the next prompt through a real Claude call. */
export const AI_PROMPT_TIMEOUT_MS = 8 * 60 * 1000;
/** Time for a fix run to converge / terminate through real Claude. */
export const AI_TERMINAL_TIMEOUT_MS = 15 * 60 * 1000;

export type ChoiceAction = "apply" | "refine" | "exit";

const CHOICE_TID: Record<ChoiceAction, string> = {
  apply: TID.promptChoiceApply,
  refine: TID.promptChoiceRefine,
  exit: TID.promptChoiceExit,
};

/** Locator for the prompt panel bound to one specific prompt id. */
export function promptPanelFor(page: Page, promptId: string) {
  return page.locator(`[data-testid="${TID.promptPanel}"][data-prompt-id="${promptId}"]`);
}

/**
 * Resolves the RunDetail once a prompt is pending whose id differs from
 * `notPromptId` (so a just-answered prompt still lingering in the poll
 * response is ignored). Throws if the run reaches a terminal state
 * before the next prompt appears.
 */
export async function waitForPendingPrompt(
  api: HarnessApi,
  runId: string,
  options: { timeoutMs?: number; what?: string; notPromptId?: string } = {},
): Promise<RunDetail> {
  return pollUntil(
    async () => {
      const run = await api.getRun(runId);
      if (run.state === "terminal") {
        throw new Error(
          `run ${runId} reached terminal (${run.outcome ?? "?"}) before a prompt appeared`,
        );
      }

      const pending = run.pending_prompt;
      if (pending === null || pending.prompt_id === options.notPromptId) {
        return undefined;
      }

      return run;
    },
    {
      timeoutMs: options.timeoutMs ?? AI_PROMPT_TIMEOUT_MS,
      what: options.what ?? `run ${runId} to publish a prompt`,
    },
  );
}

/**
 * Waits for the next pending prompt (id different from `notPromptId`) and
 * asserts its kind. Fails loudly if a different kind appears — the AI
 * specs assert an EXACT kind order.
 */
export async function waitForPromptKind(
  api: HarnessApi,
  runId: string,
  kind: PendingPrompt["kind"],
  options: { timeoutMs?: number; notPromptId?: string } = {},
): Promise<{ run: RunDetail; prompt: PendingPrompt }> {
  const run = await waitForPendingPrompt(api, runId, {
    timeoutMs: options.timeoutMs,
    notPromptId: options.notPromptId,
    what: `run ${runId} to publish a ${kind} prompt`,
  });
  const prompt = run.pending_prompt as PendingPrompt;

  if (prompt.kind !== kind) {
    throw new Error(`expected a ${kind} prompt but the run published a ${prompt.kind} prompt`);
  }

  return { run, prompt };
}

/** Clicks a choice-prompt button (apply/refine/exit) on its own panel. */
export async function clickChoice(page: Page, promptId: string, action: ChoiceAction): Promise<void> {
  await promptPanelFor(page, promptId).getByTestId(CHOICE_TID[action]).click();
}

/**
 * Fills the multiline freetext refinement textarea and submits it. The
 * remote appends the terminating blank line before the child's
 * AskRefinementFeedback returns — so the run advancing to the next
 * prompt proves that framing end-to-end (A2).
 */
export async function submitFreetext(page: Page, promptId: string, text: string): Promise<void> {
  const panel = promptPanelFor(page, promptId);
  await panel.getByTestId(TID.promptFreetextInput).fill(text);
  await panel.getByTestId(TID.promptFreetextSubmit).click();
}

/**
 * Answers a clarify prompt by OPTION NUMBER (the engine collector maps
 * the number to the option's text — A3).
 */
export async function submitClarifyNumber(page: Page, promptId: string, optionNumber: number): Promise<void> {
  const panel = promptPanelFor(page, promptId);
  await panel.getByTestId(TID.promptAnswerInput).fill(String(optionNumber));
  await panel.getByTestId(TID.promptAnswerSubmit).click();
}

export interface ApplyUntilTerminalResult {
  run: RunDetail;
  applies: number;
}

/**
 * Repeatedly clicks Apply on each fresh choice prompt until the run is
 * terminal. Bounded by the checklist cap: applying MORE than `cap` times
 * without converging is a failure (the engine's own max-apply-attempts
 * ceiling should terminate the run at the cap). Asserts at least one
 * Apply happened — a fix loop that converges with zero Applies never
 * exercised the fix path.
 */
export async function applyUntilTerminal(
  page: Page,
  api: HarnessApi,
  runId: string,
  options: { cap: number; label: string; stepTimeoutMs?: number },
): Promise<ApplyUntilTerminalResult> {
  const { cap, label } = options;
  const stepTimeout = options.stepTimeoutMs ?? AI_TERMINAL_TIMEOUT_MS;
  const seen = new Set<string>();
  let applies = 0;

  for (;;) {
    const run = await pollUntil(
      async () => {
        const candidate = await api.getRun(runId);
        if (candidate.state === "terminal") {
          return candidate;
        }

        const pending = candidate.pending_prompt;
        if (pending?.kind === "choice" && !seen.has(pending.prompt_id)) {
          return candidate;
        }

        return undefined;
      },
      { timeoutMs: stepTimeout, what: `${label}: the next choice prompt or a terminal state` },
    );

    if (run.state === "terminal") {
      if (applies < 1) {
        throw new Error(`${label}: run reached terminal without a single Apply`);
      }

      return { run, applies };
    }

    if (applies >= cap) {
      throw new Error(
        `${label}: Apply loop exceeded the checklist cap (${cap}) without converging — ` +
          "the fix is not reaching a fixpoint",
      );
    }

    const promptId = (run.pending_prompt as PendingPrompt).prompt_id;
    seen.add(promptId);
    await clickChoice(page, promptId, "apply");
    applies += 1;
  }
}
