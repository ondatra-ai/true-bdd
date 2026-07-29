/**
 * A1 (plan §4.3, MATERIAL REWRITE for v2) — `us create` no-fix,
 * identity-engineered. Epic file number 42, document epic.id 99, row-1
 * declared id 77.5. The browser Create action on the position-derived row
 * 42.1 must dispatch "42.1" (create id = <epic-filename-number>.<position>);
 * the engine writes exactly one docs/prd/stories/77.5-*.yaml.
 *
 * v2 change (critique A1): generation / folder-wide promotion is GONE (there
 * is no cache and no generation — every browser read is a FRESH CLI scan,
 * plan §1.5). Instead of asserting an "auto-refresh generation bump" on a
 * second idle remote, this spec starts a SECOND LIVE remote on the same
 * folder (shared per-project CLI store) and asserts that BOTH live CLIs'
 * fresh reads observe the created file — exercising shared-DB ownership.
 *
 * Oracle (plan §4.5): exactly one new 77.5-*.yaml, nothing else.
 */

import { expect, test } from "@playwright/test";

import { pollUntil, type RunSummary } from "./helpers/api-client";
import { ClaudeCallBudget } from "./helpers/claude-budget";
import { skipUnlessClaudeAvailable } from "./helpers/claude-gate";
import { AI_TERMINAL_TIMEOUT_MS } from "./helpers/fix-loop";
import { ProtocolEnv } from "./helpers/protocol-env";
import { TID, gotoSession, storyRow } from "./helpers/ui";
import { expectExactlyOneNewFileMatching } from "./helpers/tree-hash";

const AI_CALL_BUDGET = 5;

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

test("A1: Create on row 42.1 writes one 77.5-*.yaml; both live CLIs' fresh reads observe it", async ({ page }) => {
  skipUnlessClaudeAvailable(test);

  env = await ProtocolEnv.start("a1-create-no-fix");
  const e = env;

  const fixture = await e.materialize("a1-create-no-fix");

  // Dispatching remote (drives Create) + a SECOND LIVE remote on the SAME
  // folder (they share the per-project CLI store) — the shared-DB witness.
  const remote = await e.startRemote(fixture.target);
  const session = await e.api.waitForSession((candidate) => candidate.pid === remote.pid);
  e.note({ sessionId: session.session_id });

  const idleRemote = await e.startRemote(fixture.target);
  const idleSession = await e.api.waitForSession((candidate) => candidate.pid === idleRemote.pid);
  expect(idleSession.session_id).not.toBe(session.session_id);

  const budget = new ClaudeCallBudget(remote.pid).start();

  // Row 42.1 starts with no story file (created "missing").
  await gotoSession(page, e.server.baseURL, session.session_id);
  const row = storyRow(page, "42.1");
  await expect(row.getByTestId(TID.storyCreated)).toHaveAttribute("data-status", "missing", { timeout: 15_000 });
  await expect(row.getByTestId(TID.actionCreate)).toBeEnabled();

  // No-fix Create through the real UI control (fix toggle defaults off).
  await expect(row.getByTestId(TID.fixToggle)).not.toBeChecked();
  await row.getByTestId(TID.actionCreate).click();

  // The Create action dispatched "42.1" (position-derived) with fix=false.
  const created = await pollUntil<RunSummary>(
    async () => (await e.api.getSession(session.session_id)).runs.find((run) => run.command === "us-create"),
    { timeoutMs: 30_000, what: "the Create action to dispatch a us-create run" },
  );
  expect(created.story_id).toBe("42.1");
  expect(created.fix).toBe(false);
  e.note({ runId: created.run_id });

  const terminal = await e.api.waitForRunTerminal(session.session_id, created.run_id, {
    timeoutMs: AI_TERMINAL_TIMEOUT_MS,
  });
  expect(terminal.outcome).toBe("converged");

  // Oracle: exactly one new story file under the declared-id prefix.
  await expectExactlyOneNewFileMatching(fixture.baseline, fixture.target, "docs/prd/stories/77.5-*.yaml");

  budget.stop();
  budget.assertWithinBudget(AI_CALL_BUDGET, "A1");

  // Dispatching session: its FRESH read resolves exactly one story file.
  await expect(row.getByTestId(TID.storyCreated)).toHaveAttribute("data-status", "one", { timeout: 30_000 });

  // The SECOND LIVE CLI's fresh read (no generation, no cache) also observes
  // the created file — shared-DB ownership + no-cache scanning.
  await gotoSession(page, e.server.baseURL, idleSession.session_id);
  await expect(storyRow(page, "42.1").getByTestId(TID.storyCreated)).toHaveAttribute("data-status", "one", {
    timeout: 30_000,
  });
});
