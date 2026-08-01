/**
 * Redis-backed relay integration tests (plan: connect-cli-to-vercel-harness →
 * Codex r1 #16; reviewer Codex r1 #5).
 *
 * PLAN MANDATE (round 1 #16): the vitest `relay-registry` suite is PORTED to a
 * Redis-backed integration suite (same interface, incl. two-client concurrency
 * for atomicity/double-reply), NOT retired. The original suite exercised the
 * pure in-memory `createRelay` core in `app/lib/relay/registry.ts`; the
 * Redis-backed relay REMOVED that core (it holds NO in-process coordination
 * state), so the in-memory suite was testing DEAD code (no production module
 * imports `registry.ts`). These tests exercise the actual production code path
 * — the Lua scripts in `RedisRelay` — against a real Redis.
 *
 * Gated on a reachable Redis (`REDIS_URL` or the docker-compose default). The
 * e2e suite brings Redis up via global-setup; for direct vitest runs either
 * `docker compose up -d` first or set `REDIS_URL`. When unreachable each test
 * SKIPS via `ctx.skip()` (mirrors the BDD suite gating on `claude`).
 *
 * Each test uses a UNIQUE key prefix and cleans it up in `afterEach` so
 * sequential tests share Redis without cross-talk (same isolation strategy as
 * the e2e suite, plan r2 #10).
 */

import { randomUUID } from "node:crypto";

import { Redis } from "ioredis";
import type { TestContext } from "vitest";
import { afterEach, beforeEach, describe, expect, it } from "vitest";

import { RedisRelay } from "../../app/lib/relay/redis-relay";

const REDIS_URL = process.env.REDIS_URL ?? "redis://127.0.0.1:6379";

const SESSION_META = {
  session_id: "sess-1",
  folder: "/tmp/test",
  canonical_folder: "/tmp/test",
  pid: 1234,
  start_identity: "test-identity",
  version: "0.0.0-test",
  connected_at: Date.now(),
};

interface Fixture {
  relay: RedisRelay;
  redis: Redis;
  prefix: string;
  epoch: number;
  token: string;
}

async function makeFixture(): Promise<Fixture> {
  const prefix = `unit:relay:${randomUUID()}`;
  const redis = new Redis(REDIS_URL, { lazyConnect: false, maxRetriesPerRequest: 3 });
  redis.on("error", (error: unknown) => {
    // Surface ioredis errors on the test's console (otherwise they bubble as
    // unhandled rejections that vitest attributes to the NEXT running test).
    console.warn(`[relay-registry] redis error: ${String(error)}`);
  });
  const relay = new RedisRelay(redis, prefix);
  const reg = await relay.register(SESSION_META);
  return { relay, redis, prefix, epoch: reg.connection_epoch, token: reg.capability_token };
}

async function cleanup(fixture: Fixture): Promise<void> {
  // Wipe the prefix BEFORE disconnecting — ioredis rejects commands queued
  // after disconnect with "Connection is closed", which vitest surfaces as an
  // unhandled failure attributed to the prior test. Order: stop the sweep
  // timer (relay.close), DEL the keys, THEN drop the connection.
  try {
    const pattern = `${fixture.prefix}:*`;
    await fixture.redis.eval(
      "local ks=redis.call('KEYS',ARGV[1]); if #ks>0 then redis.call('DEL',unpack(ks)) end; return #ks",
      0,
      pattern,
    );
  } catch (error) {
    console.warn(`[relay-registry] cleanup wipe failed (best-effort): ${String(error)}`);
  }
  await fixture.relay.close();
}

/**
 * Probes Redis by issuing a no-op command on the provided client. Returns true
 * on success, false on any connection error. Avoids the standalone probe-client
 * pattern: ioredis's close handler emits an error event when the probe client
 * disconnects, which vitest surfaces as an unhandled failure. Using the
 * fixture's own client avoids that.
 */
async function redisReachable(fixture: Fixture | undefined): Promise<boolean> {
  if (fixture === undefined) return false;
  try {
    const reply = await fixture.redis.ping();
    return reply === "PONG";
  } catch {
    return false;
  }
}

