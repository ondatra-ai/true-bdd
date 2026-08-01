/**
 * Redis-backed relay coordination layer (plan: connect-cli-to-vercel-harness).
 *
 * The relay holds NO run/session database: it is a register / poll / reply
 * message bus whose COORDINATION state (session registry, bounded work queue,
 * consume-once replies) lives in Redis so a browser request served by one
 * `next start` invocation and a CLI poll served by a DIFFERENT invocation
 * rendezvous. Run output / prompts / inventory stay on the host CLI; the reply
 * body transits Redis only as the transient, consume-once coordination message.
 *
 * Key scheme (every key prefixed with `REDIS_KEY_PREFIX` for per-test
 * isolation; the prefix is mandatory and never empty):
 *   - `<p>:sessions`           SET of registered session ids.
 *   - `<p>:session:<sid>`      HASH session meta (epoch, token, lastSeen, …).
 *   - `<p>:epoch`              INCR counter for epoch allocation (persistent).
 *   - `<p>:workq:<sid>`        LIST FIFO of QUEUED work_ids awaiting claim.
 *   - `<p>:work:<wid>`         HASH per-work record (state machine + payload).
 *   - `<p>:inflight:<sid>`     SET in-flight work_ids (queued+delivered) — cap.
 *   - `<p>:inflight:global`    SET in-flight work_ids across all sessions.
 *   - `<p>:reply:<wid>`        STRING opaque reply JSON; consume-once via GETDEL.
 *
 * Per-work state machine (plan r3 #3):
 *   queued → delivered → replied                 (happy path)
 *   queued → cancelled                           (browser abort / 504 on queued)
 *   delivered → orphaned → late_replied          (browser 504 then ≤1 late reply)
 *   any in-flight → expired                      (session swept / re-registered)
 *
 * Atomicity: every transition is ONE Lua script (no app-level read-then-write).
 * `timeout` and `reply` are COMPETING Lua transitions on the same record —
 * exactly one wins. Replies are consumed atomically via GETDEL inside the
 * resolve script (never GET then DELETE).
 *
 * Serverless fit: the browser `request()` wait is bounded by the operation
 * deadline (5 s/10 s/30 s, unchanged on local `next start`); the Redis poll
 * loop observes a new reply within ~50 ms. Route-level `maxDuration` (Vercel)
 * clamps the wait under the function limit; on local the existing deadlines
 * stay (no silent lowering — Codex r3 #1).
 */

import { Redis } from "ioredis";

/** Operation deadlines the browser routes bound each work item by. */
export const DEADLINE_MS = {
  status: 5_000,
  runDetail: 5_000,
  mutation: 10_000,
  inventory: 30_000,
} as const;

/** Default relay tuning; per-session queue cap is overridable via env. */
export const RELAY_CONFIG = {
  expiryMs: 10_000,
  perSessionQueueMax: perSessionQueueMaxFromEnv(),
  globalQueueMax: globalQueueMaxFromEnv(),
  replyBudgetBytes: 1_048_576,
} as const;

/** Absence-of-poll session expiry (ms). An open poll counts as liveness. */
const EXPIRY_MS = RELAY_CONFIG.expiryMs;

/**
 * Per-request server-side wait ceiling. On local `next start`
 * (`HARNESS_DEPLOYED` is NOT `"1"`) it is +Infinity so the existing
 * 5s/10s/30s operation deadlines are preserved byte-for-byte (plan r3 #1: do
 * NOT lower inventory 30s globally without evidence — existing tests assert
 * 30s). On Vercel (`HARNESS_DEPLOYED === "1"`) the browser wait is clamped
 * under the Go client's 15s ceiling minus network slack so it fits a
 * serverless function (plan r1 #14 / r3 #1); override with HARNESS_MAX_WAIT_MS.
 */
function deployedWaitCeilingMs(): number {
  if (process.env.HARNESS_DEPLOYED !== "1") {
    return Number.POSITIVE_INFINITY;
  }
  const raw = process.env.HARNESS_MAX_WAIT_MS;
  const parsed = raw === undefined ? Number.NaN : Number(raw);

  return Number.isFinite(parsed) && parsed > 0 ? Math.floor(parsed) : 14_000;
}

/** TTLs (seconds) — backstops; per-test isolation is by key prefix. */
const SESSION_TTL_SEC = 60 * 60; // 1h: a connected CLI keeps it alive via polls.
const WORK_TTL_SEC = 60 * 60; // 1h: covers the longest inventory deadline (30s).
const REPLY_TTL_SEC = replyTtlSecFromEnv(); // consume-once transit; GETDEL is the real consumer.
const ORPHAN_TTL_SEC = orphanTtlSecFromEnv(); // reclaim capacity if no late reply arrives.

function replyTtlSecFromEnv(): number {
  const raw = process.env.HARNESS_REPLY_TTL_SEC;
  if (raw === undefined) return 60;
  const parsed = Number(raw);
  return Number.isFinite(parsed) && parsed > 0 ? Math.floor(parsed) : 60;
}

