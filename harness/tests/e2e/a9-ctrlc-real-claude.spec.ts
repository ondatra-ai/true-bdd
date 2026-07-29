/**
 * A9 (plan §4.3) — Ctrl+C through a REAL Claude call. The refine run is
 * dispatched and, once an actual `claude` descendant is in flight, the
 * remote's process group is SIGINTed (terminal Ctrl+C semantics). The
 * remote propagates the interrupt to the child group; the run completes
 * (bounded) with the `interrupted` envelope, no claude descendant
 * survives, and the folder lock is released — proven by a REPLACEMENT
 * remote/session whose `version` run reaches terminal `ok`. The signaled
 * remote legitimately dies.
 *
 * No TERM→KILL-escalation assertion here — forced escalation on an
 * engineered long-running child is the Go test's job (plan §4.6).
 *
 * Oracle (plan §4.5): NO canonical change — the run is interrupted
 * before any fix is applied.
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

test("A9: Ctrl+C during a real Claude call yields interrupted; folder reusable after", async () => {
  skipUnlessClaudeAvailable(test);

  env = await ProtocolEnv.start("a9-ctrlc-real-claude");
  const e = env;

  const fixture = await e.materialize("ai-refine-one-defect");
  const remote = await e.startRemote(fixture.target);
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

  // Only signal once a REAL, in-flight claude call exists.
  await waitForClaudeDescendant(remote.pid, AI_PROMPT_TIMEOUT_MS);

  // Ctrl+C: SIGINT the remote's process group.
  remote.signalGroup("SIGINT");

  // Bounded completion with the interrupted envelope.
  const terminal = await e.api.waitForRunTerminal(runId, { timeoutMs: INTERRUPT_COMPLETION_TIMEOUT_MS });
  expect(terminal.outcome).toBe("interrupted");

  // The signaled remote legitimately dies; no claude descendant survives.
  await remote.waitForExit(INTERRUPT_COMPLETION_TIMEOUT_MS);
  await expectNoClaudeDescendants(remote.pid, NO_SURVIVORS_TIMEOUT_MS);

  budget.stop();
  budget.assertWithinBudget(AI_CALL_BUDGET, "A9");

  // No canonical change — interrupted before any fix.
  await expectNoCanonicalChange(fixture.baseline, fixture.target);

  // Folder lock released + harness usable: a REPLACEMENT remote/session
  // drives a version run to terminal ok on the same folder.
  const replacement = await e.startRemote(fixture.target);
  const replacementSession = await e.api.waitForSession((candidate) => candidate.pid === replacement.pid);
  const version = await e.api.dispatchRun(replacementSession.id, {
    command: "version",
    fix: false,
    client_token: newRunToken(),
  });
  const versionTerminal = await e.api.waitForRunTerminal(version.runId, {
    timeoutMs: INTERRUPT_COMPLETION_TIMEOUT_MS,
  });
  expect(versionTerminal.outcome).toBe("ok");
});
