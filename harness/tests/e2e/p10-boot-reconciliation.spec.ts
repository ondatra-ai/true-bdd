/**
 * P10 (plan §1.6, NEW for v2) — boot reconciliation, the observable subset.
 *
 * When a remote dies mid-run, its nonterminal run and possibly a live
 * orphan child are left behind. A REPLACEMENT remote on the same project
 * runs boot reconciliation BEFORE it registers (plan §1.6): it validates
 * owner pid+start_identity and child pgid+start_identity, terminates a
 * verified orphan group, and marks the stale run terminal `abandoned` —
 * never inferring liveness from a PID alone, and never touching a LIVE
 * sibling's run.
 *
 * The fine-grained matrix (crash boundaries via the gated launcher, fenced
 * single-winner lease, paused-claimant takeover, PGID-reuse-with-absent-
 * leader) is exercised by the Go store reconciliation tests (plan §5 Go);
 * this spec pins the two end-to-end-observable outcomes: a dead owner's
 * orphan is cleaned + abandoned + the folder is reusable, and a LIVE
 * sibling's run is NOT abandoned by another remote's boot.
 *
 * Uses the hidden `prompt-probe` command (blocks at a prompt holding the
 * folder flock — no `claude`, so this is deterministic protocol coverage).
 */

import { expect, test } from "@playwright/test";

import { newRunToken } from "./helpers/api-client";
import { ProtocolEnv } from "./helpers/protocol-env";

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

test("P10: a killed owner's orphaned run is boot-reconciled to abandoned; the folder is reusable", async () => {
  env = await ProtocolEnv.start("p10-abandon");
  const e = env;

  const fixture = await e.materialize("bare-host");

  // Owner #1 runs prompt-probe: it acquires the folder flock and blocks at
  // its first prompt, leaving a live child that holds the lock.
  const remote1 = await e.startRemote(fixture.target);
  const session1 = await e.api.waitForSession((candidate) => candidate.pid === remote1.pid);
  e.note({ sessionId: session1.session_id });

  const run1 = await e.api.dispatchRun(session1.session_id, {
    command: "prompt-probe",
    fix: false,
    client_token: newRunToken(),
  });
  e.note({ runId: run1.runId });
  await e.api.waitForRun(session1.session_id, run1.runId, (run) => run.pending_prompt !== null, {
    timeoutMs: 30_000,
  });

  // Kill owner #1 hard: the child is orphaned but still holds the flock,
  // and the terminal event is lost — run #1 stays nonterminal in the store.
  remote1.signal("SIGKILL");
  await remote1.waitForExit(15_000);

  // A replacement remote boots on the same folder. Its boot reconciliation
  // runs BEFORE registration: it verifies + terminates the orphan group and
  // abandons run #1. By the time its session is listed, that is done.
  const remote2 = await e.startRemote(fixture.target);
  const session2 = await e.api.waitForSession((candidate) => candidate.pid === remote2.pid);
  expect(session2.session_id).not.toBe(session1.session_id);

  // Run #1 (owned by the dead remote #1) is readable through the live
  // replacement session (project-wide reads) and is terminal `abandoned`.
  const abandoned = await e.api.waitForRun(session2.session_id, run1.runId, (run) => run.state === "terminal", {
    timeoutMs: 60_000,
  });
  expect(abandoned.outcome).toBe("abandoned");

  // The folder flock was released (orphan terminated): a fresh run on the
  // replacement reaches terminal ok.
  const version = await e.api.dispatchRun(session2.session_id, {
    command: "version",
    fix: false,
    client_token: newRunToken(),
  });
  const versionTerminal = await e.api.waitForRunTerminal(session2.session_id, version.runId, { timeoutMs: 60_000 });
  expect(versionTerminal.outcome).toBe("ok");
});

test("P10: a live sibling's run is NOT abandoned when another remote boots", async () => {
  env = await ProtocolEnv.start("p10-live-sibling");
  const e = env;

  const fixture = await e.materialize("bare-host");

  // Owner #1 holds a LIVE, nonterminal prompt-probe run.
  const remote1 = await e.startRemote(fixture.target);
  const session1 = await e.api.waitForSession((candidate) => candidate.pid === remote1.pid);
  e.note({ sessionId: session1.session_id });

  const run1 = await e.api.dispatchRun(session1.session_id, {
    command: "prompt-probe",
    fix: false,
    client_token: newRunToken(),
  });
  e.note({ runId: run1.runId });
  await e.api.waitForRun(session1.session_id, run1.runId, (run) => run.pending_prompt !== null, {
    timeoutMs: 30_000,
  });

  // A second remote boots on the same folder. Its boot reconciliation must
  // leave the LIVE sibling's run untouched (owner #1 pid+identity verify).
  const remote2 = await e.startRemote(fixture.target);
  await e.api.waitForSession((candidate) => candidate.pid === remote2.pid);

  // Run #1 is still nonterminal after the sibling booted (never abandoned).
  const stillLive = await e.api.getRun(session1.session_id, run1.runId);
  expect(stillLive.state).not.toBe("terminal");
  expect(remote1.isRunning()).toBe(true);
});