/**
 * ORPHAN_TTL_SEC is configurable so a test can advance past the orphan window
 * and prove a late reply is rejected with `unknown_work` after the work hash
 * expires (Codex r1 #9). Default 30s.
 */
function orphanTtlSecFromEnv(): number {
  const raw = process.env.HARNESS_ORPHAN_TTL_SEC;
  if (raw === undefined) return 30;
  const parsed = Number(raw);
  return Number.isFinite(parsed) && parsed > 0 ? Math.floor(parsed) : 30;
}

const sleep = (ms: number): Promise<void> => new Promise((resolve) => setTimeout(resolve, ms));

function perSessionQueueMaxFromEnv(): number {
  const raw = process.env.HARNESS_MAX_PER_SESSION_QUEUE;
  if (raw === undefined) return 64;
  const parsed = Number(raw);
  return Number.isFinite(parsed) && parsed > 0 ? Math.floor(parsed) : 64;
}

/**
 * Global in-flight cap. Configurable so a test can shrink it and prove that a
 * swept session's abandoned work does not permanently leak into the global
 * inflight counter (which would eventually exhaust capacity with ghost ids —
 * the inflight SETs carry no TTL; only sweep/resolve/timeout clean them).
 */
function globalQueueMaxFromEnv(): number {
  const raw = process.env.HARNESS_MAX_GLOBAL_QUEUE;
  if (raw === undefined) return 256;
  const parsed = Number(raw);
  return Number.isFinite(parsed) && parsed > 0 ? Math.floor(parsed) : 256;
}

/** Cryptographically-random opaque id (capability token / work id). */
function randomToken(): string {
  return globalThis.crypto.randomUUID();
}

// ── Public types (preserved shape; route handlers stay thin) ──

export interface SessionMeta {
  session_id: string;
  folder: string;
  canonical_folder: string;
  pid: number;
  start_identity: string;
  version: string;
  connected_at: number;
}

export interface CliReply {
  status: number;
  body: unknown;
}

export interface Correlation {
  sessionId: string;
  epoch: number;
  token: string;
  workId: string;
}

export interface HubRegisterResult {
  connection_epoch: number;
  reply_budget_bytes: number;
  capability_token: string;
}

export type WorkType = "query" | "dispatch" | "answer";

export type PollResult =
  | { kind: "work"; work_id: string; type: WorkType; payload: unknown }
  | { kind: "empty" }
  | { kind: "rejected"; reason: "unknown_session" | "stale_epoch" | "bad_token" };

export type ReplyRejectReason =
  | "stale_epoch"
  | "bad_token"
  | "unknown_work"
  | "not_delivered"
  | "too_large"
  | "unknown_session";

// ── Lua scripts (one per atomic transition). reply_prefix is ARGV-supplied so
//    scripts can address <p>:reply:<wid> without reconstructing the prefix. ──

// Register: bump persistent epoch, rotate token, expire prior-epoch in-flight
// work (writing a correlated 404 marker each waiting request will consume),
// then write the new session hash + index entry. The epoch is allocated INSIDE
// this script (INCR on epoch_key) so two concurrent re-registrations cannot
// interleave INCR + write and leave an OLDER epoch installed after a newer one
// (plan r1 #5: register is ONE atomic transition — epoch bump, token rotate,
// prior-work invalidation, session write all indivisible).
// KEYS = [ sessions_set, session_hash, inflight_sid, inflight_global, epoch_key ]
// ARGV = [ sid, token, now, folder, canonical_folder, pid,
//          start_identity, version, reply_ttl_sec, session_ttl_sec,
//          work_prefix, reply_prefix ]
const REGISTER_SCRIPT = `
local sset, shash, inflight, ginflight, epoch_key = unpack(KEYS)
local sid, token, now = ARGV[1], ARGV[2], ARGV[3]
local folder, canon, pid, ident, ver = ARGV[4], ARGV[5], ARGV[6], ARGV[7], ARGV[8]
local reply_ttl, session_ttl = tonumber(ARGV[9]), tonumber(ARGV[10])
local work_prefix, reply_prefix = ARGV[11], ARGV[12]

local epoch = redis.call('INCR', epoch_key)

local expired = redis.call('SMEMBERS', inflight)
for _, wid in ipairs(expired) do
  redis.call('HSET', work_prefix .. wid, 'state', 'expired')
  redis.call('SREM', inflight, wid)
  redis.call('SREM', ginflight, wid)
  redis.call('SET', reply_prefix .. wid, '{"status":404,"body":{"error":"session_gone"}}', 'EX', reply_ttl)
end

redis.call('HSET', shash,
  'session_id', sid, 'folder', folder, 'canonical_folder', canon,
  'pid', pid, 'start_identity', ident, 'version', ver,
  'epoch', tostring(epoch), 'token', token,
  'lastSeen', now, 'connected_at', now)
redis.call('EXPIRE', shash, session_ttl)
redis.call('SADD', sset, sid)
return epoch
`;

