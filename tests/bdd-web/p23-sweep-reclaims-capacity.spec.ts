/**
 * P23 (plan: connect-cli-to-vercel-harness; reviewer hardening for the p2
 * sweep-marker fix) — a swept session's abandoned in-flight work MUST NOT
 * permanently leak into the global inflight counter.
 *
 * The p2 regression fix made SWEEP_SCRIPT write a consume-once 404 marker for
 * each swept session's queued/delivered in-flight work so a trapped browser
 * request unblocks. The first cut of that fix left the work STATE and both
 * inflight SETs untouched — the SETs carry no TTL, so every swept session's
 * ghost ids would accumulate in `<p>:inflight:global` until, after enough
 * abandoned sessions, `globalQueueMax` is exhausted by ghosts and EVERY new
 * enqueue returns 503 forever. (Codex review r1 #1/#9.)
 *
 * The fix: sweep also retires the work (queued→cancelled, delivered→orphaned)
 * and SREMs it from both inflight sets, reclaiming capacity while preserving
 * the marker (p2) and the orphan late-reply path (p22 — whose orphan comes
 * from TIMEOUT, not sweep). This test pins the capacity-reclamation invariant:
 *
 *   1. fill the GLOBAL cap with a dead session's in-flight reads (no poll);
 *   2. let the session be swept (no poll ⇒ stale lastSeen ⇒ evicted);
 *   3. a NEW session's enqueue succeeds (not 503) — the ghosts were reclaimed.
 *
 * Fails under the leaking sweep: step 3 returns 503 capacity because the swept
 * session's ids still hold the global cap.
 */

import { expect, test } from "@playwright/test";

import {
  newRunToken,
  pollAgent,
  pollUntil,
  registerAgent,
  replyAgent,
  STATUS,
  type AgentRegisterBody,
} from "./helpers/api-client";
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

/**
 * Retains a browser request promise (the retained-promise pattern, Codex r2 #6)
 * so it can be resolved later; the no-op catch absorbs the socket reset if an
 * earlier assertion fails and teardown drops the server.
 */
function retain<T extends Promise<unknown>>(promise: T): T {
  promise.catch(() => {});
  return promise;
}

test("P23: sweeping a dead session reclaims its in-flight work from the global capacity counter", async () => {
  // GLOBAL cap = 1 so ONE abandoned read on the dead session exhausts it; the
  // per-session cap is left higher so only the GLOBAL counter is the blocker.
  env = await ProtocolEnv.start("p23-sweep-capacity", {
    serverEnv: {
      HARNESS_MAX_GLOBAL_QUEUE: "1",
      HARNESS_MAX_PER_SESSION_QUEUE: "8",
    },
  });
  const e = env;
  const fixture = await e.materialize("bare-host");
  const folder = fixture.target;
  const port = e.server.port;

  // ── Register a session whose agent NEVER polls (so its reads stay queued
  // and it goes stale ⇒ swept). A live CLI would claim the work and mask the
  // leak (Codex r1 #8). ──
  const deadId = `p23-${newRunToken()}`;
  const deadBody: AgentRegisterBody = {
    session_id: deadId,
    folder,
    canonical_folder: folder,
    pid: 232323,
    start_identity: "p23-dead",
    version: "0.0.0-p23",
  };
  const reg = await registerAgent(port, deadBody);
  expect(reg.status).toBe(STATUS.ok);

  // Fill the GLOBAL cap (1) with the dead session's in-flight read. Use the
  // DETAIL read (inventory 30s deadline) so it outlives EXPIRY_MS (10s) and is
  // still waiting when sweep evicts the session — a status read (5s deadline)
  // would time out (504) before the sweep runs. Retained (never polled) ⇒ it
  // stays queued and counted until sweep retires it.
  const fillA = retain(e.api.sessionDetailResponse(deadId));

  // PROVE the cap is full BEFORE waiting for the sweep (Codex r2 #3): a second
  // read on the same dead session is rejected 503 capacity. This deterministically
  // anchors the test — under the old leaking sweep the later live-session
  // enqueue would pass ONLY if the dead session truly held the cap. Awaiting
  // the overflow's response serializes it after fillA's enqueue; if fillA ever
  // lost the race the overflow would 200/504 and this assertion fails LOUDLY
  // (no false pass).
  const overflow = await e.api.sessionDetailResponse(deadId);
  expect(overflow.status()).toBe(STATUS.capacity_service);

  // ── Let the dead session be swept. EXPIRY_MS=10s of no-poll stale-ages the
  // session; listSessions() drives sweep(), which evicts it AND retires its
  // queued work (cancelled + SREM from both inflight sets) AND writes the 404
  // markers (the trapped reads resolve to 404 session_gone). ──
  await pollUntil(
    async () => (await e.api.listSessions()).every((candidate) => candidate.session_id !== deadId),
    { timeoutMs: 30_000, what: "the dead session to be swept out of the registry" },
  );

  // The trapped read must have unblocked (consume-once 404 marker) — the p2
  // invariant is preserved alongside the capacity fix.
  const a = await fillA;
  expect(a.status()).toBe(STATUS.session_gone);

  // ── A NEW session's enqueue MUST succeed: the dead session's ghost ids no
  // longer hold the global cap. Under the leaking sweep this returns 503
  // capacity (SCARD global still counts the two swept ids). ──
  const liveId = `p23-${newRunToken()}`;
  const liveBody: AgentRegisterBody = {
    session_id: liveId,
    folder,
    canonical_folder: folder,
    pid: 232324,
    start_identity: "p23-live",
    version: "0.0.0-p23",
  };
  const liveReg = await registerAgent(port, liveBody);
  expect(liveReg.status).toBe(STATUS.ok);
  const creds = JSON.parse(liveReg.body) as { connection_epoch: number; capability_token: string };
  const auth = {
    session_id: liveId,
    connection_epoch: creds.connection_epoch,
    capability_token: creds.capability_token,
  };

  // Retain the read; the raw agent claims it on the SAME server (proving the
  // enqueue was accepted — not 503). If capacity leaked, the retained read
  // resolves 503 and the poll returns empty (no work delivered).
  const read = retain(e.api.sessionStatusResponse(liveId));
  const claim = await pollAgent(port, auth);
  expect(claim.status).toBe(STATUS.ok);
  const work = JSON.parse(claim.body) as { work_id: string; type: string };
  expect(work.type).toBe("query");

  await replyAgent(
    port,
    { ...auth, work_id: work.work_id },
    JSON.stringify({ status: STATUS.ok, body: { session_id: liveId, active_run: null, runs: [] } }),
  );

  const resolved = await read;
  expect(resolved.status()).toBe(STATUS.ok);
});
