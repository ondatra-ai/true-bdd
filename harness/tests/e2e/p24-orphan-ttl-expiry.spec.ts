/**
 * P24 (plan: connect-cli-to-vercel-harness; Codex r1 #9 hardening) — after the
 * orphan TTL elapses, the work hash is gone and a late CLI reply is rejected
 * with `unknown_work` (the TTL boundary that p22 doesn't cover — p22 only
 * exercises replies WITHIN the orphan window).
 *
 * Why this matters: the orphaned work hash (`tb:work:<wid>`) carries
 * ORPHAN_TTL_SEC so capacity is reclaimed if no late reply ever lands. If that
 * TTL regressed (e.g. extended indefinitely, or shortened to break the
 * within-window late-reply path), p22 alone wouldn't catch it — p22 only
 * asserts ONE late reply succeeds within the window, never that an
 * AFTER-window reply is deterministically rejected with the work record gone.
 *
 * The coder exposes `HARNESS_ORPHAN_TTL_SEC` so this test shrinks the window
 * to 1s and advances past it before the late reply.
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
import { redisExec } from "./helpers/redis";

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

function retain<T extends Promise<unknown>>(promise: T): T {
  promise.catch(() => {});
  return promise;
}

test("P24: a late reply after the orphan TTL is rejected unknown_work (work hash gone)", async () => {
  // Shrink the orphan window to 1s so the test advances past it quickly. The
  // within-window happy path (one late reply accepted) stays in p22.
  env = await ProtocolEnv.start("p24-orphan-ttl", {
    serverEnv: { HARNESS_ORPHAN_TTL_SEC: "1" },
  });
  const e = env;
  const fixture = await e.materialize("bare-host");
  const folder = fixture.target;
  const port = e.server.port;

  const sessionId = `p24-${newRunToken()}`;
  const body: AgentRegisterBody = {
    session_id: sessionId,
    folder,
    canonical_folder: folder,
    pid: 242424,
    start_identity: "p24-raw",
    version: "0.0.0-p24",
  };
  const reg = await registerAgent(port, body);
  expect(reg.status).toBe(STATUS.ok);
  const creds = JSON.parse(reg.body) as { connection_epoch: number; capability_token: string };
  const auth = {
    session_id: sessionId,
    connection_epoch: creds.connection_epoch,
    capability_token: creds.capability_token,
  };

  // Dispatch (retained); claim on the agent (delivered); let the browser
  // mutation deadline elapse → 504 cli_timeout (the delivered→orphaned path).
  const token = newRunToken();
  const pending = retain(e.api.dispatchRunResponse(sessionId, { command: "version", fix: false, client_token: token }));
  const claimPoll = await pollAgent(port, auth);
  expect(claimPoll.status).toBe(STATUS.ok);
  const claimed = JSON.parse(claimPoll.body) as { work_id: string };
  const timedOut = await pending;
  expect(timedOut.status()).toBe(STATUS.cli_timeout);

  // Pin the post-timeout state BEFORE waiting for TTL expiry (Codex r2 #3): a
  // regression where TIMEOUT immediately deleted the work hash would yield the
  // same 504 above and the same downstream EXISTS=0 — this test would pass
  // without exercising TTL. Assert the work is in `orphaned` state with a
  // positive TTL bounded by the configured 1s window FIRST.
  const workKey = `${e.prefix}:work:${claimed.work_id}`;
  expect(await redisExec(["HGET", workKey, "state"])).toBe("orphaned");
  const ttlAfterTimeout = Number.parseInt(await redisExec(["TTL", workKey]), 10);
  expect(ttlAfterTimeout).toBeGreaterThan(0);
  expect(ttlAfterTimeout).toBeLessThanOrEqual(1);

  // Wait until the work hash is gone — proves the orphan TTL fired (the work
  // record was reclaimed). `redis-cli EXISTS` returning 0 is the deterministic
  // boundary signal; polling avoids coupling to wall-clock precision.
  await pollUntil(
    async () => {
      const result = Number.parseInt(await redisExec(["EXISTS", workKey]), 10);
      return result === 0 ? true : undefined;
    },
    { timeoutMs: 15_000, what: `orphan TTL to expire and delete ${workKey}` },
  );

  // After the orphan TTL, the late CLI reply is rejected unknown_work: the work
  // hash is gone, so RESOLVE_SCRIPT's `HGET whash session_id` returns false.
  // The reply marker is NOT written and the body is NOT retained.
  const lateReply = await replyAgent(
    port,
    { ...auth, work_id: claimed.work_id },
    JSON.stringify({ status: STATUS.created, body: { run_id: "p24-late-after-ttl" } }),
  );
  expect(lateReply.status).toBe(STATUS.conflict);
  expect((JSON.parse(lateReply.body) as { error?: string }).error).toBe("unknown_work");

  // No reply marker retained — a direct GET on the reply key returns nothing.
  const replyKey = `${e.prefix}:reply:${claimed.work_id}`;
  const replyExists = Number.parseInt(await redisExec(["EXISTS", replyKey]), 10);
  expect(replyExists).toBe(0);
});