// Enqueue: capacity check (queued+delivered counts) then append a queued work
// record. The work record embeds the session's CURRENT epoch+token so a later
// reply can authenticate from the work record ALONE (it lives for WORK_TTL,
// well past any session sweep that may remove the session hash — Codex r3 #3:
// the late-reply path must not depend on the session hash surviving the
// request deadline window).
// KEYS = [ inflight_sid, inflight_global, workq_sid, work_hash, session_hash ]
// ARGV = [ sid, max_session, max_global, work_id, type, payload, queued_at,
//          work_ttl_sec ]
const ENQUEUE_SCRIPT = `
local inflight, ginflight, workq, whash, shash = unpack(KEYS)
local sid = ARGV[1]
local max_session = tonumber(ARGV[2])
local max_global = tonumber(ARGV[3])
local wid = ARGV[4]
local wtype = ARGV[5]
local payload = ARGV[6]
local queued_at = ARGV[7]
local work_ttl = tonumber(ARGV[8])

-- Validate the session FIRST (Codex r2 #2): a vanished session whose stale
-- inflight set still saturates the cap must classify as unknown_session (→ 404
-- session_gone), NOT as a capacity rejection. Capacity is only meaningful for a
-- live session.
local epoch = redis.call('HGET', shash, 'epoch')
local token = redis.call('HGET', shash, 'token')
if epoch == false or token == false then
  return {0, 'unknown_session'}
end

if redis.call('SCARD', inflight) >= max_session then
  return {0, 'queue_full_session'}
end
if redis.call('SCARD', ginflight) >= max_global then
  return {0, 'queue_full_global'}
end

redis.call('SADD', inflight, wid)
redis.call('SADD', ginflight, wid)
redis.call('RPUSH', workq, wid)
redis.call('HSET', whash,
  'work_id', wid, 'session_id', sid,
  'epoch', epoch, 'token', token,
  'type', wtype, 'payload', payload,
  'state', 'queued', 'queued_at', queued_at)
redis.call('EXPIRE', whash, work_ttl)
redis.call('EXPIRE', workq, work_ttl)
return {1, wid}
`;

// Poll claim: authenticate session+epoch+token, renew lastSeen, move the
// OLDEST queued item to delivered. Returns {err, reason} | {empty} | {work,...}.
// KEYS = [ session_hash, workq_sid ]
// ARGV = [ sid, epoch_s, token, now, session_ttl_sec, work_prefix ]
const POLL_CLAIM_SCRIPT = `
local shash, workq = unpack(KEYS)
local sid, epoch_s, token, now = ARGV[1], ARGV[2], ARGV[3], ARGV[4]
local session_ttl = tonumber(ARGV[5])
local work_prefix = ARGV[6]

local cur_epoch = redis.call('HGET', shash, 'epoch')
if cur_epoch == false then
  return {'err', 'unknown_session'}
end
if cur_epoch ~= epoch_s then
  return {'err', 'stale_epoch'}
end
local cur_token = redis.call('HGET', shash, 'token')
if cur_token ~= token then
  return {'err', 'bad_token'}
end

redis.call('HSET', shash, 'lastSeen', now)
redis.call('EXPIRE', shash, session_ttl)

-- Peel stale head ids until a QUEUED item is found or the queue empties. A
-- re-register (REGISTER_SCRIPT) invalidates prior-epoch work by HSET state=
-- expired but does not LREM the workq; a work hash may also expire (WORK_TTL)
-- while still listed. Either leaves a non-queued id at the head that would
-- otherwise shadow freshly-queued work — a wait:false poll must not return
-- 204 (empty) while valid work sits behind a stale id. Discarding here (one
-- atomic script) handles every stale-id source.
while true do
  local wid = redis.call('LPOP', workq)
  if wid == false then
    return {'empty'}
  end
  local wkey = work_prefix .. wid
  if redis.call('HGET', wkey, 'state') == 'queued' then
    redis.call('HSET', wkey, 'state', 'delivered', 'delivered_at', now)
    local wtype = redis.call('HGET', wkey, 'type')
    local payload = redis.call('HGET', wkey, 'payload')
    return {'work', wid, wtype, payload}
  end
end
`;

