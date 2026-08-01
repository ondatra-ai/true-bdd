/**
 * Typed thin client for the harness HTTP API — THE v2 protocol contract
 * (plan §2/§3) that the STATELESS RELAY + CLI-owned store are built to
 * satisfy. The protocol specs (p1–p16) assert against exactly these
 * routes, bodies, headers, and status codes; see helpers/README-testids.md
 * for the summary table the implementer works from.
 *
 * v2 shape changes from v1 (plan §1/§2/§3, critique §1/§2/§3/§4):
 *   - The relay holds NO database. All run/session state lives in the
 *     per-project CLI store (`<folder>/tmp/true-bdd-state.db`). The relay
 *     is a register/poll/reply message bus (§2) plus a session-scoped
 *     browser read/mutate surface (§3).
 *   - Run routes are SESSION-SCOPED (the global `/api/runs/:id` is
 *     impossible without server state — critique §1):
 *       GET  /api/sessions/:sid/runs/:rid
 *       POST /api/sessions/:sid/runs
 *       POST /api/sessions/:sid/runs/:rid/answer
 *   - `/api/sessions` is REGISTRY-ONLY: every listed session is connected
 *     by definition; NO reachability, NO active_run_id, NO
 *     inventory_generation (critique §2). Sessions are GONE on disconnect.
 *   - Refresh is a LIVE READ, not a mutation — the POST /refresh route and
 *     the mark-abandoned route are DELETED (plan §1.5/§6).
 *   - Browser status mapping (§3): 404 session_gone, 504 cli_timeout,
 *     502 invalid_cli_reply, 413 reply_too_large, 409 CLI domain conflict,
 *     429/503 capacity.
 *
 * Origin discipline (plan §2/§3.8): browser-family mutations REQUIRE the
 * exact allowed Origin, so this client sends `Origin: <baseURL>` on every
 * request — exactly like the real UI's fetch calls. Requests that must
 * control Origin/Host precisely (P6/P6b) use per-request header overrides
 * or the raw node:http client below. Agent-protocol helpers (register /
 * poll / reply) always go through the raw client so their loopback-Host /
 * absent-Origin discipline is exercised (P6/P6b/P11).
 */

import http from "node:http";
import { randomUUID } from "node:crypto";

import { request as playwrightRequest, type APIRequestContext, type APIResponse } from "@playwright/test";

// ── Poll ceilings (protocol project has a 2-minute test timeout) ──

export const SESSION_APPEAR_TIMEOUT_MS = 30_000;
export const SESSION_GONE_TIMEOUT_MS = 30_000;
export const RUN_TERMINAL_TIMEOUT_MS = 60_000;
export const INVENTORY_READ_TIMEOUT_MS = 30_000;

// ── Browser status-mapping contract (plan §3, critique §4) ──

export const STATUS = {
  ok: 200,
  created: 201,
  no_content: 204,
  bad_request: 400,
  forbidden: 403,
  session_gone: 404,
  conflict: 409,
  reply_too_large: 413,
  unsupported_media: 415,
  invalid_cli_reply: 502,
  capacity_service: 503,
  cli_timeout: 504,
  capacity_rate: 429,
} as const;

// ── Operation deadlines (plan §3): the relay bounds each browser op ──

export const OPERATION_DEADLINES_MS = {
  status: 5_000,
  runDetail: 5_000,
  mutation: 10_000,
  inventory: 30_000,
} as const;

// ── Wire types (plan §2/§3) ──

/**
 * The typed command allowlist — anything else is a 400. `prompt-probe`
 * is the hidden, deterministic non-Claude prompt driver dispatchable in
 * protocol tests (plan §4, P14).
 */
export const COMMANDS = [
  "version",
  "us-create",
  "us-refine",
  "us-apply",
  "build-tests",
  "build-code",
  "prompt-probe",
] as const;

export type HarnessCommand = (typeof COMMANDS)[number];

export type RunState =
  | "queued"
  | "claimed"
  | "running"
  | "prompt_published"
  | "answer_accepted"
  | "answer_consumed"
  | "terminal";

