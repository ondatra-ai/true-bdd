/**
 * A1 (plan §4.3) — `us create` no-fix, identity-engineered. Epic file
 * number 42, document epic.id 99, row-1 declared id 77.5. The browser
 * Create action on the position-derived row 42.1 must dispatch "42.1"
 * (create id = <epic-filename-number>.<position>); the engine writes
 * exactly one docs/prd/stories/77.5-*.yaml (declared id prefix).
 *
 * Folder-wide auto-refresh (plan §3.3): a SECOND idle same-folder remote
 * — which never dispatches and is never manually refreshed — sees the
 * created story after the run reaches terminal (its inventory generation
 * bumps and its story row flips), proving the terminal-run auto-refresh
 * fans out to every folder session.
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

test("A1: Create on row 42.1 writes one 77.5-*.yaml; a second idle remote auto-refreshes", async ({ page }) => {
  skipUnlessClaudeAvailable(test);

  env = await ProtocolEnv.start("a1-create-no-fix");
  const e = env;

  const fixture = await e.materialize("a1-create-no-fix");

  // Dispatching remote (drives Create) + a SECOND idle remote on the
  // SAME folder that only ever watches (auto-refresh proof).
  const remote = await e.startRemote(fixture.target);
  const session = await e.api.waitForSession((candidate) => candidate.pid === remote.pid);
  e.note({ sessionId: session.id });
  await e.api.waitForGeneration(session.id, 0);

  const idleRemote = await e.startRemote(fixture.target);
  const idleSession = await e.api.waitForSession((candidate) => candidate.pid === idleRemote.pid);
  expect(idleSession.id).not.toBe(session.id);
  const idleGenBefore = (await e.api.waitForGeneration(idleSession.id, 0)).inventory_generation;

  const budget = new ClaudeCallBudget(remote.pid).start();

  // Row 42.1 starts with no story file (created "missing").
  await gotoSession(page, e.server.baseURL, session.id);
  const row = storyRow(page, "42.1");
  await expect(row.getByTestId(TID.storyCreated)).toHaveAttribute("data-status", "missing", {
    timeout: 15_000,
  });
  await expect(row.getByTestId(TID.actionCreate)).toBeEnabled();

  // No-fix Create through the real UI control (fix toggle defaults off).
  await expect(row.getByTestId(TID.fixToggle)).not.toBeChecked();
  await row.getByTestId(TID.actionCreate).click();

  // The Create action dispatched "42.1" (position-derived) with fix=false.
  const created = await pollUntil<RunSummary>(
    async () => (await e.api.getSession(session.id)).runs.find((run) => run.command === "us-create"),
    { timeoutMs: 30_000, what: "the Create action to dispatch a us-create run" },
  );
  expect(created.story_id).toBe("42.1");
  expect(created.fix).toBe(false);
  e.note({ runId: created.id });

  const terminal = await e.api.waitForRunTerminal(created.id, { timeoutMs: AI_TERMINAL_TIMEOUT_MS });
  expect(terminal.outcome).toBe("converged");

  // Oracle: exactly one new story file under the declared-id prefix.
  await expectExactlyOneNewFileMatching(fixture.baseline, fixture.target, "docs/prd/stories/77.5-*.yaml");

  budget.stop();
  budget.assertWithinBudget(AI_CALL_BUDGET, "A1");

  // Dispatching session: row 42.1 now resolves exactly one story file.
  await expect(row.getByTestId(TID.storyCreated)).toHaveAttribute("data-status", "one", {
    timeout: 30_000,
  });

  // Folder-wide auto-refresh: the idle session's generation advanced
  // WITHOUT any dispatch or manual Refresh on it...
  await e.api.waitForGeneration(idleSession.id, idleGenBefore, { timeoutMs: 30_000 });

  // ...and its own session view renders the created story (no Refresh click).
  await gotoSession(page, e.server.baseURL, idleSession.id);
  await expect(storyRow(page, "42.1").getByTestId(TID.storyCreated)).toHaveAttribute(
    "data-status",
    "one",
    { timeout: 30_000 },
  );
});