// Resolve: authenticate correlation, then write the consume-once reply
// marker and retire the record. The marker is consumed via GETDEL by exactly
// one browser request. A late reply for an ORPHANED record is accepted ≤1
// time and the body is NOT retained.
//
// Auth order (Codex r3 #3 — the late-reply path must not depend on the
// session hash surviving the request deadline):
//   - If the SESSION hash exists, enforce its current epoch+token FIRST so a
//     bogus work_id with a wrong token still classifies as `bad_token`
//     (p17 Assert 4b), not `unknown_work`.
//   - Else (the session was swept/expired while work was in flight), fall
//     back to the epoch+token EMBEDDED in the work record at enqueue time —
//     the late-reply recovery path (p22 Assert 1).
// KEYS = [ session_hash, work_hash, reply_key, inflight_sid, inflight_global ]
// ARGV = [ sid, epoch_s, token, work_id, reply_body, reply_ttl_sec ]
const RESOLVE_SCRIPT = `
local shash, whash, reply_key, inflight, ginflight = unpack(KEYS)
local sid, epoch_s, token, wid = ARGV[1], ARGV[2], ARGV[3], ARGV[4]
local reply_body = ARGV[5]
local reply_ttl = tonumber(ARGV[6])

local s_epoch = redis.call('HGET', shash, 'epoch')
if s_epoch ~= false then
  if s_epoch ~= epoch_s then
    return {'err', 'stale_epoch'}
  end
  local s_token = redis.call('HGET', shash, 'token')
  if s_token ~= token then
    return {'err', 'bad_token'}
  end
end

local w_sid = redis.call('HGET', whash, 'session_id')
if w_sid == false or w_sid ~= sid then
  return {'err', 'unknown_work'}
end

-- Session gone (swept while work was in flight): authenticate from the work
-- record's embedded epoch+token — the late-reply recovery path (p22).
if s_epoch == false then
  local w_epoch = redis.call('HGET', whash, 'epoch')
  if w_epoch ~= epoch_s then
    return {'err', 'stale_epoch'}
  end
  local w_token = redis.call('HGET', whash, 'token')
  if w_token ~= token then
    return {'err', 'bad_token'}
  end
end

local wstate = redis.call('HGET', whash, 'state')
if wstate == 'delivered' then
  redis.call('HSET', whash, 'state', 'replied')
  redis.call('SREM', inflight, wid)
  redis.call('SREM', ginflight, wid)
  redis.call('SET', reply_key, reply_body, 'EX', reply_ttl)
  return {'ok'}
end
if wstate == 'orphaned' then
  redis.call('HSET', whash, 'state', 'late_replied')
  return {'ok'}
end
return {'err', 'not_delivered'}
`;

// Timeout: the browser request's wait elapsed. QUEUED → cancelled (dropped,
// never delivered late); DELIVERED → orphaned (CLI may still commit; ≤1 late
// reply then retires without storing the body). Timeout and reply are
// COMPETING transitions — exactly one wins. Returns the prior state.
// On the delivered→orphaned branch the owning session's lastSeen is renewed
// so a late CLI reply (which authenticates against the session hash) still
// finds the session alive for the orphan window — the alternative is to
// authenticate stale work without a session, which would defeat epoch/token
// rotation. KEYS = [ work_hash, workq_sid, inflight_sid, inflight_global, session_hash ]
// ARGV = [ work_id, now, orphan_ttl_sec, session_ttl_sec ]
const TIMEOUT_SCRIPT = `
local whash, workq, inflight, ginflight, shash = unpack(KEYS)
local wid = ARGV[1]
local now = ARGV[2]
local orphan_ttl = tonumber(ARGV[3])
local session_ttl = tonumber(ARGV[4])

local state = redis.call('HGET', whash, 'state')
if state == 'queued' then
  redis.call('HSET', whash, 'state', 'cancelled')
  redis.call('SREM', inflight, wid)
  redis.call('SREM', ginflight, wid)
  redis.call('LREM', workq, 0, wid)
  return 'cancelled'
end
if state == 'delivered' then
  redis.call('HSET', whash, 'state', 'orphaned', 'orphaned_at', now)
  redis.call('SREM', inflight, wid)
  redis.call('SREM', ginflight, wid)
  redis.call('EXPIRE', whash, orphan_ttl)
  -- Renew the session lease: the CLI may still commit and POST a late reply
  -- within the orphan window, and that reply authenticates against the
  -- session hash — so the session must survive past the request deadline.
  if redis.call('EXISTS', shash) == 1 then
    redis.call('HSET', shash, 'lastSeen', now)
    redis.call('EXPIRE', shash, session_ttl)
  end
  return 'orphaned'
end
return state or 'missing'
`;

// Cancel (browser abort): same as TIMEOUT for queued work (→ cancelled). For
// DELIVERED work the CLI may still be executing and commit a late reply, so —
// like TIMEOUT — we transition delivered→orphaned: free the inflight capacity
// now, set the orphan TTL, and renew the session lease so a late reply still
// authenticates. Without this branch an aborted delivered item would sit in the
// inflight sets (which carry no TTL) until the session disconnects and is
// swept, leaking capacity on a long-lived session (reviewer Codex r1 #1).
// KEYS = [ work_hash, workq_sid, inflight_sid, inflight_global, session_hash ]
// ARGV = [ work_id, now, orphan_ttl_sec, session_ttl_sec ]
const CANCEL_SCRIPT = `
local whash, workq, inflight, ginflight, shash = unpack(KEYS)
local wid = ARGV[1]
local now = ARGV[2]
local orphan_ttl = tonumber(ARGV[3])
local session_ttl = tonumber(ARGV[4])
local state = redis.call('HGET', whash, 'state')
if state == 'queued' then
  redis.call('HSET', whash, 'state', 'cancelled')
  redis.call('SREM', inflight, wid)
  redis.call('SREM', ginflight, wid)
  redis.call('LREM', workq, 0, wid)
  return 'cancelled'
end
if state == 'delivered' then
  redis.call('HSET', whash, 'state', 'orphaned', 'orphaned_at', now)
  redis.call('SREM', inflight, wid)
  redis.call('SREM', ginflight, wid)
  redis.call('EXPIRE', whash, orphan_ttl)
  if redis.call('EXISTS', shash) == 1 then
    redis.call('HSET', shash, 'lastSeen', now)
    redis.call('EXPIRE', shash, session_ttl)
  end
  return 'orphaned'
end
return state or 'missing'
`;

