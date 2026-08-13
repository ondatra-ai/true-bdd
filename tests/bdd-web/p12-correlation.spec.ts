/**
 * P12 (plan §2/§3, NEW for v2) — reply correlation. Every browser read is a
 * distinct CLI work item; concurrent status / run-detail / inventory reads
 * must each receive THEIR OWN reply, never another request's (critique
 * "add a relay correlation case"). Each work item carries a work_id, and
 * every reply echoes it — the relay routes replies by correlation, never by
 * arrival order.
 *
 * Driven with a real live remote: a batch of concurrent, DIFFERENT reads is
 * fired repeatedly and each response is checked to match its own request
 * (the session_status names the requested session, the run-detail names the
 * requested run, the detail carries an inventory).
 */

import { expect, test } from "@playwright/test";

import { newRunToken } from "./helpers/api-client";
import { ProtocolEnv } from "./helpers/protocol-env";

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

test("P12: concurrent status/run/inventory reads never receive each other's replies", async () => {
  env = await ProtocolEnv.start("p12-correlation");
  const e = env;

  const fixture = await e.materialize("bare-host");
  const remote = await e.startRemote(fixture.target);
  const session = await e.api.waitForSession((candidate) => candidate.pid === remote.pid);
  e.note({ sessionId: session.session_id });

  // A terminal run to read back concurrently with status/inventory.
  const { runId } = await e.api.dispatchRun(session.session_id, {
    command: "version",
    fix: false,
    client_token: newRunToken(),
  });
  e.note({ runId });
  await e.api.waitForRunTerminal(session.session_id, runId);

  // Fire many rounds of three DIFFERENT concurrent reads. If the relay ever
  // mis-routed a reply, a payload would carry the wrong session/run id.
  for (let round = 0; round < 12; round += 1) {
    const [status, detail, run] = await Promise.all([
      e.api.getSessionStatus(session.session_id),
      e.api.getSession(session.session_id),
      e.api.getRun(session.session_id, runId),
    ]);

    // The session_status reply is correlated to the requested session.
    expect(status.session_id).toBe(session.session_id);

    // The session_detail reply carries the inventory (the detail work item),
    // and its own session id — not the status or run work item's payload.
    expect(detail.session_id).toBe(session.session_id);
    expect(detail.inventory).not.toBeNull();

    // The run-detail reply is correlated to the requested run and session.
    expect(run.run_id).toBe(runId);
    expect(run.session_id).toBe(session.session_id);
  }
});
