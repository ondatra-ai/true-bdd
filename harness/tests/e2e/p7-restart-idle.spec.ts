/**
 * P7 (plan §4.3) — restart idle: within ONE test, create real
 * `version` run history, restart the server via the controller (SAME
 * DB file + SAME port), and assert the remote re-registers into the
 * SAME session (incarnation match — no duplicate session) with the
 * history intact.
 */

import { expect, test } from "@playwright/test";

import { newRunToken, pollUntil } from "./helpers/api-client";
import { ProtocolEnv } from "./helpers/protocol-env";
import { gotoSession, runRow } from "./helpers/ui";

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

test("P7: restart with same DB+port keeps the session identity and history", async ({ page }) => {
  env = await ProtocolEnv.start("p7-restart");
  const e = env;

  const fixture = await e.materialize("bare-host");
  const remote = await e.startRemote(fixture.target);
  const session = await e.api.waitForSession((candidate) => candidate.pid === remote.pid);
  e.note({ sessionId: session.id });

  // Real history BEFORE the restart: one `version` run to terminal ok.
  const { runId } = await e.api.dispatchRun(session.id, {
    command: "version",
    fix: false,
    client_token: newRunToken(),
  });
  e.note({ runId });

  const done = await e.api.waitForRunTerminal(runId);
  expect(done.outcome).toBe("ok");

  const portBefore = e.server.port;

  // Stop + start against the SAME DB and port (plan §4.1 controller).
  await e.server.restart();
  expect(e.server.port).toBe(portBefore);

  // The remote's poll loop re-registers with the SAME incarnation id,
  // so it lands in the SAME session — connected again, no duplicate.
  const after = await pollUntil(
    async () => {
      const sessions = await e.api.listSessions();

      return sessions.find(
        (candidate) => candidate.id === session.id && candidate.reachability === "connected",
      );
    },
    { timeoutMs: 60_000, what: "the SAME session to re-register as connected after restart" },
  );
  expect(after.id).toBe(session.id);

  const sessions = await e.api.listSessions();
  expect(sessions).toHaveLength(1);

  // History intact: the pre-restart run is still there, still terminal ok.
  const run = await e.api.getRun(runId);
  expect(run.state).toBe("terminal");
  expect(run.outcome).toBe("ok");

  const detail = await e.api.getSession(session.id);
  expect(detail.runs.map((entry) => entry.id)).toContain(runId);

  await gotoSession(page, e.server.baseURL, session.id);
  await expect(runRow(page, runId)).toBeVisible({ timeout: 15_000 });
});