// Final read after the request loop's deadline elapsed (reviewer Codex r1 #1):
// GETDEL the reply key once to catch a CLI reply that landed in the ~50ms race
// between the last loop poll and the timeout transition, BUT only CONSUME it
// when the work state is `replied`. The sweep (SWEEP_SCRIPT) writes a 404
// session_gone marker to this same key when it evicts the session; without the
// state gate, that marker would be consumed here and the request would return
// 404, masking the contract-preserving 504 cli_timeout path that p22's
// delivered→timeout→late-commit oracle pins. When the state is anything other
// than `replied`, RESTORE the marker under its TTL and return nil so the
// caller falls through to the timeout transition (which then runs against the
// authoritative work state) and returns 504.
// KEYS = [ reply_key, work_hash ]
// ARGV = [ reply_ttl_sec ]
const FINAL_READ_SCRIPT = `
local reply_key, whash = unpack(KEYS)
local reply_ttl = tonumber(ARGV[1])
local reply = redis.call('GETDEL', reply_key)
if reply == false then
  return nil
end
local state = redis.call('HGET', whash, 'state')
if state == 'replied' then
  return reply
end
-- A marker (sweep) or an unrelated reply left for a non-replied state: restore
-- it so the next loop iteration (a future request's getdel, or the timeout
-- path's housekeeping) still sees it, and signal "no real reply to consume".
redis.call('SET', reply_key, reply, 'EX', reply_ttl)
return nil
`;

// Sweep: remove sessions whose lastSeen is older than expiryMs from the
// REGISTRY (the session hash + the sessions index), AND unblock any browser
// request STILL WAITING on the now-dead session with a consume-once 404
// session_gone marker for its QUEUED/DELIVERED in-flight work. Without that
// marker, a poll request already past its hasSession gate would sit in
// request()'s getdel loop until its OWN (up to 30s) deadline — so an
// already-open page would keep showing the last live value for the full
// inventory window instead of clearing to the disconnected view (P2:
// SIGSTOPped remote).
//
// In-flight work is ALSO retired here so capacity is reclaimed (a swept
// session's inflight-set membership would otherwise leak — the sets carry no
// TTL, and after enough abandoned sessions the global cap would exhaust):
//   - QUEUED → cancelled: a read/mutation for a dead session is never
//     delivered; drop it from the work queue + both inflight sets.
//   - DELIVERED → orphaned: the CLI may still commit late, so the work hash
//     survives `orphan_ttl` to accept ≤1 late reply (authenticated from the
//     work record's embedded epoch+token — p22's late-reply path), but
//     capacity is reclaimed NOW by dropping inflight membership.
// The state-machine values (`cancelled` / `orphaned`) are recognized by
// RESOLVE — an earlier `state=expired` attempt broke p22 with `not_delivered`
// because `expired` is not a reply-accepting state. p22 itself is unaffected:
// its orphan transition originates in TIMEOUT (which renews the session
// lease), so its session never reaches this sweep branch.
// KEYS = [ sessions_set ]
// ARGV = [ now, expiry_ms, session_prefix, inflight_prefix, work_prefix,
//          reply_prefix, workq_prefix, reply_ttl_sec, orphan_ttl_sec ]
const SWEEP_SCRIPT = `
local sset = KEYS[1]
local now = tonumber(ARGV[1])
local expiry_ms = tonumber(ARGV[2])
local session_prefix = ARGV[3]
local inflight_prefix = ARGV[4]
local work_prefix = ARGV[5]
local reply_prefix = ARGV[6]
local workq_prefix = ARGV[7]
local reply_ttl = tonumber(ARGV[8])
local orphan_ttl = tonumber(ARGV[9])
local ginflight = inflight_prefix .. 'global'
local gone = '{"status":404,"body":{"error":"session_gone"}}'

local sids = redis.call('SMEMBERS', sset)
local removed = 0
for _, sid in ipairs(sids) do
  local shash = session_prefix .. sid
  local last = redis.call('HGET', shash, 'lastSeen')
  if last == false or (now - tonumber(last)) > expiry_ms then
    local inflight = inflight_prefix .. sid
    local workq = workq_prefix .. sid
    local wids = redis.call('SMEMBERS', inflight)
    for _, wid in ipairs(wids) do
      local wkey = work_prefix .. wid
      local wstate = redis.call('HGET', wkey, 'state')
      if wstate == 'queued' then
        redis.call('HSET', wkey, 'state', 'cancelled')
        redis.call('LREM', workq, 0, wid)
        redis.call('SREM', inflight, wid)
        redis.call('SREM', ginflight, wid)
        redis.call('SET', reply_prefix .. wid, gone, 'EX', reply_ttl)
      elseif wstate == 'delivered' then
        redis.call('HSET', wkey, 'state', 'orphaned', 'orphaned_at', now)
        redis.call('EXPIRE', wkey, orphan_ttl)
        redis.call('SREM', inflight, wid)
        redis.call('SREM', ginflight, wid)
        redis.call('SET', reply_prefix .. wid, gone, 'EX', reply_ttl)
      else
        -- The work hash already expired (WORK_TTL elapsed before sweep) or is
        -- in a terminal/non-reply state (cancelled/orphaned/replied/etc.). The
        -- inflight sets carry NO TTL, so a ghost id left here would accumulate
        -- across abandoned sessions and eventually exhaust the global cap
        -- (reviewer Codex r2 #2). Branchless SREM on both sets is safe — a
        -- non-member SREM is a no-op; for already-terminal states the prior
        -- transition should have SREM'd, but this guards the Work-hash-TTL
        -- expiry path that those transitions do not cover.
        redis.call('SREM', inflight, wid)
        redis.call('SREM', ginflight, wid)
      end
    end
    redis.call('DEL', shash)
    redis.call('SREM', sset, sid)
    removed = removed + 1
  end
end
return tostring(removed)
`;

