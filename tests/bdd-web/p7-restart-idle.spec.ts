/**
 * P7 (plan: connect-cli-to-vercel-harness, REWRITTEN) — across a Next restart
 * the session NEVER disappears: the Redis-backed registry survives a memory
 * wipe, so the live remote's existing credentials still authenticate and it
 * NEVER has to re-register.
 *
 * Today this spec explicitly waited for the remote to get 404 and re-register
 * after a relay restart (the in-process model: the registry is process memory,
 * wiped on restart). Under Redis the registration SURVIVES — the session is
 * listed continuously across the restart with the SAME session_id, pid, AND
 * connected_at (a re-register would mint a fresh connected_at). Epoch/token
 * persistence is exercised separately by p18 (raw-agent poll with the same
 * creds succeeds after a restart).
 *
 * This is a pure black-box test:
 *   - the session is present in GET /api/sessions IMMEDIATELY after restart()
 *     returns (no re-register wait), with connected_at UNCHANGED — proving no
 *       re-register happened (the discriminator: in-process wipes the registry,
 *       so the session is either gone or re-registered with a newer connected_at);
 *   - the pre-restart run history is queryable at once (CLI-owned store).
 *
 * Genuine expiry-driven re-registration (no poll past expiryMs → swept → fresh
 * register) remains covered by the p2/p11 lifecycle specs; this spec pins the
 * survival invariant the Redis backend adds.
 */

import { expect, test } from "@playwright/test";

import { newRunToken } from "./helpers/api-client";
import { ProtocolEnv } from "./helpers/protocol-env";
import { gotoSession, runRow } from "./helpers/ui";

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

test("P7: a relay restart leaves the session continuously listed (Redis-backed registry survives)", async ({ page }) => {
  env = await ProtocolEnv.start("p7-restart");
  const e = env;

  const fixture = await e.materialize("bare-host");
  const remote = await e.startRemote(fixture.target);
  const session = await e.api.waitForSession((candidate) => candidate.pid === remote.pid);
  e.note({ sessionId: session.session_id });
  const connectedAtBefore = session.connected_at;

  // Real history BEFORE the restart: one `version` run to terminal ok (durably
  // recorded in the per-project CLI store).
  const { runId } = await e.api.dispatchRun(session.session_id, {
    command: "version",
    fix: false,
    client_token: newRunToken(),
  });
  e.note({ runId });
  const done = await e.api.waitForRunTerminal(session.session_id, runId);
  expect(done.outcome).toBe("ok");

  // Restart the relay (same port). Under the in-process model the registry is
  // wiped; under Redis it survives untouched.
  await e.server.restart();

  // IMMEDIATELY after restart the session is STILL listed with the SAME
  // session_id, pid, AND connected_at — no re-register. Under the in-process
  // model it is gone (registry wiped) until the remote re-registers, and a
  // re-register would mint a NEW connected_at. Both branches fail this assertion
  // under the in-process relay.
  const sessionsAfter = await e.api.listSessions();
  const stillThere = sessionsAfter.find((candidate) => candidate.session_id === session.session_id);
  expect(stillThere, "session vanished across the relay restart").toBeDefined();
  expect(stillThere?.pid).toBe(remote.pid);
  expect(stillThere?.connected_at).toBe(connectedAtBefore);

  // History intact FROM THE CLI STORE: the pre-restart run is still there,
  // still terminal ok, reachable through the surviving session.
  const run = await e.api.getRun(session.session_id, runId);
  expect(run.state).toBe("terminal");
  expect(run.outcome).toBe("ok");

  await gotoSession(page, e.server.baseURL, session.session_id);
  await expect(runRow(page, runId)).toBeVisible({ timeout: 15_000 });
});