/** Terminal classification; `error` carries `error_detail`. */
export type RunOutcome =
  | "ok"
  | "converged"
  | "not_fixed"
  | "user_exit"
  | "max_attempts"
  | "interrupted"
  | "abandoned"
  | "error";

export type RunErrorDetail = "spawn" | "no_result" | "contradiction" | "folder_locked";

/**
 * Registry-only session summary (plan §3, critique §2). Every listed
 * session is CONNECTED by definition — no reachability, no active_run_id,
 * no inventory_generation. `session_id` is CLIENT-OWNED and process-stable
 * (the incarnation id): a relay restart does not change it.
 */
export interface SessionSummary {
  session_id: string;
  /** Canonical folder — realpath of the remote's cwd (plan §1). */
  folder: string;
  /** The remote process's pid (disambiguates same-folder sessions). */
  pid: number;
  version: string;
  /** Wall-clock ms of the register that opened this connection. */
  connected_at: number;
}

/** The full terminal envelope (diagnostics beyond the badge). */
export interface TerminalEnvelope {
  classification: RunOutcome | null;
  engine_outcome: string | null;
  finalization_ok: boolean | null;
  exit_code: number | null;
  signal: string | null;
}

export interface RunSummary {
  run_id: string;
  /** The owning CLI incarnation (distinct from the serving session). */
  owner_id: string;
  command: HarnessCommand;
  story_id: string | null;
  fix: boolean;
  state: RunState;
  /** null until state === "terminal". */
  outcome: RunOutcome | null;
  error_detail: RunErrorDetail | null;
  /**
   * Cross-owner guard (plan §1.1): true only when this run's owner is the
   * serving live session AND a local active executor/prompt exists. Other
   * owners' runs are read-only through this session.
   */
  answerable: boolean;
  created_at: number;
  updated_at: number;
}

/** A same-project active owner, surfaced for the sibling warning (§3). */
export interface ActiveOwner {
  owner_id: string;
  /** The owning live session, when it is currently connected. */
  session_id: string | null;
  run_id: string;
  command: HarnessCommand;
}

/**
 * One `session_status` CLI work item (plan §3, critique §2): active run,
 * project run history, and same-project active owners — from ONE SQLite
 * snapshot, no relay-side join or browser fan-out.
 */
export interface SessionStatus {
  session_id: string;
  owner_id: string;
  active_run: RunSummary | null;
  /** Project run history, newest first (latest N). */
  runs: RunSummary[];
  /** Same-project active owners (drives the sibling warning). */
  active_owners: ActiveOwner[];
}

/** Aggregate inventory snapshot; the fit ladder stays (critique §12). */
export interface InventorySnapshot {
  snapshot_truncated: boolean;
  stories_omitted: number;
  limit_too_small: boolean;
  [key: string]: unknown;
}

/**
 * One `session_detail` CLI work item (plan §3): `session_status` PLUS the
 * inventory, in one CLI-side consistent read.
 */
export interface SessionDetail extends SessionStatus {
  inventory: InventorySnapshot | null;
}

export interface RunEvent {
  seq: number;
  type: string;
  [key: string]: unknown;
}

export interface PendingPrompt {
  prompt_id: string;
  kind: "choice" | "clarify" | "freetext";
  payload: unknown;
}

export interface RunDetail extends RunSummary {
  session_id: string;
  events: RunEvent[];
  pending_prompt: PendingPrompt | null;
  envelope: TerminalEnvelope | null;
}

export interface DispatchRunBody {
  command: HarnessCommand;
  story_id?: string;
  fix: boolean;
  client_token: string;
}

/** Fresh dispatch idempotency token. */
export function newRunToken(): string {
  return `e2e-${randomUUID()}`;
}

// ── Agent protocol wire types (plan §2) ──

export interface AgentRegisterBody {
  /** Client-owned, process-stable incarnation id. */
  session_id: string;
  folder: string;
  canonical_folder: string;
  pid: number;
  start_identity: string;
  version: string;
}