function parseSessionMeta(sid: string, m: Record<string, string>): SessionMeta {
  return {
    session_id: sid,
    folder: m.folder ?? "",
    canonical_folder: m.canonical_folder ?? "",
    pid: Number(m.pid ?? 0),
    start_identity: m.start_identity ?? "",
    version: m.version ?? "",
    connected_at: Number(m.connected_at ?? 0),
  };
}

/**
 * The Redis-backed relay. Holds NO coordination state in process memory —
 * every read/write goes to Redis so any `next start` instance can serve any
 * request. One instance per process (cached on globalThis by `relayHub()`).
 */
export class RedisRelay {
  private readonly redis: Redis;
  private readonly prefix: string;
  private readonly workPrefix: string; // `<prefix>:work:`
  private readonly sessionPrefix: string; // `<prefix>:session:`
  private readonly replyPrefix: string; // `<prefix>:reply:`
  private readonly sweepTimer: ReturnType<typeof setInterval>;

  constructor(redis: Redis, prefix: string) {
    this.redis = redis;
    this.prefix = prefix;
    this.sessionPrefix = `${prefix}:session:`;
    this.workPrefix = `${prefix}:work:`;
    this.replyPrefix = `${prefix}:reply:`;
    // Background sweep (best-effort; unref so it never holds the process open).
    this.sweepTimer = setInterval(() => {
      this.sweep().catch(() => {});
    }, 2_000);
    if (typeof this.sweepTimer.unref === "function") {
      this.sweepTimer.unref();
    }
  }

  /** Closes the underlying Redis connection (test-only). */
  async close(): Promise<void> {
    clearInterval(this.sweepTimer);
    try {
      this.redis.disconnect();
    } catch {
      // Best-effort.
    }
  }

  // ── Key helpers ──

  private sessionsKey(): string {
    return `${this.prefix}:sessions`;
  }
  private sessionHash(sid: string): string {
    return `${this.sessionPrefix}${sid}`;
  }
  private workqKey(sid: string): string {
    return `${this.prefix}:workq:${sid}`;
  }
  private workHash(wid: string): string {
    return `${this.workPrefix}${wid}`;
  }
  private inflightKey(sid: string): string {
    return `${this.prefix}:inflight:${sid}`;
  }
  private globalInflightKey(): string {
    return `${this.prefix}:inflight:global`;
  }
  private replyKey(wid: string): string {
    return `${this.replyPrefix}${wid}`;
  }

  // ── Register ──

  async register(meta: SessionMeta): Promise<HubRegisterResult> {
    await this.sweep();
    const token = randomToken();
    const now = Date.now();
    const epoch = (await this.redis.eval(
      REGISTER_SCRIPT,
      5,
      this.sessionsKey(),
      this.sessionHash(meta.session_id),
      this.inflightKey(meta.session_id),
      this.globalInflightKey(),
      `${this.prefix}:epoch`,
      meta.session_id,
      token,
      String(now),
      meta.folder,
      meta.canonical_folder,
      String(meta.pid),
      meta.start_identity,
      meta.version,
      String(REPLY_TTL_SEC),
      String(SESSION_TTL_SEC),
      this.workPrefix,
      this.replyPrefix,
    )) as number;

    return {
      connection_epoch: epoch,
      capability_token: token,
      reply_budget_bytes: RELAY_CONFIG.replyBudgetBytes,
    };
  }

