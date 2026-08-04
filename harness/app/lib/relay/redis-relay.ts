/**
 * Redis-backed relay coordination layer.
 *
 * The relay holds NO run/session database: it is a register / poll / reply
 * message bus whose COORDINATION state (session registry, bounded work
 * queue, consume-once replies) lives in Redis so a browser request served by
 * one `next start` invocation and a CLI poll served by a DIFFERENT
 * invocation rendezvous. Run output / prompts / inventory / documents stay on
 * the host CLI; the reply body transits Redis only as the transient,
 * consume-once coordination message.
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
 *   - `<p>:receipts:<sid>`     LIST of CLI write receipts (test-only audit hook).
 *
 * Per-work state machine:
 *   queued → delivered → replied                 (happy path)
 *   queued → cancelled                           (browser abort / 504 on queued)
 *   delivered → orphaned → late_replied          (browser 504 then ≤1 late reply)
 *   any in-flight → expired                      (session swept / re-registered)
 *
 * Atomicity: every transition is ONE Lua script (no app-level read-then-write).
 * `timeout` and `reply` are COMPETING Lua transitions on the same record —
 * exactly one wins. Replies are consumed atomically via GETDEL inside the
 * resolve script (never GET then DELETE).
 */

import { Redis } from "ioredis";

/** Operation deadlines the browser routes bound each work item by. */
export const DEADLINE_MS = {
  status: 5_000,
  runDetail: 5_000,
  mutation: 10_000,
  inventory: 30_000,
  // A chat turn (plan Slice 5) can run a REAL Claude call (a10) — this must
  // stay comfortably above the CLI's own aiPromptTimeout / the a10 spec's
  // AI_TERMINAL_TIMEOUT_MS (both ~15-20 minutes), or the browser's wait gives
  // up long before the CLI's reply lands: the relay would return a spurious
  // 504 while Claude is still legitimately thinking, and the eventual late
  // CLI reply is dropped (no one is still waiting for it) — the edit would
  // NEVER reach the browser even though the CLI genuinely committed it.
  chat: 20 * 60_000,
} as const;

/** Default relay tuning; per-session queue cap is overridable via env. */
export const RELAY_CONFIG = {
  expiryMs: 10_000,
  perSessionQueueMax: perSessionQueueMaxFromEnv(),
  globalQueueMax: globalQueueMaxFromEnv(),
  replyBudgetBytes: 1_048_576,
} as const;

const EXPIRY_MS = RELAY_CONFIG.expiryMs;

function deployedWaitCeilingMs(): number {
  if (process.env.HARNESS_DEPLOYED !== "1") {
    return Number.POSITIVE_INFINITY;
  }
  const raw = process.env.HARNESS_MAX_WAIT_MS;
  const parsed = raw === undefined ? Number.NaN : Number(raw);

  return Number.isFinite(parsed) && parsed > 0 ? Math.floor(parsed) : 14_000;
}

const SESSION_TTL_SEC = 60 * 60;
const WORK_TTL_SEC = 60 * 60;
const REPLY_TTL_SEC = replyTtlSecFromEnv();
const ORPHAN_TTL_SEC = orphanTtlSecFromEnv();
const RECEIPTS_TTL_SEC = 60 * 60;
const RECEIPTS_MAX_PER_SESSION = 256;

function replyTtlSecFromEnv(): number {
  const raw = process.env.HARNESS_REPLY_TTL_SEC;
  if (raw === undefined) return 60;
  const parsed = Number(raw);

  return Number.isFinite(parsed) && parsed > 0 ? Math.floor(parsed) : 60;
}

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

// ── Public types ──

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

/** A CliReply plus the relay's own work_id for THIS request (once a work item
 * was actually created) — callers correlate a receipt to its exact request. */
export interface RelayReply extends CliReply {
  workId?: string;
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

// `doc_tree`/`doc_read`/`doc_write` and `chat` are the workspace file-as-source
// surface (plan Slice 0/5) — additive to the protocol-era query/dispatch/answer.
export type WorkType = "query" | "dispatch" | "answer" | "doc_tree" | "doc_read" | "doc_write" | "chat";

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

/** One CLI-originated write receipt recorded by the test-only audit hook. */
export interface WriteReceipt {
  path: string;
  committed_revision: string;
  content_hash: string;
  work_id: string;
  bytes?: number;
}

// ── Lua scripts (one per atomic transition) ──

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
  if redis.call('EXISTS', shash) == 1 then
    redis.call('HSET', shash, 'lastSeen', now)
    redis.call('EXPIRE', shash, session_ttl)
  end
  return 'orphaned'
end
return state or 'missing'
`;

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
redis.call('SET', reply_key, reply, 'EX', reply_ttl)
return nil
`;

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
  private readonly workPrefix: string;
  private readonly sessionPrefix: string;
  private readonly replyPrefix: string;
  private readonly sweepTimer: ReturnType<typeof setInterval>;