export interface AgentRegisterResult {
  connection_epoch: number;
  reply_budget_bytes: number;
  capability_token: string;
}

export type WorkType = "query" | "dispatch" | "answer";

export interface WorkItem {
  work_id: string;
  session_id: string;
  connection_epoch: number;
  type: WorkType;
  payload: unknown;
  deadline: number;
}

export interface AgentPollBody {
  session_id: string;
  connection_epoch: number;
  capability_token: string;
  /** Zero-wait poll (no ≤5s hold) — for the raw protocol tests (P6/P11). */
  wait?: boolean;
}

// ── Generic bounded polling ──

export interface PollOptions {
  timeoutMs: number;
  intervalMs?: number;
  /** Human description for the timeout error. */
  what: string;
}

const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

/**
 * Polls `fn` until it returns a defined value. Thrown errors are
 * swallowed until the deadline (the API may 404 while a state is still
 * propagating); the LAST error is embedded in the timeout message so a
 * missing route fails loudly and legibly.
 */
export async function pollUntil<T>(fn: () => Promise<T | undefined>, options: PollOptions): Promise<T> {
  const interval = options.intervalMs ?? 500;
  const deadline = Date.now() + options.timeoutMs;
  let lastError: unknown;

  for (;;) {
    try {
      const value = await fn();
      if (value !== undefined) {
        return value;
      }

      lastError = undefined;
    } catch (error) {
      lastError = error;
    }

    if (Date.now() >= deadline) {
      const last =
        lastError === undefined
          ? "predicate not satisfied"
          : lastError instanceof Error
            ? lastError.message
            : String(lastError);

      throw new Error(`timed out after ${options.timeoutMs}ms waiting for ${options.what} — last: ${last}`);
    }

    await sleep(interval);
  }
}

// ── The typed client ──

async function expectStatus(response: APIResponse, wanted: number, what: string): Promise<void> {
  if (response.status() !== wanted) {
    const body = (await response.text()).slice(0, 300);

    throw new Error(`${what} returned ${response.status()} (wanted ${wanted}): ${body}`);
  }
}

export class HarnessApi {
  private constructor(
    private readonly context: APIRequestContext,
    readonly baseURL: string,
  ) {}

  static async create(baseURL: string): Promise<HarnessApi> {
    const context = await playwrightRequest.newContext({
      baseURL,
      extraHTTPHeaders: { origin: baseURL },
    });

    return new HarnessApi(context, baseURL);
  }

  async dispose(): Promise<void> {
    await this.context.dispose();
  }

  // ── Raw browser responses (status-code assertions) ──

  listSessionsResponse(): Promise<APIResponse> {
    return this.context.get("/api/sessions");
  }

  sessionDetailResponse(sessionId: string): Promise<APIResponse> {
    return this.context.get(`/api/sessions/${sessionId}`);
  }

  sessionStatusResponse(sessionId: string): Promise<APIResponse> {
    return this.context.get(`/api/sessions/${sessionId}?view=status`);
  }

  runDetailResponse(sessionId: string, runId: string, afterSeq?: number): Promise<APIResponse> {
    const suffix = afterSeq === undefined ? "" : `?after_seq=${afterSeq}`;

    return this.context.get(`/api/sessions/${sessionId}/runs/${runId}${suffix}`);
  }

  /**
   * POST /api/sessions/:id/runs. `body` is deliberately untyped so the
   * gating tests can send garbage (shell-ish strings → 400); headers
   * override the context defaults (P6 origin cases).
   */
  dispatchRunResponse(sessionId: string, body: unknown, headers?: Record<string, string>): Promise<APIResponse> {
    return this.context.post(`/api/sessions/${sessionId}/runs`, { data: body, headers });
  }

  answerRunResponse(
    sessionId: string,
    runId: string,
    answer: { prompt_id: string; value: string },
    headers?: Record<string, string>,
  ): Promise<APIResponse> {
    return this.context.post(`/api/sessions/${sessionId}/runs/${runId}/answer`, { data: answer, headers });
  }

  // ── Typed happy-path wrappers ──