  // ── Registry-only browser surface ──

  async listSessions(): Promise<SessionMeta[]> {
    await this.sweep();
    const sids = await this.redis.smembers(this.sessionsKey());
    const out: SessionMeta[] = [];
    for (const sid of sids) {
      const m = await this.redis.hgetall(this.sessionHash(sid));
      if (m === undefined || m.epoch === undefined) {
        continue;
      }
      out.push(parseSessionMeta(sid, m));
    }
    return out;
  }

  async hasSession(sid: string): Promise<boolean> {
    const fresh = await this.sessionAlive(sid);
    if (fresh) {
      return true;
    }
    // The session's lastSeen was stale at first read. A concurrent agent poll
    // (or any other path that renews lastSeen) may have landed between the read
    // and now; sweeping then re-reading distinguishes "really gone" from "was
    // stale, has been renewed" — a sweep that runs after the renewal preserves
    // the session, and the second read reflects that (Codex r2 #1).
    await this.sweep();
    return this.sessionAlive(sid);
  }

  /** Whether the session hash exists and its lastSeen is within the expiry window. */
  private async sessionAlive(sid: string): Promise<boolean> {
    const m = await this.redis.hgetall(this.sessionHash(sid));
    if (m === undefined || m.epoch === undefined) {
      return false;
    }
    const lastSeen = Number(m.lastSeen ?? 0);
    if (!Number.isFinite(lastSeen) || Date.now() - lastSeen > EXPIRY_MS) {
      return false;
    }
    return true;
  }

  // ── Browser request: enqueue + await the consume-once reply ──

  async request(
    sessionId: string,
    type: WorkType,
    payload: unknown,
    deadlineMs: number,
    signal?: AbortSignal,
  ): Promise<CliReply> {
    if (signal?.aborted) {
      return { status: 499, body: { error: "client_aborted" } };
    }

    const alive = await this.hasSession(sessionId);
    if (!alive) {
      return { status: 404, body: { error: "session_gone" } };
    }

    const workId = randomToken();
    const now = Date.now();
    const enq = (await this.redis.eval(
      ENQUEUE_SCRIPT,
      5,
      this.inflightKey(sessionId),
      this.globalInflightKey(),
      this.workqKey(sessionId),
      this.workHash(workId),
      this.sessionHash(sessionId),
      sessionId,
      String(RELAY_CONFIG.perSessionQueueMax),
      String(RELAY_CONFIG.globalQueueMax),
      workId,
      type,
      JSON.stringify(payload),
      String(now),
      String(WORK_TTL_SEC),
    )) as [number, string];

    if (enq[0] === 0) {
      // Distinguish "session gone" (swept/re-registered between hasSession and
      // enqueue — Codex r1 #2) from genuine capacity exhaustion: only the
      // latter is 503.
      if (enq[1] === "unknown_session") {
        return { status: 404, body: { error: "session_gone" } };
      }
      return { status: 503, body: { error: "capacity" } };
    }

    const replyKey = this.replyKey(workId);
    const deadline = Date.now() + Math.min(deadlineMs, deployedWaitCeilingMs());
    while (Date.now() < deadline) {
      if (signal?.aborted) {
        await this.cancel(sessionId, workId);
        return { status: 499, body: { error: "client_aborted" } };
      }
      const replyJson = await this.redis.getdel(replyKey);
      if (replyJson !== null) {
        try {
          return JSON.parse(replyJson) as CliReply;
        } catch {
          return { status: 502, body: { error: "invalid_cli_reply" } };
        }
      }
      await sleep(50);
    }

    // Deadline elapsed. ONE FINAL consume-once read closes the race between the
    // last loop getdel (returned null) and the timeout transition: a CLI reply
    // that landed in that ~50 ms window must NOT be orphaned by a spurious 504
    // (the reply body would otherwise sit in Redis until its TTL — a wasted
    // transactional ack. Codex review r1 #1).
    //
    // The read is gated on the work state via Lua (FINAL_READ_SCRIPT): the
    // sweep writes a `{"status":404,...}` marker to this same reply key when it
    // evicts the session (p2 regression fix). When the dispatch deadline (10s)
    // and the session expiry (10s) coincide, that marker lands in the same
    // post-loop window as a real late reply would. WITHOUT the state gate the
    // marker is consumed and the request returns 404 — masking the contract-
    // preserving 504 cli_timeout path (p22's "delivered→timeout→late-commit"
    // assertion, which proves the orphan state accepts ≤1 late reply after a
    // genuine 504). The state gate accepts the reply ONLY when the work is in
    // `replied` (a real CLI RESOLVE landed); for any other state (delivered/
    // orphaned/cancelled) it RESTORES the marker and falls through to timeout,
    // so the sweep's notification still reaches a future request that re-enters
    // the loop, and the timeout transition still runs.
    const finalReply = (await this.redis.eval(
      FINAL_READ_SCRIPT,
      2,
      replyKey,
      this.workHash(workId),
      String(REPLY_TTL_SEC),
    )) as string | null;
    if (finalReply !== null) {
      try {
        return JSON.parse(finalReply) as CliReply;
      } catch {
        return { status: 502, body: { error: "invalid_cli_reply" } };
      }
    }

    await this.timeout(sessionId, workId);
    return { status: 504, body: { error: "cli_timeout" } };
  }