  constructor(redis: Redis, prefix: string) {
    this.redis = redis;
    this.prefix = prefix;
    this.sessionPrefix = `${prefix}:session:`;
    this.workPrefix = `${prefix}:work:`;
    this.replyPrefix = `${prefix}:reply:`;
    this.sweepTimer = setInterval(() => {
      this.sweep().catch(() => {});
    }, 2_000);
    if (typeof this.sweepTimer.unref === "function") {
      this.sweepTimer.unref();
    }
  }

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
  private receiptsKey(sid: string): string {
    return `${this.prefix}:receipts:${sid}`;
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
    await this.sweep();

    return this.sessionAlive(sid);
  }

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
  ): Promise<RelayReply> {
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

        return { status: 499, body: { error: "client_aborted" }, workId };
      }
      const replyJson = await this.redis.getdel(replyKey);
      if (replyJson !== null) {
        try {
          const parsed = JSON.parse(replyJson) as CliReply;
          this.maybeRecordReceipt(sessionId, workId, parsed);

          return { ...parsed, workId };
        } catch {
          return { status: 502, body: { error: "invalid_cli_reply" }, workId };
        }
      }
      await sleep(50);
    }

    const finalReply = (await this.redis.eval(
      FINAL_READ_SCRIPT,
      2,
      replyKey,
      this.workHash(workId),
      String(REPLY_TTL_SEC),
    )) as string | null;
    if (finalReply !== null) {
      try {
        const parsed = JSON.parse(finalReply) as CliReply;
        this.maybeRecordReceipt(sessionId, workId, parsed);

        return { ...parsed, workId };
      } catch {
        return { status: 502, body: { error: "invalid_cli_reply" }, workId };
      }
    }

    await this.timeout(sessionId, workId);

    return { status: 504, body: { error: "cli_timeout" }, workId };
  }

  // ── Test-only receipt-audit hook (S1 oracle, gated by HARNESS_RECEIPT_AUDIT) ──

  /** Records a CLI-originated write receipt from a SUCCESSFUL doc_write reply. */
  private maybeRecordReceipt(sessionId: string, workId: string, reply: CliReply): void {
    if (process.env.HARNESS_RECEIPT_AUDIT !== "1") {
      return;
    }
    if (reply.status !== 200 || typeof reply.body !== "object" || reply.body === null) {
      return;
    }
    const body = reply.body as { receipt?: Partial<WriteReceipt> };
    const receipt = body.receipt;
    if (
      receipt === undefined ||
      typeof receipt.path !== "string" ||
      typeof receipt.content_hash !== "string" ||
      typeof receipt.committed_revision !== "string"
    ) {
      return;
    }

    const full: WriteReceipt = {
      path: receipt.path,
      committed_revision: receipt.committed_revision,
      content_hash: receipt.content_hash,
      work_id: workId,
      bytes: receipt.bytes,
    };

    void this.recordReceipt(sessionId, full);
  }

  private async recordReceipt(sessionId: string, receipt: WriteReceipt): Promise<void> {
    const key = this.receiptsKey(sessionId);
    try {
      await this.redis.rpush(key, JSON.stringify(receipt));
      await this.redis.ltrim(key, -RECEIPTS_MAX_PER_SESSION, -1);
      await this.redis.expire(key, RECEIPTS_TTL_SEC);
    } catch {
      // Best-effort — the audit hook must never affect the primary reply path.
    }
  }

  /** Reads every receipt the audit hook recorded for this session (test-only). */
  async listReceipts(sessionId: string): Promise<WriteReceipt[]> {
    const raw = await this.redis.lrange(this.receiptsKey(sessionId), 0, -1);

    return raw
      .map((entry) => {
        try {
          return JSON.parse(entry) as WriteReceipt;
        } catch {
          return null;
        }
      })
      .filter((entry): entry is WriteReceipt => entry !== null);
  }

  // ── Agent poll ──

  async poll(sessionId: string, epoch: number, token: string, wait: boolean): Promise<PollResult> {
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

  private async pollClaim(sessionId: string, epoch: number, token: string): Promise<PollResult> {
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
    const wid = res[1] ?? "";
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

  failOverCap(correlation: Correlation): Promise<{ ok: boolean; reason?: ReplyRejectReason }> {
    return this.resolve(correlation, { status: 413, body: { error: "reply_too_large" } });
  }

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