  async listSessions(): Promise<SessionSummary[]> {
    const response = await this.listSessionsResponse();
    await expectStatus(response, STATUS.ok, "GET /api/sessions");
    const body = (await response.json()) as { sessions: SessionSummary[] };

    return body.sessions;
  }

  /** GET /api/sessions/:id (session_detail = status + inventory). */
  async getSession(sessionId: string): Promise<SessionDetail> {
    const response = await this.sessionDetailResponse(sessionId);
    await expectStatus(response, STATUS.ok, `GET /api/sessions/${sessionId}`);

    return (await response.json()) as SessionDetail;
  }

  /** GET /api/sessions/:id?view=status (session_status only). */
  async getSessionStatus(sessionId: string): Promise<SessionStatus> {
    const response = await this.sessionStatusResponse(sessionId);
    await expectStatus(response, STATUS.ok, `GET /api/sessions/${sessionId}?view=status`);

    return (await response.json()) as SessionStatus;
  }

  /** GET /api/sessions/:sid/runs/:rid — the ENTIRE current capped window. */
  async getRun(sessionId: string, runId: string, afterSeq?: number): Promise<RunDetail> {
    const response = await this.runDetailResponse(sessionId, runId, afterSeq);
    await expectStatus(response, STATUS.ok, `GET /api/sessions/${sessionId}/runs/${runId}`);

    return (await response.json()) as RunDetail;
  }

  /** Dispatch expecting acceptance: 201 created or 200 token-dedup. */
  async dispatchRun(sessionId: string, body: DispatchRunBody): Promise<{ runId: string; status: number }> {
    const response = await this.dispatchRunResponse(sessionId, body);
    const status = response.status();

    if (status !== STATUS.ok && status !== STATUS.created) {
      const text = (await response.text()).slice(0, 300);

      throw new Error(`POST /api/sessions/${sessionId}/runs returned ${status}: ${text}`);
    }

    const parsed = (await response.json()) as { run_id: string };

    return { runId: parsed.run_id, status };
  }

  // ── Bounded waiters ──

  waitForSession(
    predicate: (session: SessionSummary) => boolean,
    options?: Partial<PollOptions>,
  ): Promise<SessionSummary> {
    return pollUntil(async () => (await this.listSessions()).find(predicate), {
      timeoutMs: SESSION_APPEAR_TIMEOUT_MS,
      what: "session to appear in GET /api/sessions",
      ...options,
    });
  }

  /**
   * Waits until a session DISAPPEARS from the registry — sessions are GONE
   * on disconnect in v2 (plan §1.7/§3, P2). Resolves the list at the moment
   * no session matches the predicate.
   */
  waitForSessionGone(
    predicate: (session: SessionSummary) => boolean,
    options?: Partial<PollOptions>,
  ): Promise<void> {
    return pollUntil(
      async () => ((await this.listSessions()).some(predicate) ? undefined : true),
      { timeoutMs: SESSION_GONE_TIMEOUT_MS, what: "session to disappear from GET /api/sessions", ...options },
    ).then(() => undefined);
  }

  /**
   * Waits for a fresh inventory read to resolve (plan §1.5 — every read is
   * a fresh scan; there is no generation to watch). Replaces v1's
   * `waitForGeneration` in retargeted setups.
   */
  waitForInventory(sessionId: string, options?: Partial<PollOptions>): Promise<SessionDetail> {
    return pollUntil(
      async () => {
        const detail = await this.getSession(sessionId);

        return detail.inventory !== null ? detail : undefined;
      },
      { timeoutMs: INVENTORY_READ_TIMEOUT_MS, what: `session ${sessionId} to return a live inventory`, ...options },
    );
  }

  waitForRun(
    sessionId: string,
    runId: string,
    predicate: (run: RunDetail) => boolean,
    options?: Partial<PollOptions>,
  ): Promise<RunDetail> {
    return pollUntil(
      async () => {
        const run = await this.getRun(sessionId, runId);

        return predicate(run) ? run : undefined;
      },
      { timeoutMs: RUN_TERMINAL_TIMEOUT_MS, what: `run ${runId} to satisfy predicate`, ...options },
    );
  }