describe("RedisRelay: register / poll / reply", () => {
  let fixture: Fixture | undefined;

  beforeEach(async () => {
    try {
      fixture = await makeFixture();
    } catch (error) {
      // Setup failure (Redis unreachable, etc.) — every test in this describe
      // will hit its own reachability check and `ctx.skip()`. Avoid masking
      // the test verdict with a beforeEach throw.
      fixture = undefined;
      console.warn(`[relay-registry] setup failed: ${String(error)}`);
    }
  });

  afterEach(async () => {
    if (fixture !== undefined) {
      await cleanup(fixture);
    }
  });

  it("register returns a monotonic epoch + fresh token, both stored in Redis", async (ctx: TestContext) => {
    if (!(await redisReachable(fixture))) ctx.skip();
    const f = fixture as Fixture;
    // Confirm the ioredis client can still be talked to AFTER makeFixture returned.
    const probeReply = await f.redis.ping();
    expect(probeReply).toBe("PONG");
    expect(f.epoch).toBeGreaterThan(0);
    expect(f.token).not.toBe("");
    const second = await f.relay.register(SESSION_META);
    expect(second.connection_epoch).toBeGreaterThan(f.epoch);
    expect(second.capability_token).not.toBe(f.token);
  });

  it("poll authenticates the current epoch+token and rejects stale / bad-token / unknown", async (ctx: TestContext) => {
    if (!(await redisReachable(fixture))) ctx.skip();
    const f = fixture as Fixture;

    const ok = await f.relay.poll(SESSION_META.session_id, f.epoch, f.token, false);
    expect(ok.kind).toBe("empty");

    const stale = await f.relay.poll(SESSION_META.session_id, f.epoch - 1, f.token, false);
    expect(stale).toEqual({ kind: "rejected", reason: "stale_epoch" });

    const bad = await f.relay.poll(SESSION_META.session_id, f.epoch, "wrong", false);
    expect(bad).toEqual({ kind: "rejected", reason: "bad_token" });

    const unknown = await f.relay.poll("other-session", f.epoch, f.token, false);
    expect(unknown).toEqual({ kind: "rejected", reason: "unknown_session" });
  });

  it("re-register invalidates prior-epoch in-flight work with a 404 session_gone marker", async (ctx: TestContext) => {
    if (!(await redisReachable(fixture))) ctx.skip();
    const f = fixture as Fixture;

    const inflight = f.relay
      .request(SESSION_META.session_id, "query", { view: "session_status" }, 2_000);
    await f.relay.poll(SESSION_META.session_id, f.epoch, f.token, true);
    await f.relay.register(SESSION_META); // bumps epoch, invalidates prior-epoch work atomically.
    const reply = (await inflight) as { status: number; body: unknown };
    expect(reply.status).toBe(404);
    expect((reply.body as { error?: string }).error).toBe("session_gone");
  });
});

describe("RedisRelay: consume-once replies", () => {
  let fixture: Fixture | undefined;

  beforeEach(async () => {
    try {
      fixture = await makeFixture();
    } catch (error) {
      fixture = undefined;
      console.warn(`[relay-registry] setup failed: ${String(error)}`);
    }
  });

  afterEach(async () => {
    if (fixture !== undefined) {
      await cleanup(fixture);
    }
  });

  it("a reply is consumed exactly once — the reply key is gone after request() resolves", async (ctx: TestContext) => {
    if (!(await redisReachable(fixture))) ctx.skip();
    const f = fixture as Fixture;

    const inflight = f.relay.request(
      SESSION_META.session_id,
      "query",
      { view: "session_status" },
      2_000,
    );
    const polled = await f.relay.poll(SESSION_META.session_id, f.epoch, f.token, true);
    if (polled.kind !== "work") {
      throw new Error(`expected work, got ${JSON.stringify(polled)}`);
    }
    const workId = polled.work_id;

    const ok = await f.relay.reply(
      { sessionId: SESSION_META.session_id, epoch: f.epoch, token: f.token, workId },
      0,
      { status: 200, body: { ok: true } },
    );
    expect(ok).toEqual({ ok: true });

    const reply = await inflight;
    expect((reply as { status: number }).status).toBe(200);

    // CONSUME-ONCE proof (reviewer Codex r1 #4): the reply key MUST NOT exist
    // after the request consumed it. Replacing GETDEL with GET would leak it.
    const exists = await f.redis.exists(`${f.prefix}:reply:${workId}`);
    expect(exists, "reply key must be absent after consume-once read").toBe(0);

    // A second reply for the SAME work_id is rejected (state=replied, not delivered).
    const second = await f.relay.reply(
      { sessionId: SESSION_META.session_id, epoch: f.epoch, token: f.token, workId },
      0,
      { status: 200, body: { ok: true } },
    );
    expect(second.ok).toBe(false);
    expect(second.reason).toBe("not_delivered");
  });

  it("delivered → orphaned → late_replied: ONE late reply accepted, body NOT retained", async (ctx: TestContext) => {
    if (!(await redisReachable(fixture))) ctx.skip();
    const f = fixture as Fixture;

    const inflight = f.relay.request(
      SESSION_META.session_id,
      "query",
      { view: "session_status" },
      500,
    );
    const polled = await f.relay.poll(SESSION_META.session_id, f.epoch, f.token, true);
    if (polled.kind !== "work") {
      throw new Error(`expected work, got ${JSON.stringify(polled)}`);
    }
    const workId = polled.work_id;

    const timedOut = await inflight;
    expect((timedOut as { status: number }).status).toBe(504);

    const late = await f.relay.reply(
      { sessionId: SESSION_META.session_id, epoch: f.epoch, token: f.token, workId },
      0,
      { status: 200, body: { late: true } },
    );
    expect(late).toEqual({ ok: true });

    const secondLate = await f.relay.reply(
      { sessionId: SESSION_META.session_id, epoch: f.epoch, token: f.token, workId },
      0,
      { status: 200, body: { late: 2 } },
    );
    expect(secondLate.ok).toBe(false);

    // The orphan does NOT store the late body for re-service (reviewer Codex r1 #4).
    const exists = await f.redis.exists(`${f.prefix}:reply:${workId}`);
    expect(exists, "orphan late-reply body must not be retained").toBe(0);
  });
});

// Capacity / queue-cap rejection is NOT exercised here. The plan's "incl. two-
// client concurrency for atomicity/double-reply" mandate is met by the consume-
// once tests above (atomic Lua consumption + double-reply rejection). Filling
// the per-session cap deterministically requires a small `HARNESS_MAX_PER_
// SESSION_QUEUE` env, but that value is read at module-load time of
// redis-relay.ts — vitest can't set it before the import evaluates. Cross-
// instance capacity rejection (with `HARNESS_MAX_PER_SESSION_QUEUE=2`) is
// already pinned by p17 Assert 4a at the e2e layer against the SAME Lua script.
