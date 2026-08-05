/**
 * Per-test environment for the workspace file-as-source specs (plan §"End-to-end
 * test cases"). A thin wrapper over ProtocolEnv that:
 *
 *   - starts one harness CONTAINER (no host-folder mount — the S1 relay-ruled-out
 *     fact), a `true-bdd remote` CLI agent with cwd = a materialized
 *     `w-workspace-happy` fixture, and waits for the session to appear in
 *     `GET /api/sessions`;
 *   - exposes `{ sid, baseURL, fixtureDir }`;
 *   - reads the host fixture folder (the CLI's cwd) for the S1 on-disk oracle
 *     (`readDocOnDisk` / `waitForDocOnDisk`) and a full-byte snapshot of every
 *     allowed document (the authoritative "no write" negative oracle);
 *   - observes the browser `doc_write` RESPONSE receipts (via a page response
 *     listener) and reads the test-only relay RECEIPT-AUDIT hook keyed by
 *     work_id — together the two-part S1 oracle (browser → relay → CLI → disk).
 *
 * The CLI receipt proves the CLI (not merely some host process) wrote the file;
 * `/api/agent/reply` is CLI→relay traffic a browser interceptor cannot see, so
 * the audit hook is the only browser-side window onto it.
 */

import crypto from "node:crypto";
import fs from "node:fs";
import path from "node:path";

import type { Page, Response, TestInfo } from "@playwright/test";

import { pollUntil, type PollOptions, type SessionSummary } from "./api-client";
import { ProtocolEnv } from "./protocol-env";
import type { RemoteProcess } from "./remote-process";

/** The workspace fixture every `w*` spec (and a10) materializes. */
export const WORKSPACE_FIXTURE = "w-workspace-happy";

/**
 * The FIXED documents persistence may touch. The FULL allowlist also includes
 * every `docs/prd/stories/*.yaml` — enumerated dynamically per snapshot (see
 * `allowedDocPaths`) so the negative oracle detects a story-file modification OR
 * a NEW story file created via ANY route (the plan allowlist).
 */
export const FIXED_DOCS = [
  "docs/architecture/architecture.yaml",
  "docs/prd/prd.yaml",
  "docs/prd/features.yaml",
  "docs/scenarios.yaml",
] as const;

/**
 * Test-only relay receipt-audit route (gated off in production; enabled by the
 * HARNESS_RECEIPT_AUDIT server env this helper injects). Returns the
 * CLI-originated write receipts the relay recorded, keyed by work_id.
 */
export const RECEIPT_AUDIT_ROUTE = "/api/_test/receipts";

/**
 * A CLI write receipt (browser `doc_write` RESPONSE body AND relay audit record).
 * `work_id` is REQUIRED so the two receipts are correlated by the SAME work item
 * — matching only on path/content_hash could let an unrelated write with
 * identical bytes satisfy the audit assertion.
 */
export interface WriteReceipt {
  path: string;
  committed_revision: string;
  content_hash: string;
  work_id: string;
  bytes?: number;
}

export interface WorkspaceEnvOptions {
  /**
   * Enable the deterministic (non-Claude) chat driver on the CLI remote so
   * protocol `w6` chat cases exercise the buffer→doc_write path with a scripted
   * structured result. a10 passes `false` to prove the REAL Claude turn.
   */
  deterministicChat?: boolean;
}

const DEFAULT_DOC_WAIT_MS = 20_000;

const sha256 = (bytes: string | Buffer): string =>
  crypto.createHash("sha256").update(bytes).digest("hex");

/** A unique per-run token to type into the editor (defeats hardcoded-string passes). */
export function runToken(prefix = "tok"): string {
  return `${prefix}-${crypto.randomUUID().slice(0, 12)}`;
}

export class WorkspaceEnv {
  /** Session id of the connected CLI agent (the workspace edits one project). */
  sid!: string;
  /** Materialized fixture folder on the host — the CLI's cwd (the S1 oracle root). */
  fixtureDir!: string;
  /** Baseline tree hash captured at materialize time. */
  baseline!: Record<string, string>;

  private remote!: RemoteProcess;

  private constructor(readonly env: ProtocolEnv) {}

  get baseURL(): string {
    return this.env.server.baseURL;
  }

  /** The restartable harness container (w4.3). */
  get server() {
    return this.env.server;
  }

  get api() {
    return this.env.api;
  }

  /** The CLI remote's pid — for a10's ClaudeCallBudget descendant tracking. */
  get remotePid(): number {
    return this.remote.pid;
  }

  static async start(slug: string, options: WorkspaceEnvOptions = {}): Promise<WorkspaceEnv> {
    const protocol = await ProtocolEnv.start(slug, {
      // Enable the test-only receipt-audit seam (the S1 CLI-receipt oracle).
      serverEnv: { HARNESS_RECEIPT_AUDIT: "1" },
    });
    const ws = new WorkspaceEnv(protocol);
    await ws.connect(options);

    return ws;
  }

