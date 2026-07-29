/**
 * P5 (plan §4.3) — dispatch gating:
 *   - shell-ish command string → 400 (typed allowlist);
 *   - dispatch to an unreachable session → 409;
 *   - duplicate client_token → the SAME run id;
 *   - with a SIGSTOPped remote making a real queued run observable:
 *     a second distinct-token dispatch → 409 AND that session's
 *     controls are disabled in the UI;
 *   - a same-folder SIBLING session shows the warning banner with
 *     controls ENABLED (sibling activity never 409s — plan §3.3).
 */

import { expect, test } from "@playwright/test";

import { newRunToken } from "./helpers/api-client";
import { ProtocolEnv } from "./helpers/protocol-env";
import { TID, gotoSession } from "./helpers/ui";

let env: ProtocolEnv | undefined;

test.afterEach(async () => {
  // Extend (never overwrite) the shared budget so scoped teardown has
  // room even after a test-body timeout (Playwright's documented idiom).
  const info = test.info();
  info.setTimeout(info.timeout + 60_000);
  const current = env;
  env = undefined;

  if (current !== undefined) {
    await current.teardown(test.info());
  }
});

test("P5: allowlist 400, unreachable 409, token dedup, queued-run 409 + UI gating", async ({ page }) => {
  env = await ProtocolEnv.start("p5-dispatch-gating");
  const e = env;

  const fixture = await e.materialize("bare-host");

  // Two sessions on the SAME folder.
  const remote1 = await e.startRemote(fixture.target);
  const session1 = await e.api.waitForSession((candidate) => candidate.pid === remote1.pid);
  e.note({ sessionId: session1.id });

  const remote2 = await e.startRemote(fixture.target);
  const session2 = await e.api.waitForSession((candidate) => candidate.pid === remote2.pid);
  expect(session2.id).not.toBe(session1.id);

  // 1. Shell-ish command string → 400 from the typed allowlist.
  const shellish = await e.api.dispatchRunResponse(session1.id, {
    command: "version; echo pwned",
    fix: false,
    client_token: newRunToken(),
  });
  expect(shellish.status()).toBe(400);

  // 2. Dispatch to an UNREACHABLE session → 409. Make session2
  // unreachable (SIGSTOP + threshold), prove the 409, then revive it.
  remote2.signal("SIGSTOP");
  await e.api.waitForReachability(session2.id, "unreachable");

  const unreachable = await e.api.dispatchRunResponse(session2.id, {
    command: "version",
    fix: false,
    client_token: newRunToken(),
  });
  expect(unreachable.status()).toBe(409);

  remote2.signal("SIGCONT");
  await e.api.waitForReachability(session2.id, "connected", { timeoutMs: 30_000 });

  // 3. A REAL observable queued run: freeze remote1 so the dispatched
  // run can never be claimed, then dispatch while still "connected".
  remote1.signal("SIGSTOP");

  const token = newRunToken();
  const first = await e.api.dispatchRun(session1.id, {
    command: "version",
    fix: false,
    client_token: token,
  });
  expect(first.status).toBe(201);
  e.note({ runId: first.runId });

  // Duplicate client_token → the SAME run id (dedup, not a new run).
  const duplicate = await e.api.dispatchRun(session1.id, {
    command: "version",
    fix: false,
    client_token: token,
  });
  expect(duplicate.runId).toBe(first.runId);
  expect(duplicate.status).toBe(200);

  // The run is genuinely queued (remote1 is frozen).
  const queued = await e.api.getRun(first.runId);
  expect(queued.state).toBe("queued");

  // Distinct token while a non-terminal run exists → 409.
  const second = await e.api.dispatchRunResponse(session1.id, {
    command: "version",
    fix: false,
    client_token: newRunToken(),
  });
  expect(second.status()).toBe(409);

  // 4. UI: session1's own controls are DISABLED (own active run).
  await gotoSession(page, e.server.baseURL, session1.id);
  await expect(page.getByTestId(TID.actionBuildTests)).toBeDisabled({ timeout: 15_000 });
  await expect(page.getByTestId(TID.actionBuildCode)).toBeDisabled();

  // 5. UI: the same-folder SIBLING session warns but stays ENABLED.
  await gotoSession(page, e.server.baseURL, session2.id);
  await expect(page.getByTestId(TID.folderWarningBanner)).toBeVisible({ timeout: 15_000 });
  await expect(page.getByTestId(TID.actionBuildTests)).toBeEnabled();
  await expect(page.getByTestId(TID.actionBuildCode)).toBeEnabled();
});
