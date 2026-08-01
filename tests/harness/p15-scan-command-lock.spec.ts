/**
 * P15 (plan §1.5, NEW for v2) — scan-vs-command lock priority, the
 * observable subset. The linearizable protocol (SCAN takes scan.lock
 * shared then probes command-intent.lock; COMMAND plants intent exclusive,
 * drains scans, then takes the folder flock) is fully exercised by the Go
 * lock-protocol test (plan §5 Go — barrier around the probe/acquire
 * boundary; paused shared scan ⇒ dispatch waits and succeeds).
 *
 * This spec pins the end-to-end-observable half: a command running against
 * a folder holds the folder flock (command-vs-command stays fail-fast), so
 * a same-folder SIBLING command fails fast with error(folder_locked) — no
 * `claude`, via the deterministic `prompt-probe` barrier.
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

test("P15: a same-folder sibling command fails fast with folder_locked while the flock is held", async () => {
  env = await ProtocolEnv.start("p15-scan-command-lock");
  const e = env;

  const fixture = await e.materialize("bare-host");

  const remote1 = await e.startRemote(fixture.target);
  const session1 = await e.api.waitForSession((candidate) => candidate.pid === remote1.pid);
  e.note({ sessionId: session1.session_id });

  const remote2 = await e.startRemote(fixture.target);
  const session2 = await e.api.waitForSession((candidate) => candidate.pid === remote2.pid);
  expect(session2.session_id).not.toBe(session1.session_id);

  // Remote #1's command acquires the folder flock and blocks at its prompt.
  const held = await e.api.dispatchRun(session1.session_id, {
    command: "prompt-probe",
    fix: false,
    client_token: newRunToken(),
  });
  e.note({ runId: held.runId });
  await e.api.waitForRun(session1.session_id, held.runId, (run) => run.pending_prompt !== null, {
    timeoutMs: 30_000,
  });

  // Remote #2's command is ACCEPTED by the relay (sibling activity never
  // 409s) but fails fast with error(folder_locked) — command-vs-command is
  // fail-fast, and no child is ever spawned for it.
  const blocked = await e.api.dispatchRun(session2.session_id, {
    command: "prompt-probe",
    fix: false,
    client_token: newRunToken(),
  });
  const terminal = await e.api.waitForRunTerminal(session2.session_id, blocked.runId, { timeoutMs: 60_000 });
  expect(terminal.outcome).toBe("error");
  expect(terminal.error_detail).toBe("folder_locked");
});
