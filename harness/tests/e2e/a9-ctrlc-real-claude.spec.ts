/**
 * A9 (plan §1.7/§4.3, SEMANTIC REWRITE for v2) — Ctrl+C through a REAL
 * Claude call; the interrupted terminal is read through a REPLACEMENT CLI.
 *
 * The refine run is dispatched and, once an actual `claude` descendant is
 * in flight, the remote's process group is SIGINTed (terminal Ctrl+C). The
 * remote propagates the interrupt, the run completes (bounded) with the
 * `interrupted` envelope (persisted to the CLI store), no claude descendant
 * survives, and the signaled remote legitimately dies.
 *
 * v2 change (critique A9): once the interrupted CLI exits, its session
 * DISAPPEARS and the run is NOT queryable through it. So the persisted
 * interrupted terminal is read through a REPLACEMENT CLI's project history
 * (project-wide reads), and the folder-reusable proof (a version run to
 * terminal ok) runs on that same replacement.
 *
 * Oracle (plan §4.5): NO canonical change — interrupted before any fix.
 */

import { expect, test } from "@playwright/test";

import { newRunToken } from "./helpers/api-client";
import { ClaudeCallBudget, expectNoClaudeDescendants, waitForClaudeDescendant } from "./helpers/claude-budget";
import { skipUnlessClaudeAvailable } from "./helpers/claude-gate";
import { AI_PROMPT_TIMEOUT_MS } from "./helpers/fix-loop";
import { ProtocolEnv } from "./helpers/protocol-env";
import { expectNoCanonicalChange } from "./helpers/tree-hash";

const AI_CALL_BUDGET = 8;
const INTERRUPT_COMPLETION_TIMEOUT_MS = 90_000; // bounded interrupt path
const NO_SURVIVORS_TIMEOUT_MS = 30_000;

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

test("A9: Ctrl+C during a real Claude call; a replacement CLI reads the interrupted terminal", async () => {
  skipUnlessClaudeAvailable(test);

  env = await ProtocolEnv.start("a9-ctrlc-real-claude");
  const e = env;

  const fixture = await e.materialize("ai-refine-one-defect");
  const remote = await e.startRemote(fixture.target);
  const session = await e.api.waitForSession((candidate) => candidate.pid === remote.pid);
  e.note({ sessionId: session.session_id });

  const budget = new ClaudeCallBudget(remote.pid).start();

  const { runId } = await e.api.dispatchRun(session.session_id, {
    command: "us-refine",
    story_id: "30.1",
    fix: true,
    client_token: newRunToken(),
  });
  e.note({ runId });

  // Only signal once a REAL, in-flight claude call exists.
  await waitForClaudeDescendant(remote.pid, AI_PROMPT_TIMEOUT_MS);

  // Ctrl+C: SIGINT the remote's process group.
  remote.signalGroup("SIGINT");

  // The signaled remote legitimately dies; no claude descendant survives.
  await remote.waitForExit(INTERRUPT_COMPLETION_TIMEOUT_MS);
  await expectNoClaudeDescendants(remote.pid, NO_SURVIVORS_TIMEOUT_MS);

  budget.stop();
  budget.assertWithinBudget(AI_CALL_BUDGET, "A9");

  // No canonical change — interrupted before any fix.
  await expectNoCanonicalChange(fixture.baseline, fixture.target);

  // The dead CLI's session is GONE and the run is not queryable through it.
  // A REPLACEMENT CLI on the same folder reads the persisted interrupted
  // terminal from the project history (project-wide reads).
  const replacement = await e.startRemote(fixture.target);
  const replacementSession = await e.api.waitForSession((candidate) => candidate.pid === replacement.pid);

  const interrupted = await e.api.getRun(replacementSession.session_id, runId);
  expect(interrupted.state).toBe("terminal");
  expect(interrupted.outcome).toBe("interrupted");

  const history = await e.api.getSession(replacementSession.session_id);
  expect(history.runs.map((run) => run.run_id)).toContain(runId);

  // Folder lock released + harness usable: the replacement drives a version
  // run to terminal ok on the same folder.
  const version = await e.api.dispatchRun(replacementSession.session_id, {
    command: "version",
    fix: false,
    client_token: newRunToken(),
  });
  const versionTerminal = await e.api.waitForRunTerminal(replacementSession.session_id, version.runId, {
    timeoutMs: INTERRUPT_COMPLETION_TIMEOUT_MS,
  });
  expect(versionTerminal.outcome).toBe("ok");
});
