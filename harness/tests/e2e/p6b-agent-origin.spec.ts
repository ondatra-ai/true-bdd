/**
 * P6b (plan §3.8 / finding 3) — the agent-route hardening the original P6
 * did not cover, exercised over RAW HTTP against the two most consequential
 * agent routes (inventory needs neither a valid session nor a run id;
 * events mutates run state):
 *
 *   - `Origin: null` (the opaque-origin serialization) → 403. Only a
 *     genuinely ABSENT Origin is trusted; a present `null` has no matching
 *     loopback authority.
 *   - A `text/plain` body carrying JSON → 415, BEFORE any state access, so
 *     a cross-origin "simple request" CSRF vector can never reach the store.
 *   - Absent Origin + loopback Host + application/json → NOT 403 (an
 *     accepted request may then fail with a business 4xx, never 403).
 *
 * This is ADDITIVE (a new spec file); it does not touch the existing P6
 * assertions. It never resolves or executes `claude` (protocol project).
 */

import { expect, test } from "@playwright/test";

import { rawHttpRequest } from "./helpers/api-client";
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

test("P6b: agent routes reject Origin:null and non-JSON content-type (inventory + events)", async () => {
  env = await ProtocolEnv.start("p6b-agent-origin");
  const e = env;

  const fixture = await e.materialize("bare-host");
  const remote = await e.startRemote(fixture.target);
  const session = await e.api.waitForSession((candidate) => candidate.pid === remote.pid);
  e.note({ sessionId: session.id });

  const port = e.server.port;
  const inventoryBody = JSON.stringify({
    session_id: session.id,
    canonical_folder: fixture.target,
    scan_epoch: 1,
    snapshot: {},
    client_token: "p6b-token",
  });
  const eventsBody = JSON.stringify({ session_id: session.id, run_id: "no-such-run", events: [] });

  for (const route of [
    { path: "/api/agent/inventory", body: inventoryBody },
    { path: "/api/agent/events", body: eventsBody },
  ]) {
    // Origin: null (opaque) → 403, even on a loopback Host.
    let raw = await rawHttpRequest({
      port,
      method: "POST",
      path: route.path,
      headers: { ...JSON_HEADERS, origin: "null" },
      body: route.body,
    });
    expect(raw.status, `${route.path} Origin:null`).toBe(403);

    // Present foreign Origin → 403.
    raw = await rawHttpRequest({
      port,
      method: "POST",
      path: route.path,
      headers: { ...JSON_HEADERS, origin: FOREIGN_ORIGIN },
      body: route.body,
    });
    expect(raw.status, `${route.path} foreign Origin`).toBe(403);

    // A text/plain body carrying JSON passes origin (absent Origin) but is
    // rejected 415 BEFORE any state access — never a 2xx.
    raw = await rawHttpRequest({
      port,
      method: "POST",
      path: route.path,
      headers: { "content-type": "text/plain" },
      body: route.body,
    });
    expect(raw.status, `${route.path} text/plain`).toBe(415);

    // Absent Origin + loopback Host + application/json → NOT 403.
    raw = await rawHttpRequest({
      port,
      method: "POST",
      path: route.path,
      headers: JSON_HEADERS,
      body: route.body,
    });
    expect(raw.status, `${route.path} absent Origin`).not.toBe(403);
  }
});
