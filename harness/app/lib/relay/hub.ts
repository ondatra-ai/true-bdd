/**
 * Process-wide access to the Redis-backed relay. The relay itself
 * (`redis-relay.ts`) holds NO coordination state in process memory — every
 * read/write goes to Redis so any `next start` instance can serve any
 * request and a browser request served by one invocation rendezvous with a
 * CLI poll served by another.
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
  type RelayReply,
  type ReplyRejectReason,
  type SessionMeta,
  type WorkType,
  type WriteReceipt,
} from "./redis-relay";

export { DEADLINE_MS, RELAY_CONFIG, type CliReply, type Correlation, type HubRegisterResult, type SessionMeta };

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
      connectTimeout: 2_000,
      commandTimeout: 5_000,
      retryStrategy: (times: number): number | null => {
        if (times > 10) return null;

        return Math.min(times * 200, 1_000);
      },
    });
    redis.on("error", (error: unknown) => {
      const message = error instanceof Error ? error.message : String(error);
      console.error(`[relay] redis connection error: ${message}`);
    });
    const relay = new RedisRelay(redis, keyPrefix());
    redis.once("end", () => {
      if (store[GLOBAL_KEY] === relay) {
        store[GLOBAL_KEY] = undefined;
        void relay.close();
      }
    });
    store[GLOBAL_KEY] = relay;
  }

  return store[GLOBAL_KEY];
}

export type { PollResult, RelayReply, ReplyRejectReason, WorkType, WriteReceipt };
