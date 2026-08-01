/**
 * Redis test-isolation helper (plan: connect-cli-to-vercel-harness → Startup
 * scaffolding / E2E wiring).
 *
 * The Redis-backed relay shares ONE Redis (brought up by global-setup via
 * docker-compose) across sequential tests. Per-test isolation is by a UNIQUE
 * key prefix: EVERY relay key (session index, epoch counter, work records,
 * capacity counters, replies) is namespaced under `<prefix>:`, and scoped
 * teardown deletes only that prefix. The plan (Codex r1 #15, r2 #10) mandates
 * prefix-every-key over logical-DB allocation: the logical-DB range is small
 * and fixed and does not isolate Lua counters unless every key is prefixed.
 *
 * No Redis client dependency is required for this helper: a unique prefix is
 * minted with crypto.randomUUID, and scoped cleanup shells out to the running
 * compose container's `redis-cli` (best-effort hygiene — the prefix uniqueness
 * already guarantees logical isolation across tests; this only reclaims disk).
 */

import { randomUUID } from "node:crypto";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import path from "node:path";

import { suiteContext } from "./suite-root";

const execFileAsync = promisify(execFile);

/**
 * Redis URL for the e2e suite. The compose file maps 6379:6379 and global-setup
 * brings the container up before any test runs, so the default reaches the
 * container from the host `next start` processes. Allow an override for CI.
 */
export const REDIS_URL = process.env.REDIS_URL ?? "redis://127.0.0.1:6379";

/**
 * A unique namespace for one test's relay keys. Shared by that test's A and B
 * `next start` instances (p17/p20/p21) so they rendezvous, and never reused.
 */
export function uniquePrefix(slug: string): string {
  return `e2e:${slug}:${randomUUID()}`;
}

function composeFile(): string {
  return path.join(suiteContext().repoRoot, "docker-compose.yml");
}

/**
 * Runs a single `redis-cli` command inside the compose container and returns
 * its trimmed stdout. Used by tests that need a deterministic Redis-side signal
 * (e.g. p24 polls `EXISTS <work-key>` to prove the orphan TTL fired before
 * asserting the late reply is rejected). Throws on non-zero exit so callers
 * can retry via `pollUntil`.
 */
export async function redisExec(args: string[]): Promise<string> {
  const file = composeFile();
  const { stdout } = await execFileAsync(
    "docker",
    ["compose", "-f", file, "exec", "-T", "redis", "redis-cli", ...args],
    { timeout: 5_000, maxBuffer: 1024 },
  );
  return stdout.trim();
}

/**
 * Polls Redis until the session's work queue holds ≥1 queued work item (or
 * `timeoutMs` elapses). A deterministic enqueue barrier for tests that need to
 * prove work was durably queued before some destructive action (p18 restarts
 * the server before any agent poll — a fixed `sleep(1000)` was the prior
 * flake-prone barrier, Codex r1 #6). Returns the queue length observed.
 */
export async function waitForQueuedWork(
  prefix: string,
  sessionId: string,
  timeoutMs = 5_000,
): Promise<number> {
  const file = composeFile();
  const key = `${prefix}:workq:${sessionId}`;
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    try {
      const { stdout } = await execFileAsync(
        "docker",
        ["compose", "-f", file, "exec", "-T", "redis", "redis-cli", "LLEN", key],
        { timeout: 5_000, maxBuffer: 1024 },
      );
      const len = Number.parseInt(stdout.trim() || "0", 10);
      if (Number.isFinite(len) && len > 0) {
        return len;
      }
    } catch (error) {
      // Redis exec transient failure — retry until the deadline.
      console.warn(`[redis] waitForQueuedWork LLEN ${key} failed: ${String(error)}`);
    }
    await new Promise((resolve) => setTimeout(resolve, 50));
  }
  throw new Error(
    `waitForQueuedWork: no queued work for ${key} within ${timeoutMs}ms — enqueue did not reach Redis`,
  );
}

/**
 * Best-effort scoped cleanup: deletes every key under this test's prefix.
 * Never throws — prefix uniqueness already provides logical isolation; this
 * only reclaims disk/memory. Uses the compose container's redis-cli so no new
 * dependency is introduced.
 */
export async function flushPrefix(prefix: string): Promise<void> {
  const file = composeFile();
  const pattern = `${prefix}:*`;
  try {
    // SCAN + DEL atomically inside Redis via EVAL (KEYS is fine here — a single
    // test's prefix holds a tiny number of keys).
    const script =
      "local ks=redis.call('KEYS',ARGV[1]); if #ks>0 then redis.call('DEL',unpack(ks)) end; return #ks";
    await execFileAsync(
      "docker",
      ["compose", "-f", file, "exec", "-T", "redis", "redis-cli", "EVAL", script, "0", pattern],
      { timeout: 30_000, maxBuffer: 8 * 1024 * 1024 },
    );
  } catch (error) {
    // Best-effort: a failed flush must never mask the test verdict. Log only.
    console.warn(`[redis] flushPrefix(${pattern}) failed (best-effort): ${String(error)}`);
  }
}

/**
 * Brings the Redis container up and waits for its healthcheck (never a fixed
 * sleep — plan r1 #15). Called once from global-setup. Throws on failure: a
 * Redis that won't start is an environment failure (not a valid RED).
 */
export async function ensureRedisUp(timeoutMs = 60_000): Promise<void> {
  const file = composeFile();
  console.log(`[harness-e2e] docker compose up -d (redis) — ${file}`);
  await execFileAsync("docker", ["compose", "-f", file, "up", "-d"], {
    timeout: 120_000,
    maxBuffer: 8 * 1024 * 1024,
  });

  const deadline = Date.now() + timeoutMs;
  let last = "";
  while (Date.now() < deadline) {
    try {
      const { stdout } = await execFileAsync(
        "docker",
        ["compose", "-f", file, "exec", "-T", "redis", "redis-cli", "ping"],
        { timeout: 5_000, maxBuffer: 1024 },
      );
      if (stdout.trim() === "PONG") {
        console.log("[harness-e2e] redis is ready (PONG)");
        return;
      }
      last = stdout.trim();
    } catch (error) {
      last = String(error);
    }
    await new Promise((resolve) => setTimeout(resolve, 500));
  }

  throw new Error(
    `redis did not become ready within ${timeoutMs}ms — last: ${last}\n` +
      "the Redis-backed e2e suite requires docker compose; bring it up manually to diagnose.",
  );
}

/** Stops the Redis compose stack. Called once from global-teardown (best-effort). */
export async function takeRedisDown(): Promise<void> {
  const file = composeFile();
  try {
    await execFileAsync("docker", ["compose", "-f", file, "down"], {
      timeout: 60_000,
      maxBuffer: 8 * 1024 * 1024,
    });
    console.log("[harness-e2e] docker compose down (redis) — done");
  } catch (error) {
    console.warn(`[harness-e2e] docker compose down failed (best-effort): ${String(error)}`);
  }
}
