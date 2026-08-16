/**
 * P5 (plan §1.4/§3, REWRITTEN for v2) — dispatch gating.
 *
 * v1 froze the CLI so a SERVER-queued run was observable and dispatched to
 * an "unreachable" session for a 409. v2 has neither: dispatch is a CLI
 * store transaction (critique §6), so a frozen CLI can never commit one —
 * the browser dispatch TIMES OUT (504) with no observable run. To make a
 * REAL non-terminal run observable while the CLI stays live, this spec uses
 * a CLI-side EXECUTION BARRIER: the hidden `prompt-probe` command blocks at
 * its first prompt (plan §4), giving the owner a durable active run.
 *
 * Asserts:
 *   - a shell-ish command string → 400 (typed allowlist);
 *   - the barrier run makes a duplicate client_token dedup to the SAME run
 *     id, and a distinct-token dispatch → 409 (one non-terminal run per
 *     owner — a CLI domain conflict);
 *   - the owner's own controls are disabled in the UI;
 *   - a same-folder SIBLING session shows the warning banner with controls
 *     ENABLED (sibling activity never 409s — the folder flock decides);
 *   - a FROZEN CLI's dispatch times out / gone (never a server-queued run).
 */

import { expect, test } from "@playwright/test";

import { newRunToken, STATUS } from "./helpers/api-client";
import { ProtocolEnv } from "./helpers/protocol-env";
import { TID, gotoSession } from "./helpers/ui";

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

test("P5: allowlist 400, barrier dedup + 409, UI gating, sibling enabled, frozen dispatch times out", async ({
  page,
}) => {
  env = await ProtocolEnv.start("p5-dispatch-gating");
  const e = env;

  const fixture = await e.materialize("bare-host");

  // Two sessions on the SAME folder (they share the per-project CLI store).
  const remote1 = await e.startRemote(fixture.target);
  const session1 = await e.api.waitForSession((candidate) => candidate.pid === remote1.pid);
  e.note({ sessionId: session1.session_id });

  const remote2 = await e.startRemote(fixture.target);
  const session2 = await e.api.waitForSession((candidate) => candidate.pid === remote2.pid);
  expect(session2.session_id).not.toBe(session1.session_id);

  // 1. Shell-ish command string → 400 from the typed allowlist.
  const shellish = await e.api.dispatchRunResponse(session1.session_id, {
    command: "version; echo pwned",
    fix: false,
    client_token: newRunToken(),
  });
  expect(shellish.status()).toBe(STATUS.bad_request);

  // 2. CLI-side execution barrier: prompt-probe blocks at its first prompt,
  // giving session1's owner a durable non-terminal run WITHOUT freezing it.
  const token = newRunToken();
  const barrier = await e.api.dispatchRun(session1.session_id, {
    command: "prompt-probe",
    fix: false,
    client_token: token,
  });
  expect(barrier.status).toBe(STATUS.created);
  e.note({ runId: barrier.runId });

  await e.api.waitForRun(session1.session_id, barrier.runId, (run) => run.pending_prompt !== null, {
    timeoutMs: 30_000,
  });

  // Duplicate client_token → the SAME run id (dedup, not a new run).
  const duplicate = await e.api.dispatchRun(session1.session_id, {
    command: "prompt-probe",
    fix: false,
    client_token: token,
  });
  expect(duplicate.runId).toBe(barrier.runId);
  expect(duplicate.status).toBe(STATUS.ok);

  // Distinct token while a non-terminal run exists → 409 (CLI domain conflict).
  const second = await e.api.dispatchRunResponse(session1.session_id, {
    command: "version",
    fix: false,
    client_token: newRunToken(),
  });
  expect(second.status()).toBe(STATUS.conflict);

  // 3. UI: session1's own controls are DISABLED (own active run).
  await gotoSession(page, e.server.baseURL, session1.session_id);
  await expect(page.getByTestId(TID.actionBuildTests)).toBeDisabled({ timeout: 15_000 });
  await expect(page.getByTestId(TID.actionBuildCode)).toBeDisabled();

  // 4. UI: the same-folder SIBLING (session2) warns from the CLI's
  // active_owners but stays ENABLED (sibling activity never 409s).
  await gotoSession(page, e.server.baseURL, session2.session_id);
  await expect(page.getByTestId(TID.folderWarningBanner)).toBeVisible({ timeout: 15_000 });
  await expect(page.getByTestId(TID.actionBuildTests)).toBeEnabled();
  await expect(page.getByTestId(TID.actionBuildCode)).toBeEnabled();

  // 5. A FROZEN CLI can never commit a dispatch: the relay delivers the
  // work item, the stopped CLI never replies, and the browser sees a
  // timeout / gone — NEVER a server-queued run (there is no server store).
  remote2.signal("SIGSTOP");
  const frozen = await e.api.dispatchRunResponse(session2.session_id, {
    command: "version",
    fix: false,
    client_token: newRunToken(),
  });
  expect([STATUS.cli_timeout, STATUS.session_gone]).toContain(frozen.status());
});
