/**
 * A7 (plan §1.6/§3, SEMANTIC REWRITE for v2) — folder exclusion + boot
 * reconciliation through a REPLACEMENT CLI.
 *
 * Two remotes share a folder. Remote #1's refine run reaches a prompt and
 * holds the host-side folder flock. The sibling warning banner is asserted
 * on remote #2's session FIRST; remote #2's dispatch is then ACCEPTED by
 * the relay (sibling activity never 409s — the flock decides) and fails
 * fast with error(folder_locked). Killing remote #1 at the prompt orphans
 * its child (still holding the flock) and loses the terminal event, so run
 * #1 stays nonterminal — until a REPLACEMENT CLI boots on the folder, and
 * its boot reconciliation (plan §1.6) terminates the orphan and flips run
 * #1 to `abandoned`. The abandoned history is then read THROUGH the
 * replacement's session (project-wide reads).
 *
 * Oracle (plan §4.5): NO canonical change — nothing is applied.
 */

import { expect, test } from "@playwright/test";

import { newRunToken } from "./helpers/api-client";
import { ClaudeCallBudget } from "./helpers/claude-budget";
import { skipUnlessClaudeAvailable } from "./helpers/claude-gate";
import { waitForPendingPrompt } from "./helpers/fix-loop";
import { ProtocolEnv } from "./helpers/protocol-env";
import { TID, gotoSession } from "./helpers/ui";
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

test("A7: folder-locked dispatch fails fast; a killed remote's stale run is boot-reconciled abandoned", async ({
  page,
}) => {
  skipUnlessClaudeAvailable(test);

  env = await ProtocolEnv.start("a7-folder-exclusion");
  const e = env;

  const fixture = await e.materialize("ai-refine-one-defect");

  const remote1 = await e.startRemote(fixture.target);
  const session1 = await e.api.waitForSession((candidate) => candidate.pid === remote1.pid);
  const remote2 = await e.startRemote(fixture.target);
  const session2 = await e.api.waitForSession((candidate) => candidate.pid === remote2.pid);
  expect(session2.session_id).not.toBe(session1.session_id);
  e.note({ sessionId: session1.session_id });

  const budget1 = new ClaudeCallBudget(remote1.pid).start();
  const budget2 = new ClaudeCallBudget(remote2.pid).start();

  // Remote #1's run reaches a prompt and holds the folder flock.
  const run1 = await e.api.dispatchRun(session1.session_id, {
    command: "us-refine",
    story_id: "30.1",
    fix: true,
    client_token: newRunToken(),
  });
  e.note({ runId: run1.runId });
  await waitForPendingPrompt(e.api, session1.session_id, run1.runId);

  // Sibling warning banner asserted BEFORE remote #2 dispatches.
  await gotoSession(page, e.server.baseURL, session2.session_id);
  await expect(page.getByTestId(TID.folderWarningBanner)).toBeVisible({ timeout: 15_000 });

  // Remote #2's dispatch is ACCEPTED but fails fast error(folder_locked).
  const run2 = await e.api.dispatchRun(session2.session_id, {
    command: "us-refine",
    story_id: "30.1",
    fix: true,
    client_token: newRunToken(),
  });
  const run2Terminal = await e.api.waitForRunTerminal(session2.session_id, run2.runId, {
    timeoutMs: FOLDER_LOCKED_TIMEOUT_MS,
  });
  expect(run2Terminal.outcome).toBe("error");
  expect(run2Terminal.error_detail).toBe("folder_locked");

  // Kill remote #1 AT the prompt: its orphaned child still holds the flock
  // and the terminal event is lost — run #1 stays nonterminal.
  remote1.signal("SIGKILL");
  await remote1.waitForExit(15_000);

  // A REPLACEMENT CLI boots on the folder. Its boot reconciliation (BEFORE
  // registration) terminates the orphan group and abandons run #1.
  const replacement = await e.startRemote(fixture.target);
  const replacementSession = await e.api.waitForSession((candidate) => candidate.pid === replacement.pid);

  // The abandoned history is read THROUGH the replacement (project-wide read).
  const abandoned = await e.api.waitForRun(
    replacementSession.session_id,
    run1.runId,
    (run) => run.state === "terminal",
    { timeoutMs: ABANDON_TIMEOUT_MS },
  );
  expect(abandoned.outcome).toBe("abandoned");

  budget1.stop();
  budget2.stop();
  budget1.assertWithinBudget(AI_CALL_BUDGET_PER_REMOTE, "A7/remote1");
  budget2.assertWithinBudget(AI_CALL_BUDGET_PER_REMOTE, "A7/remote2");

  await expectNoCanonicalChange(fixture.baseline, fixture.target);
});
