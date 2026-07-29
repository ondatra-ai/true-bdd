/**
 * A8 (plan §4.3) — server restart during an active run + DURABLE answer.
 * The refine run reaches a choice prompt; the remote is SIGSTOPped (its
 * command child stays blocked); the Exit answer is submitted while the
 * remote is frozen (accepted + durable); the server is restarted against
 * the SAME DB; the remote is SIGCONTinued and delivers the persisted
 * answer to the SAME child (no respawn — child PGID unchanged), which is
 * consumed exactly once. Pre-restart event sequences appear once and
 * exactly one user_exit terminal results.
 *
 * Oracle (plan §4.5): NO canonical change — Exit applies nothing.
 */

import { expect, test } from "@playwright/test";

import { newRunToken, type RunEvent } from "./helpers/api-client";
import { ClaudeCallBudget } from "./helpers/claude-budget";
import { skipUnlessClaudeAvailable } from "./helpers/claude-gate";
import { AI_TERMINAL_TIMEOUT_MS, waitForPromptKind } from "./helpers/fix-loop";
import { ProtocolEnv } from "./helpers/protocol-env";
import type { RemoteProcess } from "./helpers/remote-process";
import { expectNoCanonicalChange } from "./helpers/tree-hash";

const AI_CALL_BUDGET = 6;

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

/** The command child's process-group id for a run (from the pids file). */
function childPgidFor(remote: RemoteProcess, runId: string): number | undefined {
  const entries = remote.readChildrenPids();
  const forRun = entries.find((entry) => entry.runId === runId);

  return (forRun ?? entries[0])?.pgid;
}

function seqs(events: RunEvent[]): number[] {
  return events.map((event) => event.seq);
}

test("A8: restart with a pending answer consumes it once against the same child", async () => {
  skipUnlessClaudeAvailable(test);

  env = await ProtocolEnv.start("a8-restart-durable-answer");
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

  // Reach the choice prompt; snapshot the child PGID and pre-restart seqs.
  const atPrompt = await waitForPromptKind(e.api, runId, "choice");
  const promptId = atPrompt.prompt.prompt_id;
  const childPgidBefore = childPgidFor(remote, runId);
  expect(childPgidBefore, "no command child recorded in the pids file").not.toBeUndefined();
  const preRestartSeqs = seqs(atPrompt.run.events);
  expect(new Set(preRestartSeqs).size).toBe(preRestartSeqs.length);

  // Freeze the remote; its command child stays blocked at the prompt.
  remote.signal("SIGSTOP");

  // Submit Exit while frozen: accepted and durable (answer_accepted).
  const answer = await e.api.answerRunResponse(runId, { prompt_id: promptId, value: "exit" });
  expect(answer.status()).toBe(200);
  await e.api.waitForRun(runId, (run) => run.state === "answer_accepted", { timeoutMs: 30_000 });

  // Restart the server against the SAME DB + port; the answer survives.
  await e.server.restart();

  // Resume the remote: it delivers the persisted answer to the SAME
  // still-alive child (no respawn).
  remote.signal("SIGCONT");
  const childPgidAfter = childPgidFor(remote, runId);
  expect(childPgidAfter).toBe(childPgidBefore);

  const terminal = await e.api.waitForRunTerminal(runId, { timeoutMs: AI_TERMINAL_TIMEOUT_MS });
  expect(terminal.outcome).toBe("user_exit");

  // Answer consumed once / no replay: unique seqs, pre-restart seqs still
  // present exactly once, exactly one prompt ever published.
  const finalSeqs = seqs(terminal.events);
  expect(new Set(finalSeqs).size).toBe(finalSeqs.length);
  for (const seq of preRestartSeqs) {
    expect(finalSeqs.filter((candidate) => candidate === seq)).toHaveLength(1);
  }
  const promptEvents = terminal.events.filter((event) => event.type === "prompt");
  expect(promptEvents).toHaveLength(1);

  budget.stop();
  budget.assertWithinBudget(AI_CALL_BUDGET, "A8");

  await expectNoCanonicalChange(fixture.baseline, fixture.target);
});
