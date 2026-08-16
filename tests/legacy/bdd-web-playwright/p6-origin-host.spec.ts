/**
 * P6 (plan §2/§3.8, REWRITTEN for v2) — origin + host policy per route
 * family. The browser mutation family is now SESSION-SCOPED
 * (POST /api/sessions/:id/runs); the agent family is the v2
 * register / poll / reply protocol (plan §2), origin-checked on ALL THREE
 * routes (critique §14).
 *
 * Browser mutation family:
 *   foreign / missing / null Origin → 403; correct Origin + wrong Host →
 *   403; correct both → 2xx/409-class (never 403).
 * Agent family (register / poll / reply):
 *   any PRESENT foreign Origin → 403; absent Origin + loopback Host →
 *   accepted; wrong Host → 403. Poll uses the ZERO-WAIT option so the raw
 *   helper never sits on the ≤5s hold.
 * GET family: wrong Host → 403.
 *
 * Playwright's APIRequestContext controls Origin where it can; raw
 * node:http is used where it cannot (Host overrides, truly ABSENT Origin,
 * and the agent register/poll/reply bodies).
 */

import { expect, test } from "@playwright/test";

import { newRunToken, pollAgent, rawHttpRequest, registerAgent, replyAgent, STATUS } from "./helpers/api-client";
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

const FOREIGN_ORIGIN = "http://evil.example";
const WRONG_HOST = "harness.example.com";
const JSON_HEADERS = { "content-type": "application/json" };

test("P6: origin+host policy — browser mutation, agent register/poll/reply, and GET families", async () => {
  env = await ProtocolEnv.start("p6-origin-host");
  const e = env;

  const fixture = await e.materialize("bare-host");
  const remote = await e.startRemote(fixture.target);
  const session = await e.api.waitForSession((candidate) => candidate.pid === remote.pid);
  e.note({ sessionId: session.session_id });

  const port = e.server.port;
  const goodOrigin = e.server.baseURL;
  const runsPath = `/api/sessions/${session.session_id}/runs`;
  const dispatchBody = () => JSON.stringify({ command: "version", fix: false, client_token: newRunToken() });

  // ── Browser mutation family (session-scoped runs) ──

  let response = await e.api.dispatchRunResponse(
    session.session_id,
    { command: "version", fix: false, client_token: newRunToken() },
    { origin: FOREIGN_ORIGIN },
  );
  expect(response.status()).toBe(STATUS.forbidden);

  response = await e.api.dispatchRunResponse(
    session.session_id,
    { command: "version", fix: false, client_token: newRunToken() },
    { origin: "null" },
  );
  expect(response.status()).toBe(STATUS.forbidden);

  // MISSING Origin (raw client — correct Host by default) → 403.
  let raw = await rawHttpRequest({ port, method: "POST", path: runsPath, headers: JSON_HEADERS, body: dispatchBody() });
  expect(raw.status).toBe(STATUS.forbidden);

  // Correct Origin + WRONG Host → 403.
  raw = await rawHttpRequest({
    port,
    method: "POST",
    path: runsPath,
    headers: { ...JSON_HEADERS, host: WRONG_HOST, origin: goodOrigin },
    body: dispatchBody(),
  });
  expect(raw.status).toBe(STATUS.forbidden);

  // Correct Origin + correct Host → accepted-class, NEVER 403.
  response = await e.api.dispatchRunResponse(session.session_id, {
    command: "version",
    fix: false,
    client_token: newRunToken(),
  });
  expect(response.status()).not.toBe(STATUS.forbidden);

  // ── Agent register family ──

  const registerBody = {
    session_id: `p6-probe-${newRunToken()}`,
    folder: fixture.target,
    canonical_folder: fixture.target,
    pid: 424242,
    start_identity: "p6-identity",
    version: "0.0.0-test",
  };

  // Foreign Origin → 403.
  let reg = await registerAgent(port, registerBody, { origin: FOREIGN_ORIGIN });
  expect(reg.status).toBe(STATUS.forbidden);

  // Wrong Host → 403.
  reg = await registerAgent(port, registerBody, { host: WRONG_HOST });
  expect(reg.status).toBe(STATUS.forbidden);

  // Absent Origin + loopback Host → accepted; capture the epoch + token.
  reg = await registerAgent(port, registerBody);
  expect(reg.status).toBe(STATUS.ok);
  const creds = JSON.parse(reg.body) as { connection_epoch: number; capability_token: string };

  // ── Agent poll family (zero-wait so the raw helper never blocks) ──

  const pollBase = {
    session_id: registerBody.session_id,
    connection_epoch: creds.connection_epoch,
    capability_token: creds.capability_token,
    wait: false,
  };

  // Foreign Origin → 403.
  let poll = await pollAgent(port, pollBase, { origin: FOREIGN_ORIGIN });
  expect(poll.status).toBe(STATUS.forbidden);

  // Wrong Host → 403.
  poll = await pollAgent(port, pollBase, { host: WRONG_HOST });
  expect(poll.status).toBe(STATUS.forbidden);

  // Absent Origin + loopback Host + valid creds → NOT 403 (204 nothing, or
  // 200 with work — an accepted request may fail business, never 403).
  poll = await pollAgent(port, pollBase);
  expect(poll.status).not.toBe(STATUS.forbidden);
  expect([STATUS.no_content, STATUS.ok]).toContain(poll.status);

  // ── Agent reply family ──

  const correlation = {
    session_id: registerBody.session_id,
    connection_epoch: creds.connection_epoch,
    capability_token: creds.capability_token,
    work_id: "p6-no-such-work",
  };

  // Foreign Origin → 403 (decided BEFORE the unknown-work business error).
  let reply = await replyAgent(port, correlation, JSON.stringify({ ok: true }), { origin: FOREIGN_ORIGIN });
  expect(reply.status).toBe(STATUS.forbidden);

  // Absent Origin + loopback Host → NOT 403 (may then fail business).
  reply = await replyAgent(port, correlation, JSON.stringify({ ok: true }));
  expect(reply.status).not.toBe(STATUS.forbidden);

  // ── GET family: Host check only ──

  raw = await rawHttpRequest({ port, method: "GET", path: "/api/sessions", headers: { host: WRONG_HOST } });
  expect(raw.status).toBe(STATUS.forbidden);

  raw = await rawHttpRequest({ port, method: "GET", path: "/api/sessions" });
  expect(raw.status).toBe(STATUS.ok);
});