  /** Materialize the workspace fixture, spawn the CLI agent, await its session. */
  private async connect(options: WorkspaceEnvOptions): Promise<void> {
    const fixture = await this.env.materialize(WORKSPACE_FIXTURE);
    this.fixtureDir = fixture.target;
    this.baseline = fixture.baseline;

    const deterministicChat = options.deterministicChat ?? true;
    this.remote = await this.env.startRemote(fixture.target, {
      env: deterministicChat ? { TRUE_BDD_CHAT_DRIVER: "deterministic" } : {},
    });

    const session = await this.env.api.waitForSession((candidate) => candidate.pid === this.remote.pid);
    this.sid = session.session_id;
    this.env.note({ sessionId: this.sid });
  }

  /** Kills the CLI agent so its session disconnects (w4.5 negative-S1 proof). */
  async killAgent(): Promise<void> {
    await this.remote.killTree();
  }

  /** Waits until this env's session disappears from the registry (sessions are gone on disconnect). */
  waitForSessionGone(options?: Partial<PollOptions>): Promise<void> {
    return this.env.api.waitForSessionGone((s) => s.session_id === this.sid, options);
  }

  /**
   * Waits for the agent to RE-REGISTER after a harness restart (w4.3). The
   * client-owned session id is process-stable, so the same session reappears.
   */
  waitForReconnect(options?: Partial<PollOptions>): Promise<SessionSummary> {
    return this.env.api.waitForSession((s) => s.session_id === this.sid, options);
  }

  /**
   * Appends a UNIQUE per-run token as a trailing YAML comment to a doc on disk
   * (keeps the file valid YAML) and returns it. Seeded BEFORE navigation so the
   * FilesProvider's `doc_read` picks it up — the anti-placeholder discipline: a
   * hardcoded rendered string cannot satisfy a per-run token.
   */
  seedTokenInDoc(relPath: string, prefix = "run"): string {
    const token = `${prefix}-${crypto.randomUUID().slice(0, 12)}`;
    fs.appendFileSync(path.join(this.fixtureDir, relPath), `\n# ${prefix}-token: ${token}\n`);

    return token;
  }

  // ── S1 on-disk oracle (reads the CLI's cwd on the host) ──

  /** Reads a workspace document's current bytes from disk (throws if absent). */
  readDocOnDisk(relPath: string): string {
    return fs.readFileSync(path.join(this.fixtureDir, relPath), "utf8");
  }

  /** SHA-256 of a document's on-disk bytes (matches the CLI receipt's content_hash). */
  hashDocOnDisk(relPath: string): string {
    return sha256(fs.readFileSync(path.join(this.fixtureDir, relPath)));
  }

  /**
   * Polls the on-disk document until `predicate(bytes)` holds. Deterministic S1
   * success wait — never a fixed sleep. Returns the satisfying bytes.
   */
  waitForDocOnDisk(
    relPath: string,
    predicate: (bytes: string) => boolean,
    options?: Partial<PollOptions>,
  ): Promise<string> {
    return pollUntil(
      async () => {
        let bytes: string;
        try {
          bytes = this.readDocOnDisk(relPath);
        } catch {
          return undefined;
        }

        return predicate(bytes) ? bytes : undefined;
      },
      { timeoutMs: DEFAULT_DOC_WAIT_MS, what: `${relPath} on disk to satisfy predicate`, ...options },
    );
  }

  /** Every story file currently on disk (repo-relative), for P22 new-file oracles. */
  listStoryFilesOnDisk(): string[] {
    const dir = path.join(this.fixtureDir, "docs", "prd", "stories");
    try {
      return fs
        .readdirSync(dir)
        .filter((name) => name.endsWith(".yaml"))
        .map((name) => `docs/prd/stories/${name}`);
    } catch {
      return [];
    }
  }

  /** Polls until a story file's bytes satisfy `predicate` (a NEW story created via P22). */
  waitForStoryFile(
    predicate: (bytes: string, relPath: string) => boolean,
    options?: Partial<PollOptions>,
  ): Promise<string> {
    return pollUntil(
      async () => {
        for (const rel of this.listStoryFilesOnDisk()) {
          const bytes = this.readDocOnDisk(rel);
          if (predicate(bytes, rel)) {
            return rel;
          }
        }

        return undefined;
      },
      { timeoutMs: DEFAULT_DOC_WAIT_MS, what: "a story file to satisfy predicate", ...options },
    );
  }

  /** Every allowed document currently on disk: the 4 fixed docs + all story files. */
  allowedDocPaths(): string[] {
    return [...FIXED_DOCS, ...this.listStoryFilesOnDisk()];
  }

