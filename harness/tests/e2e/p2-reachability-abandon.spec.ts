/**
 * P2 (plan §4.3) — reachability + mark abandoned: SIGSTOP a
 * registered remote, dispatch `version` while the session still reads
 * "connected", SIGKILL before any claim. The queued run must stay
 * non-terminal (heartbeat silence affects REACHABILITY only — never
 * run state), the unreachable overlay must appear after the
 * threshold, and the real "Mark abandoned" control must flip the run
 * to `abandoned`.
 */

import { expect, test } from "@playwright/test";

import { newRunToken } from "./helpers/api-client";
import { ProtocolEnv } from "./helpers/protocol-env";
import { TID, gotoRun } from "./helpers/ui";

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

test("P2: queued run survives remote death; mark-abandoned flips it", async ({ page }) => {
  env = await ProtocolEnv.start("p2-reachability");
  const e = env;

  const fixture = await e.materialize("bare-host");
  const remote = await e.startRemote(fixture.target);
  const session = await e.api.waitForSession((candidate) => candidate.pid === remote.pid);
  e.note({ sessionId: session.id });

  // Freeze the remote AFTER registration — the server cannot know yet.
  remote.signal("SIGSTOP");

  // Dispatch while the session still reads "connected" (the
  // reachability threshold is >= 5s; see helpers/README-testids.md).
  const before = await e.api.getSession(session.id);
  expect(before.reachability).toBe("connected");

  const { runId, status } = await e.api.dispatchRun(session.id, {
    command: "version",
    fix: false,
    client_token: newRunToken(),
  });
  expect(status).toBe(201);
  e.note({ runId });

  // Kill before any claim is possible (the remote is stopped, so it
  // never posted an event for this run).
  remote.signal("SIGKILL");
  await remote.waitForExit(10_000);

  let run = await e.api.getRun(runId);
  expect(run.state).toBe("queued");
  expect(run.outcome).toBeNull();

  // Heartbeat silence flips reachability after the threshold...
  await e.api.waitForReachability(session.id, "unreachable");

  // ...but NEVER the run state.
  run = await e.api.getRun(runId);
  expect(run.state).toBe("queued");
  expect(run.outcome).toBeNull();

  // Run view: unreachable overlay + the real "Mark abandoned" control.
  await gotoRun(page, e.server.baseURL, runId);
  await expect(page.getByTestId(TID.reachabilityOverlay)).toBeVisible({ timeout: 15_000 });
  await expect(page.getByTestId(TID.runState)).toHaveText("queued");

  const control = page.getByTestId(TID.markAbandoned);
  await expect(control).toBeVisible();
  await control.click();

  await expect(page.getByTestId(TID.runOutcome)).toHaveText("abandoned", { timeout: 15_000 });

  run = await e.api.getRun(runId);
  expect(run.state).toBe("terminal");
  expect(run.outcome).toBe("abandoned");
});
