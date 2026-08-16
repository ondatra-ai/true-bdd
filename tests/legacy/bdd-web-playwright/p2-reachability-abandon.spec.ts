/**
 * P2 (plan §1.7/§3, v2; reworked for `home-sessions-list` H1) — sessions are
 * GONE on disconnect, asserted as PROTOCOL FACTS only.
 *
 * v1 modelled a lingering "unreachable" session with a queued run and a
 * "Mark abandoned" control. v2 deletes all of that: the relay is a stateless
 * registry, an open poll is the only liveness signal, and a session that stops
 * polling for the disconnect threshold is REMOVED atomically. This spec asserts
 * the honest v2 disconnect at the API:
 *   - a SIGSTOPped remote (its in-flight poll can never be renewed) drops out of
 *     `GET /api/sessions` after the threshold;
 *   - the session- and run-scoped endpoints then 404 `session_gone` — the run
 *     lives in the CLI DB but is not reachable through a dead session.
 *
 * The UI live-drop half (an already-open page CLEARS its stale data) was retired
 * here and RE-HOMED, upgraded, to the list-level `w18.5` (a stopped CLI vanishes
 * live from `/`, structurally, with no marker). p2 is now API-only.
 */

import { expect, test } from "@playwright/test";

import { newRunToken, STATUS } from "./helpers/api-client";
import { ProtocolEnv } from "./helpers/protocol-env";

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

test("P2: a disconnected remote leaves the registry; session- and run-scoped endpoints 404", async () => {
  env = await ProtocolEnv.start("p2-disconnect");
  const e = env;

  const fixture = await e.materialize("bare-host");
  const remote = await e.startRemote(fixture.target);
  const session = await e.api.waitForSession((candidate) => candidate.pid === remote.pid);
  e.note({ sessionId: session.session_id });

  // A run in flight before the disconnect — its id is used to prove the
  // run-scoped endpoint 404s once the session is gone. `version` stays in the
  // command enum, still exercised here by this in-flight dispatch.
  const { runId } = await e.api.dispatchRun(session.session_id, {
    command: "version",
    fix: false,
    client_token: newRunToken(),
  });
  e.note({ runId });

  // Freeze the remote: its in-flight poll can never be renewed, so the relay
  // expires the session after the disconnect threshold.
  remote.signal("SIGSTOP");

  // The session DROPS OUT of the registry (every listed session is connected by
  // definition — plan §3).
  await e.api.waitForSessionGone((candidate) => candidate.pid === remote.pid);
  const stillListed = (await e.api.listSessions()).some((s) => s.session_id === session.session_id);
  expect(stillListed).toBe(false);

  // Session- AND run-scoped endpoints now 404 session_gone (critique §4): the
  // run persists in the CLI DB, but a dead session cannot serve it.
  expect((await e.api.sessionDetailResponse(session.session_id)).status()).toBe(STATUS.session_gone);
  expect((await e.api.sessionStatusResponse(session.session_id)).status()).toBe(STATUS.session_gone);
  expect((await e.api.runDetailResponse(session.session_id, runId)).status()).toBe(STATUS.session_gone);
});
