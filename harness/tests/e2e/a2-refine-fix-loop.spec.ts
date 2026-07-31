/**
 * A2 (plan §4.3) — full `us refine --fix` loop, identity-engineered.
 * Story file 77.5-*.yaml carries internal id 88.9; the Refine action on
 * epic row 70.1 sends the file-prefix id "77.5". The browser drives the
 * event-kind order choice → freetext → choice, the second choice prompt
 * carrying a NEW prompt id, then Applies to convergence.
 *
 * EXACT multiline transport (plan §4.3): the freetext refinement value
 * has a real embedded newline and NO trailing blank line; the run only
 * advances to the next choice prompt because the remote appended the
 * terminating blank line before the child's AskRefinementFeedback
 * returned — so reaching choice #2 is the end-to-end proof of that
 * framing. (Byte-exact stdin serialization is the Go test's job, §4.6.)
 *
 * Final invariant (plan §4.3/§4.5): the designed defect is gone — AC-1's
 * description is now a must/should rule with no inline Gherkin — and
 * ONLY the target story file changed.
 */

import fs from "node:fs";
import path from "node:path";

import { expect, test } from "@playwright/test";

import { pollUntil, type RunEvent, type RunSummary } from "./helpers/api-client";
import { ClaudeCallBudget } from "./helpers/claude-budget";
import { skipUnlessClaudeAvailable } from "./helpers/claude-gate";
import {
  AI_TERMINAL_TIMEOUT_MS,
  applyUntilTerminal,
  clickChoice,
  submitFreetext,
  waitForPromptKind,
} from "./helpers/fix-loop";
import { ProtocolEnv } from "./helpers/protocol-env";
import { TID, gotoRun, gotoSession, storyRow } from "./helpers/ui";
import { expectOnlyPathsChanged } from "./helpers/tree-hash";

const AI_CALL_BUDGET = 12;
const APPLY_CAP = 5; // us-refine has no config block → engine default max_apply_attempts

// Two-line refinement feedback (real embedded newline, no trailing
// blank line). Consistent with the fix direction (single must sentence)
// so the loop still converges.
const REFINEMENT_FEEDBACK = [
  "Keep the rewritten description to a single sentence.",
  'Use the keyword "must" since a 150-word summary is core behavior.',
].join("\n");

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

/** Kinds of the run's prompt events, in emission order. */
function promptEvents(events: RunEvent[]): { kind: string; promptId: string }[] {
  return events
    .filter((event) => event.type === "prompt")
    .map((event) => ({ kind: String(event.kind), promptId: String(event.prompt_id) }));
}

test("A2: refine fix loop — choice→freetext→choice, converges, defect gone", async ({ page }) => {
  skipUnlessClaudeAvailable(test);

  env = await ProtocolEnv.start("a2-refine-fix-loop");
  const e = env;

  const fixture = await e.materialize("a2-refine-fix-loop");
  const remote = await e.startRemote(fixture.target);
  const session = await e.api.waitForSession((candidate) => candidate.pid === remote.pid);
  e.note({ sessionId: session.session_id });

  const budget = new ClaudeCallBudget(remote.pid).start();

  // Dispatch through the row action with --fix toggled on.
  await gotoSession(page, e.server.baseURL, session.session_id);
  const row = storyRow(page, "70.1");
  await expect(row.getByTestId(TID.actionRefine)).toBeEnabled({ timeout: 15_000 });
  await row.getByTestId(TID.fixToggle).check();
  await row.getByTestId(TID.actionRefine).click();

  const dispatched = await pollUntil<RunSummary>(
    async () => (await e.api.getSession(session.session_id)).runs.find((run) => run.command === "us-refine"),
    { timeoutMs: 30_000, what: "the Refine action to dispatch a us-refine run" },
  );
  expect(dispatched.story_id).toBe("77.5"); // file-prefix id, not the position-derived 70.1
  expect(dispatched.fix).toBe(true);
  const runId = dispatched.run_id;
  e.note({ runId });

  await gotoRun(page, e.server.baseURL, session.session_id, runId);

  // choice #1 → Refine.
  const choice1 = await waitForPromptKind(e.api, session.session_id, runId, "choice");
  const choice1Id = choice1.prompt.prompt_id;
  await clickChoice(page, choice1Id, "refine");

  // freetext → submit the EXACT multiline feedback (no trailing blank line).
  const freetext = await waitForPromptKind(e.api, session.session_id, runId, "freetext", { notPromptId: choice1Id });
  const freetextId = freetext.prompt.prompt_id;
  await submitFreetext(page, freetextId, REFINEMENT_FEEDBACK);

  // choice #2 — a NEW prompt id — proves the freetext was consumed
  // (remote framed it with the terminating blank line).
  const choice2 = await waitForPromptKind(e.api, session.session_id, runId, "choice", { notPromptId: freetextId });
  const choice2Id = choice2.prompt.prompt_id;
  expect(choice2Id).not.toBe(choice1Id);
  expect(choice2Id).not.toBe(freetextId);

  // Apply-until-terminal from choice #2 (≥1 Apply, fail past the cap).
  const { run: terminal, applies } = await applyUntilTerminal(page, e.api, session.session_id, runId, {
    cap: APPLY_CAP,
    label: "A2",
    stepTimeoutMs: AI_TERMINAL_TIMEOUT_MS,
  });
  expect(applies).toBeGreaterThanOrEqual(1);
  expect(terminal.outcome).toBe("converged");
  await expect(page.getByTestId(TID.runOutcome)).toHaveText("converged", { timeout: 15_000 });

  // Event-kind order: the run published choice → freetext → choice, the
  // two choice prompts distinct.
  const kinds = promptEvents(terminal.events);
  expect(kinds.slice(0, 3).map((event) => event.kind)).toEqual(["choice", "freetext", "choice"]);
  expect(kinds[0].promptId).not.toBe(kinds[2].promptId);

  budget.stop();
  budget.assertWithinBudget(AI_CALL_BUDGET, "A2");

  // Oracle: only the target story file changed.
  await expectOnlyPathsChanged(fixture.baseline, fixture.target, ["docs/prd/stories/77.5-*.yaml"]);

  // Final invariant: AC-1's description is now a must/should rule with
  // no inline Gherkin (the designed defect is gone). Isolate AC-1's
  // description field (id → description → steps struct order).
  const storyText = fs.readFileSync(
    path.join(fixture.target, "docs", "prd", "stories", "77.5-document-summary.yaml"),
    "utf8",
  );
  const ac1Block = storyText.slice(storyText.indexOf("AC-1"), storyText.indexOf("AC-2"));
  const descMatch = ac1Block.match(/description:\s*([\s\S]*?)\n\s*steps:/);
  expect(descMatch, "AC-1 description/steps block not found").not.toBeNull();
  const ac1Description = (descMatch as RegExpMatchArray)[1].replace(/\s+/g, " ").trim();

  expect(ac1Description).toMatch(/\b(must|should)\b/i);
  // "No inline Gherkin" = not in the Given/When/Then STRUCTURE. The
  // shipped us-refine checklist's rule-based template is literally
  // "[Subject] must/should [behavior] when [condition]" (us-refine.yaml),
  // and every clean AC in this fixture's story (AC-2..AC-5) uses "when"
  // as a conjunction — so "when" is part of a correct rule description,
  // not a Gherkin marker. The Gherkin structural keywords absent from the
  // rule template are "given"/"then"; the original defect carried both.
  expect(ac1Description).not.toMatch(/\b(given|then)\b/i);
});
