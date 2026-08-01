/**
 * Process-wide access to the Redis-backed relay (plan: connect-cli-to-vercel-
 * harness). The relay itself (`redis-relay.ts`) holds NO coordination state
 * in process memory — every read/write goes to Redis so any `next start`
 * instance can serve any request and a browser request served by one
 * invocation rendezvous with a CLI poll served by another.
 *
 * `relayHub()` returns ONE `RedisRelay` per process (cached on globalThis so
 * Next.js route-module re-eval reuses the same Redis connection). The
 * `globalThis` cache holds ONLY the Redis connection — no coordination state.
 */

import { Redis } from "ioredis";

import {
  DEADLINE_MS,
  RELAY_CONFIG,
  RedisRelay,
  type CliReply,
  type Correlation,
  type HubRegisterResult,
  type PollResult,
  type ReplyRejectReason,
  type SessionMeta,
  type WorkType,
} from "./redis-relay";

export {
  DEADLINE_MS,
  RELAY_CONFIG,
  type CliReply,
  type Correlation,
  type HubRegisterResult,
  type SessionMeta,
};

const GLOBAL_KEY = "__trueBddRelayHub__";
const DEFAULT_REDIS_URL = "redis://127.0.0.1:6379";
const DEFAULT_KEY_PREFIX = "tb";

interface GlobalWithHub {
  [GLOBAL_KEY]?: RedisRelay;
}

function keyPrefix(): string {
  const raw = process.env.REDIS_KEY_PREFIX;
  if (raw === undefined || raw.length === 0) return DEFAULT_KEY_PREFIX;
  return raw;
}

function redisUrl(): string {
  const raw = process.env.REDIS_URL;
  if (raw === undefined || raw.length === 0) return DEFAULT_REDIS_URL;
  return raw;
}

/**
 * Returns the process-wide Redis-backed relay, creating it on first access.
 * The Redis connection is lazy-established and reused across warm invocations.
 */
export function relayHub(): RedisRelay {
  const store = globalThis as unknown as GlobalWithHub;
  if (store[GLOBAL_KEY] === undefined) {
    const redis = new Redis(redisUrl(), {
      lazyConnect: false,
      maxRetriesPerRequest: 3,
      enableReadyCheck: true,
      // A serverless function MUST fail fast when Redis is unreachable — a
      // hung request serves no one. The default retry ceiling keeps a single
      // warm boot's reconnect bounded.
      connectTimeout: 2_000,
      commandTimeout: 5_000,
      // Bounded reconnect strategy (Codex r1 #10): `maxRetriesPerRequest`
      // caps retries for an IN-FLIGHT command but does not stop the cached
      // client's background reconnect loop. In a warm serverless invocation a
      // perpetually-reconnecting client would burn CPU and emit noise, so cap
      // the reconnect backoff at 1s and reconnect attempts at 10 (after which
      // ioredis stops retrying the connection itself — the next request will
      // surface a real error rather than silently queuing forever).
      retryStrategy: (times: number): number | null => {
        if (times > 10) return null;
        return Math.min(times * 200, 1_000);
      },
    });
    // Surface connection errors so they never become unhandled rejections.
    redis.on("error", (error: unknown) => {
      const message = error instanceof Error ? error.message : String(error);
      console.error(`[relay] redis connection error: ${message}`);
    });
    store[GLOBAL_KEY] = new RedisRelay(redis, keyPrefix());
  }

  return store[GLOBAL_KEY];
}

// Re-export the PollResult / ReplyRejectReason / WorkType types for callers
// that import them from the hub module.
export type { PollResult, ReplyRejectReason, WorkType };