  // ── Agent poll ──

  async poll(
    sessionId: string,
    epoch: number,
    token: string,
    wait: boolean,
  ): Promise<PollResult> {
    const holdUntil = Date.now() + 5_000;
    for (;;) {
      const result = await this.pollClaim(sessionId, epoch, token);
      if (result.kind === "work" || result.kind === "rejected") {
        return result;
      }
      if (!wait || Date.now() >= holdUntil) {
        return { kind: "empty" };
      }
      await sleep(50);
    }
  }

  private async pollClaim(
    sessionId: string,
    epoch: number,
    token: string,
  ): Promise<PollResult> {
    const now = Date.now();
    const res = (await this.redis.eval(
      POLL_CLAIM_SCRIPT,
      2,
      this.sessionHash(sessionId),
      this.workqKey(sessionId),
      sessionId,
      String(epoch),
      token,
      String(now),
      String(SESSION_TTL_SEC),
      this.workPrefix,
    )) as string[];

    if (res[0] === "err") {
      const reason = res[1] as "unknown_session" | "stale_epoch" | "bad_token";
      return { kind: "rejected", reason };
    }
    if (res[0] === "empty") {
      return { kind: "empty" };
    }
    const wid = res[1];
    const type = res[2] as WorkType;
    let payload: unknown = undefined;
    if (res[3] !== undefined && res[3] !== null) {
      try {
        payload = JSON.parse(res[3]);
      } catch {
        payload = res[3];
      }
    }
    return { kind: "work", work_id: wid, type, payload };
  }

  // ── Agent reply (+ failure markers) ──

  async reply(
    correlation: Correlation,
    _byteLength: number,
    replyBody: CliReply,
  ): Promise<{ ok: boolean; reason?: ReplyRejectReason }> {
    return this.resolve(correlation, replyBody);
  }

  /** Writes a correlated 413 marker for an over-cap reply body. */
  failOverCap(correlation: Correlation): Promise<{ ok: boolean; reason?: ReplyRejectReason }> {
    return this.resolve(correlation, { status: 413, body: { error: "reply_too_large" } });
  }

  /** Writes a correlated 502 marker for a malformed reply envelope. */
  failInvalid(correlation: Correlation): Promise<{ ok: boolean; reason?: ReplyRejectReason }> {
    return this.resolve(correlation, { status: 502, body: { error: "invalid_cli_reply" } });
  }

  private async resolve(
    correlation: Correlation,
    replyBody: CliReply,
  ): Promise<{ ok: boolean; reason?: ReplyRejectReason }> {
    const res = (await this.redis.eval(
      RESOLVE_SCRIPT,
      5,
      this.sessionHash(correlation.sessionId),
      this.workHash(correlation.workId),
      this.replyKey(correlation.workId),
      this.inflightKey(correlation.sessionId),
      this.globalInflightKey(),
      correlation.sessionId,
      String(correlation.epoch),
      correlation.token,
      correlation.workId,
      JSON.stringify(replyBody),
      String(REPLY_TTL_SEC),
    )) as string[];

    if (res[0] === "ok") {
      return { ok: true };
    }
    const reason = res[1] as ReplyRejectReason;
    return { ok: false, reason };
  }

  // ── Internal transitions ──

  private async timeout(sessionId: string, workId: string): Promise<void> {
    await this.redis.eval(
      TIMEOUT_SCRIPT,
      5,
      this.workHash(workId),
      this.workqKey(sessionId),
      this.inflightKey(sessionId),
      this.globalInflightKey(),
      this.sessionHash(sessionId),
      workId,
      String(Date.now()),
      String(ORPHAN_TTL_SEC),
      String(SESSION_TTL_SEC),
    );
  }

  private async cancel(sessionId: string, workId: string): Promise<void> {
    await this.redis.eval(
      CANCEL_SCRIPT,
      5,
      this.workHash(workId),
      this.workqKey(sessionId),
      this.inflightKey(sessionId),
      this.globalInflightKey(),
      this.sessionHash(sessionId),
      workId,
      String(Date.now()),
      String(ORPHAN_TTL_SEC),
      String(SESSION_TTL_SEC),
    );
  }

  private async sweep(): Promise<void> {
    await this.redis.eval(
      SWEEP_SCRIPT,
      1,
      this.sessionsKey(),
      String(Date.now()),
      String(EXPIRY_MS),
      this.sessionPrefix,
      `${this.prefix}:inflight:`,
      this.workPrefix,
      this.replyPrefix,
      `${this.prefix}:workq:`,
      String(REPLY_TTL_SEC),
      String(ORPHAN_TTL_SEC),
    );
  }
}
