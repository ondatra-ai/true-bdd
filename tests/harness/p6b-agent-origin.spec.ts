/**
 * P6b (plan §2/§3.8, FULL REWRITE for v2) — the v2 agent routes are the
 * register / poll / reply protocol (plan §2); the deleted `/agent/inventory`
 * and `/agent/events` routes are gone (plan §6). This spec hardens all
 * three v2 agent routes over RAW HTTP:
 *
 *   - `Origin: null` (the opaque-origin serialization) → 403 on every
 *     agent route. Only a genuinely ABSENT Origin is trusted.
 *   - A `text/plain` body carrying JSON → 415, BEFORE any state access, so
 *     a cross-origin "simple request" CSRF vector can never reach the relay.
 *   - Absent Origin + loopback Host + application/json → NOT 403.
 *   - An over-cap REPLY body → a browser-visible, CORRELATED 413
 *     `reply_too_large` (plan §2 — auth/correlation live in headers OUTSIDE
 *     the capped body, so an over-cap body never degrades to a blind 504).
 *
 * It never resolves or executes `claude` (protocol project).
 */

import { expect, test } from "@playwright/test";

import { newRunToken, rawHttpRequest, registerAgent, STATUS } from "./helpers/api-client";
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
const JSON_HEADERS = { "content-type": "application/json" };

test("P6b: register/poll/reply reject Origin:null and non-JSON content-type; over-cap reply → correlated 413", async () => {
  env = await ProtocolEnv.start("p6b-agent-origin");
  const e = env;

  const fixture = await e.materialize("bare-host");
  const remote = await e.startRemote(fixture.target);
  const session = await e.api.waitForSession((candidate) => candidate.pid === remote.pid);
  e.note({ sessionId: session.session_id });

  const port = e.server.port;

  const registerBody = {
    session_id: `p6b-probe-${newRunToken()}`,
    folder: fixture.target,
    canonical_folder: fixture.target,
    pid: 525252,
    start_identity: "p6b-identity",
    version: "0.0.0-test",
  };
  const pollBody = { session_id: registerBody.session_id, connection_epoch: 1, capability_token: "x", wait: false };
  const replyBody = { ok: true };

  const routes = [
    { path: "/api/agent/register", body: JSON.stringify(registerBody) },
    { path: "/api/agent/poll", body: JSON.stringify(pollBody) },
    { path: "/api/agent/reply", body: JSON.stringify(replyBody) },
  ];

  for (const route of routes) {
    // Origin: null (opaque) → 403, even on a loopback Host.
    let raw = await rawHttpRequest({
      port,
      method: "POST",
      path: route.path,
      headers: { ...JSON_HEADERS, origin: "null" },
      body: route.body,
    });
    expect(raw.status, `${route.path} Origin:null`).toBe(STATUS.forbidden);

    // Present foreign Origin → 403.
    raw = await rawHttpRequest({
      port,
      method: "POST",
      path: route.path,
      headers: { ...JSON_HEADERS, origin: FOREIGN_ORIGIN },
      body: route.body,
    });
    expect(raw.status, `${route.path} foreign Origin`).toBe(STATUS.forbidden);

    // A text/plain body carrying JSON is rejected 415 BEFORE state access.
    raw = await rawHttpRequest({
      port,
      method: "POST",
      path: route.path,
      headers: { "content-type": "text/plain" },
      body: route.body,
    });
    expect(raw.status, `${route.path} text/plain`).toBe(STATUS.unsupported_media);

    // Absent Origin + loopback Host + application/json → NOT 403.
    raw = await rawHttpRequest({ port, method: "POST", path: route.path, headers: JSON_HEADERS, body: route.body });
    expect(raw.status, `${route.path} absent Origin`).not.toBe(STATUS.forbidden);
  }

  // ── Over-cap reply → correlated, browser-visible 413 ──

  const reg = await registerAgent(port, registerBody);
  expect(reg.status).toBe(STATUS.ok);
  const creds = JSON.parse(reg.body) as {
    connection_epoch: number;
    capability_token: string;
    reply_budget_bytes: number;
  };
  expect(typeof creds.reply_budget_bytes).toBe("number");

  // A body one byte over the negotiated cap. The correlation (work_id,
  // epoch, token) lives in HEADERS, so the streamed size cap fires and the
  // relay returns a CORRELATED 413 (never a blind 504).
  const overCap = "x".repeat(creds.reply_budget_bytes + 1);
  const reply = await rawHttpRequest({
    port,
    method: "POST",
    path: "/api/agent/reply",
    headers: {
      ...JSON_HEADERS,
      "x-session-id": registerBody.session_id,
      "x-connection-epoch": String(creds.connection_epoch),
      "x-capability-token": creds.capability_token,
      "x-work-id": "p6b-work",
    },
    body: overCap,
  });
  expect(reply.status).toBe(STATUS.reply_too_large);
});
