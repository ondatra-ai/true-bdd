/**
 * P11 (plan §2/§3, NEW for v2) — relay lifecycle + status mapping.
 *
 * The stateless relay is a register/poll/reply bus with explicit lifecycle
 * and status semantics (critique §4). This spec drives the raw agent
 * protocol to pin:
 *   - exact 404-vs-504 mapping: an unknown session read → 404 session_gone;
 *     a registered-but-non-replying session read → 504 cli_timeout at the
 *     operation deadline (registered ≠ answering);
 *   - a missing / wrong capability token on poll → rejected (never work);
 *   - epoch invalidation: a fresh register bumps the connection_epoch and
 *     invalidates the old one — a poll/reply on the stale epoch is rejected
 *     (late replies from a stale epoch never land);
 *   - the browser-visible 413 over-cap reply (also in P6b) is a CORRELATED
 *     failure, not a blind 504.
 *
 * The atomic expiry semantics (queued-waiter fail, delivered-query cancel,
 * client-abort drops undelivered mutations) are unit-tested against the
 * relay's pure registry logic (plan §5 vitest); this spec covers the
 * HTTP-observable surface.
 */

import { expect, test } from "@playwright/test";

import { newRunToken, pollAgent, registerAgent, replyAgent, STATUS } from "./helpers/api-client";
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

test("P11: 404-vs-504 mapping, capability-token + epoch rejection, correlated 413", async () => {
  env = await ProtocolEnv.start("p11-relay-lifecycle");
  const e = env;

  const fixture = await e.materialize("bare-host");
  const port = e.server.port;

  // ── 404 session_gone: a read for a session that never existed ──
  expect((await e.api.sessionStatusResponse(`p11-nope-${newRunToken()}`)).status()).toBe(STATUS.session_gone);
  expect((await e.api.runDetailResponse(`p11-nope-${newRunToken()}`, "no-run")).status()).toBe(STATUS.session_gone);

  // ── 504 cli_timeout: registered but never polls/replies ──
  // A probe agent registers (so the session EXISTS in the registry) but
  // never opens a poll, so no CLI ever serves the enqueued session_status
  // work item. The browser read hits its operation deadline → 504.
  const probeId = `p11-silent-${newRunToken()}`;
  const reg = await registerAgent(port, {
    session_id: probeId,
    folder: fixture.target,
    canonical_folder: fixture.target,
    pid: 616161,
    start_identity: "p11-silent-identity",
    version: "0.0.0-test",
  });
  expect(reg.status).toBe(STATUS.ok);
  const creds = JSON.parse(reg.body) as {
    connection_epoch: number;
    capability_token: string;
    reply_budget_bytes: number;
  };

  // The read must resolve within the 5s status deadline (< 10s expiry), so
  // the "registered but not answering" state maps to 504, not 404.
  expect((await e.api.sessionStatusResponse(probeId)).status()).toBe(STATUS.cli_timeout);

  // ── Capability-token rejection on poll ──
  const goodPoll = { session_id: probeId, connection_epoch: creds.connection_epoch, capability_token: creds.capability_token, wait: false };
  const wrongToken = await pollAgent(port, { ...goodPoll, capability_token: "wrong-token" });
  expect(wrongToken.status).toBeGreaterThanOrEqual(400);
  expect([STATUS.no_content, STATUS.ok]).not.toContain(wrongToken.status);

  // ── Epoch invalidation: a fresh register bumps the epoch ──
  const reReg = await registerAgent(port, {
    session_id: probeId,
    folder: fixture.target,
    canonical_folder: fixture.target,
    pid: 616161,
    start_identity: "p11-silent-identity",
    version: "0.0.0-test",
  });
  expect(reReg.status).toBe(STATUS.ok);
  const creds2 = JSON.parse(reReg.body) as { connection_epoch: number; capability_token: string };
  expect(creds2.connection_epoch).toBeGreaterThan(creds.connection_epoch);

  // A poll on the STALE epoch is rejected (never returns work).
  const stalePoll = await pollAgent(port, { ...goodPoll });
  expect(stalePoll.status).toBeGreaterThanOrEqual(400);
  expect([STATUS.no_content, STATUS.ok]).not.toContain(stalePoll.status);

  // A late reply on the STALE epoch is rejected too.
  const staleReply = await replyAgent(
    port,
    { session_id: probeId, connection_epoch: creds.connection_epoch, capability_token: creds.capability_token, work_id: "p11-stale-work" },
    JSON.stringify({ ok: true }),
  );
  expect(staleReply.status).toBeGreaterThanOrEqual(400);
  expect([STATUS.no_content, STATUS.ok]).not.toContain(staleReply.status);

  // ── Correlated over-cap 413 (auth in headers, size cap on the body) ──
  const overCap = await replyAgent(
    port,
    { session_id: probeId, connection_epoch: creds2.connection_epoch, capability_token: creds2.capability_token, work_id: "p11-work" },
    "x".repeat(creds.reply_budget_bytes + 1),
  );
  expect(overCap.status).toBe(STATUS.reply_too_large);
});