  /**
   * Full byte snapshot of every allowed document (fixed docs + all story files)
   * — the AUTHORITATIVE "no write" oracle. Absent files are recorded so a NEW
   * file appears as a change.
   */
  snapshotDocs(): Record<string, string> {
    const snapshot: Record<string, string> = {};
    for (const rel of this.allowedDocPaths()) {
      try {
        snapshot[rel] = sha256(fs.readFileSync(path.join(this.fixtureDir, rel)));
      } catch {
        snapshot[rel] = "<absent>";
      }
    }

    return snapshot;
  }

  /**
   * Docs whose bytes changed since `before` (empty ⇒ nothing was written by ANY
   * route). Compares the UNION of paths, so a newly-created story file (present
   * in `after`, absent in `before`) is detected.
   */
  changedDocsSince(before: Record<string, string>): string[] {
    const after = this.snapshotDocs();
    const keys = new Set([...Object.keys(before), ...Object.keys(after)]);

    return [...keys].filter((rel) => (before[rel] ?? "<absent>") !== (after[rel] ?? "<absent>")).sort();
  }

  /**
   * Appends a UNIQUE valid feature record (id + description only) to
   * features.yaml on disk and returns its id. Seeded BEFORE navigation so
   * reassignment cases (w5.4/w5.7) target a per-run feature — a hardcoded picker
   * cannot satisfy them (anti-placeholder discipline).
   */
  seedFeatureOnDisk(prefix = "feat"): string {
    const id = `${prefix}-${crypto.randomUUID().slice(0, 12)}`;
    fs.appendFileSync(
      path.join(this.fixtureDir, "docs/prd/features.yaml"),
      `  - id: ${id}\n    description: seeded unique feature\n`,
    );

    return id;
  }

  // ── S1 CLI-receipt oracle (the two-part proof's second half) ──

  /**
   * Reads the test-only relay receipt-audit hook — the CLI-originated write
   * receipts the relay recorded, keyed by work_id. `/api/agent/reply` itself is
   * CLI→relay traffic a browser interceptor cannot see, so this is the only
   * browser-side window onto the CLI receipt.
   */
  async readReceiptAudit(): Promise<WriteReceipt[]> {
    const url = new URL(RECEIPT_AUDIT_ROUTE, this.baseURL);
    url.searchParams.set("sid", this.sid);
    const response = await fetch(url.href, { headers: { origin: this.baseURL } });
    if (response.status !== 200) {
      throw new Error(`receipt-audit route ${RECEIPT_AUDIT_ROUTE} returned ${response.status} (wanted 200)`);
    }
    const body = (await response.json()) as { receipts: WriteReceipt[] };

    return body.receipts;
  }

  async teardown(testInfo: TestInfo): Promise<void> {
    await this.env.teardown(testInfo);
  }
}

/**
 * Installs a browser observer over the `doc_write` route: captures every
 * response's parsed receipt (the browser-side half of the S1 oracle) and counts
 * requests (the diagnostic "zero doc_write" supplement — the byte snapshot is
 * the authoritative negative oracle). Call BEFORE navigating/editing.
 */
export function installDocWriteSpy(page: Page): DocWriteSpy {
  return new DocWriteSpy(page);
}

export class DocWriteSpy {
  readonly receipts: WriteReceipt[] = [];
  private requests = 0;
  private readonly waiters: Array<(receipt: WriteReceipt) => void> = [];

  constructor(page: Page) {
    page.on("request", (request) => {
      if (this.isDocWrite(request.url()) && request.method() === "POST") {
        this.requests += 1;
      }
    });
    page.on("response", (response: Response) => {
      if (!this.isDocWrite(response.url()) || response.request().method() !== "POST") {
        return;
      }
      if (!response.ok()) {
        return;
      }
      void response
        .json()
        .then((body) => {
          const receipt = (body as { receipt?: WriteReceipt }).receipt ?? (body as WriteReceipt);
          if (receipt && typeof receipt.content_hash === "string") {
            this.receipts.push(receipt);
            const waiter = this.waiters.shift();
            if (waiter !== undefined) waiter(receipt);
          }
        })
        .catch(() => {
          /* non-JSON body — ignore. */
        });
    });
  }

  private isDocWrite(url: string): boolean {
    return /\/docs\/write(\?|$)/.test(url);
  }

  /** Number of `doc_write` POSTs the browser issued (diagnostic — not conclusive alone). */
  get writeCount(): number {
    return this.requests;
  }

  /** Resolves with the next accepted `doc_write` receipt (browser-side S1 half). */
  waitForReceipt(timeoutMs = 20_000): Promise<WriteReceipt> {
    const existing = this.receipts.at(-1);
    if (existing !== undefined) {
      return Promise.resolve(existing);
    }

    return new Promise<WriteReceipt>((resolve, reject) => {
      const timer = setTimeout(() => reject(new Error(`no doc_write receipt within ${timeoutMs}ms`)), timeoutMs);
      this.waiters.push((receipt) => {
        clearTimeout(timer);
        resolve(receipt);
      });
    });
  }
}
