/**
 * Typed thin client for the harness HTTP API — THE protocol contract
 * (plan §3.3) that the server implementation is built to satisfy. The
 * protocol specs (p1–p8) assert against exactly these routes, bodies,
 * and status codes; see helpers/README-testids.md for the summary
 * table the implementer works from.
 *
 * Status-code contract:
 *   POST /api/sessions/:id/runs   → 201 {run_id} created,
 *                                   200 {run_id} client_token dedup (same id),
 *                                   400 invalid command/body,
 *                                   409 session unreachable OR any
 *                                       non-terminal run on the session,
 *                                   403 origin/host policy violation.
 *   GET  /api/sessions            → 200 {sessions: SessionSummary[]}.
 *   GET  /api/sessions/:id        → 200 SessionDetail, 404 unknown.
 *   POST /api/sessions/:id/refresh→ 202 (want_inventory flagged).
 *   GET  /api/runs/:id?after_seq=N→ 200 RunDetail, 404 unknown.
 *   POST /api/runs/:id/answer     → 200 accepted or exact retry,
 *                                   409 conflicting retry, 400 invalid.
 *   POST /api/runs/:id/abandon    → 200 (run flips terminal/abandoned),
 *                                   409 not eligible (session reachable
 *                                       or run already terminal).
 *
 * Origin discipline (plan §3.8): browser-family mutations REQUIRE the
 * exact allowed Origin, so this client sends `Origin: <baseURL>` on
 * every request — exactly like the real UI's fetch calls. Requests
 * that must control Origin/Host precisely (P6) use per-request header
 * overrides or the raw node:http client below.
 */

import http from "node:http";
import { randomUUID } from "node:crypto";

import { request as playwrightRequest, type APIRequestContext, type APIResponse } from "@playwright/test";

// ── Poll ceilings (protocol project has a 2-minute test timeout) ──

export const SESSION_APPEAR_TIMEOUT_MS = 30_000;
export const REACHABILITY_TIMEOUT_MS = 60_000;
export const RUN_TERMINAL_TIMEOUT_MS = 60_000;
export const GENERATION_TIMEOUT_MS = 30_000;

// ── Wire types (plan §3.3) ──

/** The typed command allowlist — anything else is a 400. */
export const COMMANDS = [
  "version",
  "us-create",
  "us-refine",
  "us-apply",
  "build-tests",
  "build-code",
] as const;

export type HarnessCommand = (typeof COMMANDS)[number];

export type Reachability = "connected" | "unreachable";

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

export interface SessionSummary {
  id: string;
  /** Canonical folder — realpath of the remote's cwd (plan §3.1). */
  folder: string;
  /** The remote process's pid (disambiguates same-folder sessions). */
  pid: number;
  reachability: Reachability;
  active_run_id: string | null;
  /** 0 until the first inventory snapshot is promoted. */
  inventory_generation: number;
}

export interface RunSummary {
  id: string;
  command: HarnessCommand;
  story_id: string | null;
  fix: boolean;
  state: RunState;
  /** null until state === "terminal". */
  outcome: RunOutcome | null;
  error_detail?: RunErrorDetail | null;
}

export interface SessionDetail extends SessionSummary {
  /** Run history, newest first. */
  runs: RunSummary[];
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

  // ── Raw responses (status-code assertions) ──

  listSessionsResponse(): Promise<APIResponse> {
    return this.context.get("/api/sessions");
  }

  /**
   * POST /api/sessions/:id/runs. `body` is deliberately untyped so the
   * gating tests can send garbage (shell-ish strings → 400); headers
   * override the context defaults (P6 origin cases).
   */
  dispatchRunResponse(
    sessionId: string,
    body: unknown,
    headers?: Record<string, string>,
  ): Promise<APIResponse> {
    return this.context.post(`/api/sessions/${sessionId}/runs`, { data: body, headers });
  }

  refreshSessionResponse(sessionId: string): Promise<APIResponse> {
    return this.context.post(`/api/sessions/${sessionId}/refresh`);
  }

  answerRunResponse(runId: string, answer: { prompt_id: string; value: string }): Promise<APIResponse> {
    return this.context.post(`/api/runs/${runId}/answer`, { data: answer });
  }

  markAbandonedResponse(runId: string): Promise<APIResponse> {
    return this.context.post(`/api/runs/${runId}/abandon`);
  }

  // ── Typed happy-path wrappers ──

  async listSessions(): Promise<SessionSummary[]> {
    const response = await this.listSessionsResponse();
    await expectStatus(response, 200, "GET /api/sessions");
    const body = (await response.json()) as { sessions: SessionSummary[] };

    return body.sessions;
  }

  async getSession(sessionId: string): Promise<SessionDetail> {
    const response = await this.context.get(`/api/sessions/${sessionId}`);
    await expectStatus(response, 200, `GET /api/sessions/${sessionId}`);

    return (await response.json()) as SessionDetail;
  }

  async getRun(runId: string, afterSeq?: number): Promise<RunDetail> {
    const suffix = afterSeq === undefined ? "" : `?after_seq=${afterSeq}`;
    const response = await this.context.get(`/api/runs/${runId}${suffix}`);
    await expectStatus(response, 200, `GET /api/runs/${runId}`);

    return (await response.json()) as RunDetail;
  }

  /** Dispatch expecting acceptance: 201 created or 200 token-dedup. */
  async dispatchRun(sessionId: string, body: DispatchRunBody): Promise<{ runId: string; status: number }> {
    const response = await this.dispatchRunResponse(sessionId, body);
    const status = response.status();

    if (status !== 200 && status !== 201) {
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

  waitForReachability(
    sessionId: string,
    wanted: Reachability,
    options?: Partial<PollOptions>,
  ): Promise<SessionDetail> {
    return pollUntil(
      async () => {
        const session = await this.getSession(sessionId);

        return session.reachability === wanted ? session : undefined;
      },
      {
        timeoutMs: REACHABILITY_TIMEOUT_MS,
        what: `session ${sessionId} to become ${wanted}`,
        ...options,
      },
    );
  }

  waitForRun(
    runId: string,
    predicate: (run: RunDetail) => boolean,
    options?: Partial<PollOptions>,
  ): Promise<RunDetail> {
    return pollUntil(
      async () => {
        const run = await this.getRun(runId);

        return predicate(run) ? run : undefined;
      },
      {
        timeoutMs: RUN_TERMINAL_TIMEOUT_MS,
        what: `run ${runId} to satisfy predicate`,
        ...options,
      },
    );
  }

  waitForRunTerminal(runId: string, options?: Partial<PollOptions>): Promise<RunDetail> {
    return this.waitForRun(runId, (run) => run.state === "terminal", {
      what: `run ${runId} to reach a terminal state`,
      ...options,
    });
  }

  /** Waits for inventory_generation strictly greater than `floor`. */
  waitForGeneration(sessionId: string, floor: number, options?: Partial<PollOptions>): Promise<SessionDetail> {
    return pollUntil(
      async () => {
        const session = await this.getSession(sessionId);

        return session.inventory_generation > floor ? session : undefined;
      },
      {
        timeoutMs: GENERATION_TIMEOUT_MS,
        what: `session ${sessionId} inventory generation > ${floor}`,
        ...options,
      },
    );
  }
}

// ── Raw HTTP client (P6: full control of Origin AND Host) ──

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
}

export interface RawHttpResult {
  status: number;
  body: string;
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
          }),
        );
      },
    );

    request.on("error", reject);
    request.setTimeout(15_000, () => request.destroy(new Error("raw http request timed out")));

    if (options.body !== undefined) {
      request.write(options.body);
    }

    request.end();
  });
}