  waitForRunTerminal(sessionId: string, runId: string, options?: Partial<PollOptions>): Promise<RunDetail> {
    return this.waitForRun(sessionId, runId, (run) => run.state === "terminal", {
      what: `run ${runId} to reach a terminal state`,
      ...options,
    });
  }
}

// ── Raw HTTP client (P6/P6b/P11: full control of Origin AND Host) ──

export interface RawHttpOptions {
  port: number;
  method: string;
  path: string;
  /**
   * Sent verbatim. Provide `host` to override the Host header; omit
   * `origin` to send NO Origin at all (Playwright's stack cannot
   * unset or reliably override these).
   */
  headers?: Record<string, string>;
  body?: string;
  /** Raw ceiling; long-poll cases (P6/P11) override the default 15s. */
  timeoutMs?: number;
}

export interface RawHttpResult {
  status: number;
  body: string;
  headers: http.IncomingHttpHeaders;
}

export function rawHttpRequest(options: RawHttpOptions): Promise<RawHttpResult> {
  return new Promise<RawHttpResult>((resolve, reject) => {
    const request = http.request(
      {
        host: "127.0.0.1",
        port: options.port,
        method: options.method,
        path: options.path,
        headers: options.headers,
      },
      (response) => {
        const chunks: Buffer[] = [];
        response.on("data", (chunk: Buffer) => chunks.push(chunk));
        response.on("end", () =>
          resolve({
            status: response.statusCode ?? 0,
            body: Buffer.concat(chunks).toString("utf8"),
            headers: response.headers,
          }),
        );
      },
    );

    request.on("error", reject);
    request.setTimeout(options.timeoutMs ?? 15_000, () =>
      request.destroy(new Error("raw http request timed out")),
    );

    if (options.body !== undefined) {
      request.write(options.body);
    }

    request.end();
  });
}

// ── Agent protocol raw helpers (register / poll / reply, plan §2) ──

const JSON_CT = { "content-type": "application/json" };

/** POST /api/agent/register — returns the raw result plus parsed body. */
export async function registerAgent(
  port: number,
  body: AgentRegisterBody,
  headers: Record<string, string> = {},
): Promise<RawHttpResult> {
  return rawHttpRequest({
    port,
    method: "POST",
    path: "/api/agent/register",
    headers: { ...JSON_CT, ...headers },
    body: JSON.stringify(body),
  });
}

/**
 * POST /api/agent/poll. Held ≤5s by default; `wait: false` asks for the
 * zero-wait variant so the raw helper never sits past the 15s ceiling
 * (critique P6: the raw helper times out shorter than a held poll).
 */
export async function pollAgent(
  port: number,
  body: AgentPollBody,
  headers: Record<string, string> = {},
): Promise<RawHttpResult> {
  return rawHttpRequest({
    port,
    method: "POST",
    path: "/api/agent/poll",
    headers: { ...JSON_CT, ...headers },
    body: JSON.stringify(body),
    // A held poll returns within ~5s; allow generous slack under the ceiling.
    timeoutMs: 8_000,
  });
}

/**
 * POST /api/agent/reply. Auth/correlation live OUTSIDE the capped body
 * (headers: session_id, connection_epoch, capability_token, work_id) so an
 * over-cap body still yields a CORRELATED 413 to the browser waiter
 * (plan §2 — never a blind 504).
 */
export async function replyAgent(
  port: number,
  correlation: { session_id: string; connection_epoch: number; capability_token: string; work_id: string },
  resultBody: string,
  headers: Record<string, string> = {},
): Promise<RawHttpResult> {
  return rawHttpRequest({
    port,
    method: "POST",
    path: "/api/agent/reply",
    headers: {
      ...JSON_CT,
      "x-session-id": correlation.session_id,
      "x-connection-epoch": String(correlation.connection_epoch),
      "x-capability-token": correlation.capability_token,
      "x-work-id": correlation.work_id,
      ...headers,
    },
    body: resultBody,
    timeoutMs: 8_000,
  });
}
