/**
 * P6 (plan §4.3, §3.8) — origin + host policy per route family.
 *
 * Browser mutation family (POST /api/sessions/:id/runs):
 *   foreign / missing / null Origin → 403; correct Origin + wrong
 *   Host → 403; correct both → 2xx/409-class (never 403).
 * Agent family (POST /api/agent/poll):
 *   any PRESENT foreign Origin → 403; absent Origin + loopback Host
 *   → accepted; wrong Host → 403.
 * GET family: wrong Host → 403.
 *
 * Playwright's APIRequestContext is used where it can control the
 * header (Origin); raw node:http is used where it cannot (Host
 * overrides, and truly ABSENT Origin — the shared client always sends
 * one, like the real UI).
 */

import { expect, test } from "@playwright/test";

import { newRunToken, rawHttpRequest } from "./helpers/api-client";
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

const FOREIGN_ORIGIN = "http://evil.example";
const WRONG_HOST = "harness.example.com";
const JSON_HEADERS = { "content-type": "application/json" };

test("P6: origin+host policy — browser mutation, agent, and GET families", async () => {
  env = await ProtocolEnv.start("p6-origin-host");
  const e = env;

  const fixture = await e.materialize("bare-host");
  const remote = await e.startRemote(fixture.target);
  const session = await e.api.waitForSession((candidate) => candidate.pid === remote.pid);
  e.note({ sessionId: session.id });

  const port = e.server.port;
  const goodOrigin = e.server.baseURL;
  const runsPath = `/api/sessions/${session.id}/runs`;
  const dispatchBody = () =>
    JSON.stringify({ command: "version", fix: false, client_token: newRunToken() });

  // ── Browser mutation family ──

  // Foreign Origin → 403.
  let response = await e.api.dispatchRunResponse(
    session.id,
    { command: "version", fix: false, client_token: newRunToken() },
    { origin: FOREIGN_ORIGIN },
  );
  expect(response.status()).toBe(403);

  // Origin: null (the literal serialization of opaque origins) → 403.
  response = await e.api.dispatchRunResponse(
    session.id,
    { command: "version", fix: false, client_token: newRunToken() },
    { origin: "null" },
  );
  expect(response.status()).toBe(403);

  // MISSING Origin (raw client — correct Host by default) → 403.
  let raw = await rawHttpRequest({
    port,
    method: "POST",
    path: runsPath,
    headers: JSON_HEADERS,
    body: dispatchBody(),
  });
  expect(raw.status).toBe(403);

  // Correct Origin + WRONG Host → 403.
  raw = await rawHttpRequest({
    port,
    method: "POST",
    path: runsPath,
    headers: { ...JSON_HEADERS, host: WRONG_HOST, origin: goodOrigin },
    body: dispatchBody(),
  });
  expect(raw.status).toBe(403);

  // Correct Origin + correct Host → accepted-class (2xx or 409), NEVER 403.
  response = await e.api.dispatchRunResponse(session.id, {
    command: "version",
    fix: false,
    client_token: newRunToken(),
  });
  expect(response.status()).not.toBe(403);
  expect([200, 201, 409]).toContain(response.status());

  // ── Agent family (POST /api/agent/poll) ──

  const pollBody = JSON.stringify({ session_id: session.id });

  // Any PRESENT foreign Origin → 403.
  raw = await rawHttpRequest({
    port,
    method: "POST",
    path: "/api/agent/poll",
    headers: { ...JSON_HEADERS, origin: FOREIGN_ORIGIN },
    body: pollBody,
  });
  expect(raw.status).toBe(403);

  // Absent Origin + loopback Host → accepted.
  raw = await rawHttpRequest({
    port,
    method: "POST",
    path: "/api/agent/poll",
    headers: JSON_HEADERS,
    body: pollBody,
  });
  expect(raw.status).toBe(200);

  // Wrong Host → 403.
  raw = await rawHttpRequest({
    port,
    method: "POST",
    path: "/api/agent/poll",
    headers: { ...JSON_HEADERS, host: WRONG_HOST },
    body: pollBody,
  });
  expect(raw.status).toBe(403);

  // ── GET family: Host check ──

  raw = await rawHttpRequest({
    port,
    method: "GET",
    path: "/api/sessions",
    headers: { host: WRONG_HOST },
  });
  expect(raw.status).toBe(403);

  raw = await rawHttpRequest({ port, method: "GET", path: "/api/sessions" });
  expect(raw.status).toBe(200);
});
