/**
 * A7 (plan §4.3, §3.7) — folder exclusion + stale abandonment. Two
 * remotes share a folder. Remote #1's refine run reaches a prompt and
 * holds the host-side folder flock. The sibling warning banner is
 * asserted on remote #2's session FIRST; remote #2's dispatch is then
 * ACCEPTED by the server (sibling activity never 409s — the flock
 * decides) and fails fast with error(folder_locked). Killing remote #1
 * at the prompt makes its orphaned child EOF-exit — releasing the lock
 * but losing the terminal event — so run #1 stays non-terminal until a
 * later remote #2 dispatch acquires the folder flock and flips it to
 * `abandoned`.
 *
 * Oracle (plan §4.5): NO canonical change — nothing is applied.
 */

import { expect, test } from "@playwright/test";

import { newRunToken } from "./helpers/api-client";
import { ClaudeCallBudget } from "./helpers/claude-budget";
import { skipUnlessClaudeAvailable } from "./helpers/claude-gate";
import { AI_TERMINAL_TIMEOUT_MS, clickChoice, waitForPendingPrompt, waitForPromptKind } from "./helpers/fix-loop";
import { ProtocolEnv } from "./helpers/protocol-env";
import { TID, gotoRun, gotoSession } from "./helpers/ui";
import { expectNoCanonicalChange } from "./helpers/tree-hash";

const AI_CALL_BUDGET_PER_REMOTE = 6;
const FOLDER_LOCKED_TIMEOUT_MS = 60_000; // fails fast: no claude is ever spawned
const ABANDON_TIMEOUT_MS = 60_000;

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

test("A7: folder-locked dispatch fails fast; a killed remote's stale run flips abandoned", async ({ page }) => {
  skipUnlessClaudeAvailable(test);

  env = await ProtocolEnv.start("a7-folder-exclusion");
  const e = env;

  const fixture = await e.materialize("ai-refine-one-defect");

  const remote1 = await e.startRemote(fixture.target);
  const session1 = await e.api.waitForSession((candidate) => candidate.pid === remote1.pid);
  const remote2 = await e.startRemote(fixture.target);
  const session2 = await e.api.waitForSession((candidate) => candidate.pid === remote2.pid);
  expect(session2.id).not.toBe(session1.id);
  e.note({ sessionId: session1.id });
  await e.api.waitForGeneration(session1.id, 0);
  await e.api.waitForGeneration(session2.id, 0);

  const budget1 = new ClaudeCallBudget(remote1.pid).start();
  const budget2 = new ClaudeCallBudget(remote2.pid).start();

  // Remote #1's run reaches a prompt and holds the folder flock.
  const run1 = await e.api.dispatchRun(session1.id, {
    command: "us-refine",
    story_id: "30.1",
    fix: true,
    client_token: newRunToken(),
  });
  e.note({ runId: run1.runId });
  await waitForPendingPrompt(e.api, run1.runId);

  // Sibling warning banner asserted BEFORE remote #2 dispatches.
  await gotoSession(page, e.server.baseURL, session2.id);
  await expect(page.getByTestId(TID.folderWarningBanner)).toBeVisible({ timeout: 15_000 });

  // Remote #2's dispatch is ACCEPTED by the server but fails fast with
  // error(folder_locked) — no claude is ever spawned for it.
  const run2 = await e.api.dispatchRun(session2.id, {
    command: "us-refine",
    story_id: "30.1",
    fix: true,
    client_token: newRunToken(),
  });
  const run2Terminal = await e.api.waitForRunTerminal(run2.runId, { timeoutMs: FOLDER_LOCKED_TIMEOUT_MS });
  expect(run2Terminal.outcome).toBe("error");
  expect(run2Terminal.error_detail).toBe("folder_locked");

  // Kill remote #1 AT the prompt: its orphaned child sees stdin EOF and
  // Exits, releasing the flock, but the terminal event is lost — run #1
  // stays non-terminal.
  remote1.signal("SIGKILL");
  await remote1.waitForExit(15_000);

  // Remote #2 dispatches again; acquiring the now-free flock abandons the
  // stale run #1.
  const run3 = await e.api.dispatchRun(session2.id, {
    command: "us-refine",
    story_id: "30.1",
    fix: true,
    client_token: newRunToken(),
  });

  const abandoned = await e.api.waitForRun(
    run1.runId,
    (run) => run.state === "terminal",
    { timeoutMs: ABANDON_TIMEOUT_MS },
  );
  expect(abandoned.outcome).toBe("abandoned");

  // End run #3 cleanly (reach its prompt, Exit) so nothing hangs.
  await gotoRun(page, e.server.baseURL, run3.runId);
  const choice = await waitForPromptKind(e.api, run3.runId, "choice");
  await clickChoice(page, choice.prompt.prompt_id, "exit");
  const run3Terminal = await e.api.waitForRunTerminal(run3.runId, { timeoutMs: AI_TERMINAL_TIMEOUT_MS });
  expect(run3Terminal.outcome).toBe("user_exit");

  budget1.stop();
  budget2.stop();
  budget1.assertWithinBudget(AI_CALL_BUDGET_PER_REMOTE, "A7/remote1");
  budget2.assertWithinBudget(AI_CALL_BUDGET_PER_REMOTE, "A7/remote2");

  await expectNoCanonicalChange(fixture.baseline, fixture.target);
});
