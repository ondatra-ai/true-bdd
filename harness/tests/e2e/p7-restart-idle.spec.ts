/**
 * P7 (plan §1.7/§2, REWRITTEN for v2) — relay restart, re-register with the
 * SAME client-owned session id, history intact from the CLI store.
 *
 * The relay is STATELESS (no DB). When it restarts, its in-memory registry
 * is empty, so the remote's next poll gets 404 / stale-epoch and it
 * RE-REGISTERS using the SAME client-owned (process-stable) session id
 * (critique §5) — never a duplicate session. The run history is NOT in the
 * relay; it lives in the per-project CLI store, so reading the session
 * after the restart returns the pre-restart run straight from the CLI DB.
 */

import { expect, test } from "@playwright/test";

import { newRunToken, pollUntil } from "./helpers/api-client";
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

test("P7: relay restart re-registers the SAME session id; CLI-owned history survives", async ({ page }) => {
  env = await ProtocolEnv.start("p7-restart");
  const e = env;

  const fixture = await e.materialize("bare-host");
  const remote = await e.startRemote(fixture.target);
  const session = await e.api.waitForSession((candidate) => candidate.pid === remote.pid);
  e.note({ sessionId: session.session_id });

  // Real history BEFORE the restart: one `version` run to terminal ok
  // (durably recorded in the per-project CLI store).
  const { runId } = await e.api.dispatchRun(session.session_id, {
    command: "version",
    fix: false,
    client_token: newRunToken(),
  });
  e.note({ runId });

  const done = await e.api.waitForRunTerminal(session.session_id, runId);
  expect(done.outcome).toBe("ok");

  const portBefore = e.server.port;

  // Restart the STATELESS relay (same port). Its registry is wiped.
  await e.server.restart();
  expect(e.server.port).toBe(portBefore);

  // The remote's poll gets 404 from the fresh relay and re-registers with
  // the SAME client-owned session id, so it lands in the SAME session — no
  // duplicate.
  const after = await pollUntil(
    async () => (await e.api.listSessions()).find((candidate) => candidate.session_id === session.session_id),
    { timeoutMs: 60_000, what: "the SAME client-owned session id to re-register after the relay restart" },
  );
  expect(after.session_id).toBe(session.session_id);
  expect(after.pid).toBe(remote.pid);

  const sessions = await e.api.listSessions();
  expect(sessions).toHaveLength(1);

  // History intact FROM THE CLI STORE: the pre-restart run is still there,
  // still terminal ok, reachable through the re-registered session.
  const run = await e.api.getRun(session.session_id, runId);
  expect(run.state).toBe("terminal");
  expect(run.outcome).toBe("ok");

  const detail = await e.api.getSession(session.session_id);
  expect(detail.runs.map((entry) => entry.run_id)).toContain(runId);

  await gotoSession(page, e.server.baseURL, session.session_id);
  await expect(runRow(page, runId)).toBeVisible({ timeout: 15_000 });
});
