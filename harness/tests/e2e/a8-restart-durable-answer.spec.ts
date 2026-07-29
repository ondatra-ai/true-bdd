/**
 * A8 (plan §1.7/§2, SEMANTIC REWRITE for v2) — RELAY restart while the CLI +
 * child stay ALIVE; the pending prompt SURVIVES from the CLI store.
 *
 * v1 expected an answer submitted while the CLI was FROZEN to be durable in
 * the SERVER DB. v2 deletes that (critique A8): the relay holds no state, so
 * a frozen CLI can neither accept nor persist an answer. Instead: the refine
 * run reaches a choice prompt with the remote AND its command child alive;
 * the RELAY is restarted; the remote re-registers with the same client-owned
 * session id; the pending prompt is still readable (persisted in the CLI
 * SQLite store, not the relay). The Exit answer is then RESUBMITTED and
 * delivered to the SAME still-alive child (no respawn — child PGID
 * unchanged), consumed exactly once, yielding one user_exit terminal with
 * no duplicated event sequences.
 *
 * The "reply lost after the CLI commit" fault is covered separately by
 * P13's exact-retry semantics.
 *
 * Oracle (plan §4.5): NO canonical change — Exit applies nothing.
 */

import { expect, test } from "@playwright/test";

import { newRunToken, pollUntil, type RunEvent } from "./helpers/api-client";
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

test("A8: relay restart with a live CLI+child; the prompt survives from the CLI store; answer once", async () => {
  skipUnlessClaudeAvailable(test);

  env = await ProtocolEnv.start("a8-restart-durable-answer");
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

  // Reach the choice prompt; snapshot the child PGID and pre-restart seqs.
  const atPrompt = await waitForPromptKind(e.api, session.session_id, runId, "choice");
  const promptId = atPrompt.prompt.prompt_id;
  const childPgidBefore = childPgidFor(remote, runId);
  expect(childPgidBefore, "no command child recorded in the pids file").not.toBeUndefined();
  const preRestartSeqs = seqs(atPrompt.run.events);
  expect(new Set(preRestartSeqs).size).toBe(preRestartSeqs.length);

  // Restart the STATELESS relay. The remote AND its child STAY ALIVE — no
  // SIGSTOP. The pending prompt is NOT in the relay; it persists in the CLI
  // store.
  await e.server.restart();

  // The remote re-registers with the SAME client-owned session id, and the
  // pending prompt is still readable (from the CLI SQLite store).
  const survived = await pollUntil(
    async () => {
      const listed = (await e.api.listSessions()).some((s) => s.session_id === session.session_id);
      if (!listed) {
        return undefined;
      }
      const run = await e.api.getRun(session.session_id, runId);

      return run.pending_prompt?.prompt_id === promptId ? run : undefined;
    },
    { timeoutMs: 60_000, what: "the pending prompt to survive the relay restart from the CLI store" },
  );
  expect(survived.pending_prompt?.prompt_id).toBe(promptId);

  // RESUBMIT Exit AFTER the restart: delivered to the SAME still-alive child.
  const answer = await e.api.answerRunResponse(session.session_id, runId, { prompt_id: promptId, value: "exit" });
  expect(answer.status()).toBe(200);

  const childPgidAfter = childPgidFor(remote, runId);
  expect(childPgidAfter).toBe(childPgidBefore);

  const terminal = await e.api.waitForRunTerminal(session.session_id, runId, { timeoutMs: AI_TERMINAL_TIMEOUT_MS });
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
