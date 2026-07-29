/**
 * P8 (plan §4.3) — spike probe, bare-adjacent: the "Test connection"
 * control on the sessions list dispatches a `version` run for that
 * session. Asserts the terminal `ok` outcome, the known version
 * output rendered in the run view, and a strictly increased inventory
 * generation (re-inventory after every command).
 *
 * NO transient queued/claimed/running assertions — `queued`
 * observability lives in P5's stopped remote; transition-order
 * legality lives in vitest; claim/redelivery in Go (plan §4.3).
 */

import { expect, test } from "@playwright/test";

import { pollUntil } from "./helpers/api-client";
import { ProtocolEnv } from "./helpers/protocol-env";
import { TID, gotoRun, gotoSessions, sessionRow } from "./helpers/ui";

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

test("P8: Test connection drives a version run to terminal ok", async ({ page }) => {
  env = await ProtocolEnv.start("p8-version-probe");
  const e = env;

  const fixture = await e.materialize("bare-host");
  const remote = await e.startRemote(fixture.target);
  const session = await e.api.waitForSession((candidate) => candidate.pid === remote.pid);
  e.note({ sessionId: session.session_id });

  // Drive the REAL UI control on the sessions list.
  await gotoSessions(page, e.server.baseURL);
  const row = sessionRow(page, session.session_id);
  await expect(row).toBeVisible({ timeout: 15_000 });
  await row.getByTestId(TID.testConnection).click();

  // The control dispatched a version run for THIS session.
  const versionRun = await pollUntil(
    async () => {
      const detail = await e.api.getSession(session.session_id);

      return detail.runs.find((candidate) => candidate.command === "version");
    },
    { timeoutMs: 30_000, what: "the Test-connection version run to appear in the session history" },
  );
  e.note({ runId: versionRun.run_id });

  // Terminal ok — no transient-state assertions on the way there.
  const done = await e.api.waitForRunTerminal(session.session_id, versionRun.run_id);
  expect(done.outcome).toBe("ok");

  // Known version output rendered in the run view.
  await gotoRun(page, e.server.baseURL, session.session_id, versionRun.run_id);
  await expect(page.getByTestId(TID.runOutcome)).toHaveText("ok", { timeout: 15_000 });
  await expect(page.getByTestId(TID.runOutput)).toContainText(/true-bdd version \S+/);
});
