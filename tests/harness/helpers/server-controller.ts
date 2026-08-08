/**
 * Restartable harness-server controller (perf plan Phase 1, host-process variant).
 *
 * Each test starts its OWN harness SERVER as a host `node server.js` process from
 * a per-instance clone of the cached Next standalone bundle (built once in
 * global-setup, see helpers/harness-build.ts), on its own free 127.0.0.1 port.
 * That per-instance isolation is what keeps the suite's three hard requirements
 * working without any docker container per test:
 *   - stop/start/restart on the SAME port within one test (P7 / A8 / P18) —
 *     restart respawns the process; coordination state survives in Redis;
 *   - two instances at once (P17 / P20 / P21) — two processes, two ports;
 *   - per-test env injection — REDIS_URL / REDIS_KEY_PREFIX and any serverEnv are
 *     passed straight into the child's process environment.
 *
 * The server reaches the suite-level Redis over plain loopback (redis://127.0.0.1
 * :6379) — no host-rewrite, because both live on the host now.
 *
 * The public surface (port, baseURL, pid, isRunning, start/stop/restart, dbPath,
 * logDir, stdout/stderrPath) is unchanged from the compose era, so ProtocolEnv /
 * CrossInstanceEnv and every spec are untouched. `pid` maps to a host process
 * again (the child's live pid) rather than always-undefined.
 */

import { spawn, type ChildProcess } from "node:child_process";
import { randomUUID } from "node:crypto";
import fs from "node:fs";
import net from "node:net";
import path from "node:path";

import { cloneBundle } from "./harness-build";
import { registerProcess } from "./process-registry";
import { processStartIdentity } from "./start-identity";
import { suiteContext } from "./suite-root";

const READINESS_POLL_MS = 100;
const BIND_RETRIES = 3;

const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

/**
 * Asks the kernel for a free 127.0.0.1 port. A bind race (another probe winning
 * it, or a just-freed port on restart) is handled by ServerController.start()'s
 * retry loop.
 */
export async function allocatePort(): Promise<number> {
  return new Promise<number>((resolve, reject) => {
    const probe = net.createServer();
    probe.once("error", reject);
    probe.listen(0, "127.0.0.1", () => {
      const address = probe.address();
      if (address === null || typeof address === "string") {
        probe.close();
        reject(new Error("port probe returned no address"));
        return;
      }

      const { port } = address;
      probe.close(() => resolve(port));
    });
  });
}

export interface ServerControllerOptions {
  /**
   * Directory for the server's local scratch (logs adjacency + repro manifest).
   * The relay holds NO database; coordination state lives in Redis. Kept for the
   * test artifact layout and the reproduction manifest.
   */
  dbPath: string;
  /** Directory for server.stdout.log / server.stderr.log + the per-instance bundle clone. */
  logDir: string;
  /** Pin a port; omitted → allocated on first start(). */
  port?: number;
  /** Readiness ceiling per start attempt. Default 60s. */
  readinessTimeoutMs?: number;
  /**
   * Extra environment merged into the child's process env — additive over the
   * fixed PORT/HOSTNAME/REDIS_URL/REDIS_KEY_PREFIX. The degraded-inventory P9
   * case sets a tiny MAX_INVENTORY_REQUEST_BYTES this way; p19 sets
   * HARNESS_DEPLOYED=1; workspace specs set HARNESS_RECEIPT_AUDIT=1.
   */
  env?: Record<string, string>;
}

export class ServerController {
  readonly dbPath: string;
  readonly logDir: string;
  readonly stdoutPath: string;
  readonly stderrPath: string;

  private portValue: number | undefined;
  private readonly portPinned: boolean;
  private readonly readinessTimeoutMs: number;
  private readonly extraEnv: Record<string, string>;
  /** Short id disambiguating this instance's log banners. */
  private readonly instanceId: string;
  /**
   * Per-instance copy-on-write clone of the cached standalone bundle. Cloned
   * lazily on the first start() and REUSED across restart() — a restart models a
   * respawn against the same clone, never a fresh bundle.
   */
  private bundleDir: string | undefined;
  private child: ChildProcess | undefined;
  private exitPromise: Promise<void> | undefined;
  private running = false;
  private startCount = 0;

  constructor(options: ServerControllerOptions) {
    this.dbPath = options.dbPath;
    this.logDir = options.logDir;
    this.portValue = options.port;
    this.portPinned = options.port !== undefined;
    this.readinessTimeoutMs = options.readinessTimeoutMs ?? 60_000;
    this.extraEnv = options.env ?? {};
    this.instanceId = randomUUID().slice(0, 12);
    this.stdoutPath = path.join(this.logDir, "server.stdout.log");
    this.stderrPath = path.join(this.logDir, "server.stderr.log");
    fs.mkdirSync(this.logDir, { recursive: true });
    fs.mkdirSync(path.dirname(this.dbPath), { recursive: true });
  }

  /** Assigned after the first successful start(). */
  get port(): number {
    if (this.portValue === undefined) {
      throw new Error("server port not assigned yet — call start() first");
    }

    return this.portValue;
  }

  get baseURL(): string {
    return `http://127.0.0.1:${this.port}`;
  }

  /** The live child pid while the server is running; undefined once stopped. */
  get pid(): number | undefined {
    return this.child?.pid;
  }

  isRunning(): boolean {
    return this.running;
  }

  /**
   * Starts the harness server and readiness-polls GET / until it responds. On a
   * bind/early-exit failure with a NON-pinned port a fresh port is allocated and
   * the start retried (bind race); with a pinned port the same port is retried
   * after a short delay (restart scenarios where the old listener just released).
   */
  async start(): Promise<void> {
    if (this.running) {
      throw new Error("server already running — call stop() or restart()");
    }

    // Clone the cached bundle ONCE for this instance; reused across restart().
    if (this.bundleDir === undefined) {
      this.bundleDir = await cloneBundle(
        suiteContext().standaloneBundle,
        path.join(this.logDir, "bundle"),
      );
    }

    let lastError: Error | undefined;

    for (let attempt = 0; attempt < BIND_RETRIES; attempt += 1) {
      if (this.portValue === undefined) {
        this.portValue = await allocatePort();
      }

      const outcome = await this.startOnce(this.portValue);
      if (outcome === "ready") {
        return;
      }

      lastError = new Error(
        `harness server failed on port ${this.portValue} (attempt ${attempt + 1}): ${outcome}`,
      );

      if (!this.portPinned) {
        this.portValue = undefined; // pick a fresh port next attempt
      } else {
        await sleep(1000);
      }
    }

    throw lastError ?? new Error("server failed to start");
  }

  /** SIGKILLs the server's process group and awaits its exit (crash-kill contract). */
  async stop(): Promise<void> {
    if (!this.running && this.child === undefined) {
      return;
    }

    await this.killAndAwait();
  }

  /** stop() + start() against the SAME port and bundle clone (P7/A8/P18). */
  async restart(): Promise<void> {
    const port = this.port; // throws if never started — restart implies a prior start
    await this.stop();
    this.portValue = port;
    await this.start();
  }

  /** Environment for the `node server.js` child (fixed keys over any serverEnv). */
  private childEnv(port: number): NodeJS.ProcessEnv {
    const env: NodeJS.ProcessEnv = {
      ...process.env,
      ...this.extraEnv,
      PORT: String(port),
      HOSTNAME: "127.0.0.1",
      NODE_ENV: "production",
      NEXT_TELEMETRY_DISABLED: "1",
    };

    // Plain loopback: server and Redis both live on the host now (no rewrite).
    if (env.REDIS_URL === undefined) {
      env.REDIS_URL = "redis://127.0.0.1:6379";
    }

    return env;
  }

  private async startOnce(port: number): Promise<"ready" | string> {
    this.startCount += 1;
    const banner = `\n--- start #${this.startCount} instance=${this.instanceId} port=${port} ---\n`;
    fs.appendFileSync(this.stdoutPath, banner);
    fs.appendFileSync(this.stderrPath, banner);

    const stdoutFd = fs.openSync(this.stdoutPath, "a");
    const stderrFd = fs.openSync(this.stderrPath, "a");

    const child = spawn("node", ["server.js"], {
      cwd: this.bundleDir,
      detached: true, // own process group (setpgid) → SIGKILL the whole group
      stdio: ["ignore", stdoutFd, stderrFd],
      env: this.childEnv(port),
    });

    fs.closeSync(stdoutFd);
    fs.closeSync(stderrFd);

    if (child.pid === undefined) {
      return "spawn returned no pid";
    }

    this.child = child;
    this.running = true;

    let exited: { code: number | null; signal: NodeJS.Signals | null } | null = null;
    this.exitPromise = new Promise<void>((resolve) => {
      child.once("exit", (code, signal) => {
        exited = { code, signal };
        resolve();
      });
    });

    // Register the pgid so global-teardown's emergency cleanup covers a server
    // leaked by a crashed test (idiom from remote-process.ts): record the OS
    // start identity so a recycled pid is never signalled.
    registerProcess({
      pid: child.pid,
      pgid: child.pid, // detached ⇒ session/group leader
      label: `server:${port}`,
      startedAt: new Date().toISOString(),
      startIdentity: processStartIdentity(child.pid),
    });

    // Readiness: poll GET / (100ms), failing FAST if the child exits early
    // (bind failure / crash) rather than waiting out the full timeout.
    const deadline = Date.now() + this.readinessTimeoutMs;
    while (Date.now() < deadline) {
      if (exited !== null) {
        const { code, signal } = exited;
        await this.killAndAwait();

        return `server exited during startup (code ${code}, signal ${signal}) — see ${this.stderrPath}`;
      }

      try {
        const response = await fetch(`http://127.0.0.1:${port}/`, {
          signal: AbortSignal.timeout(2000),
        });
        if (response.ok) {
          return "ready";
        }
      } catch {
        // Not accepting connections yet — keep polling.
      }

      await sleep(READINESS_POLL_MS);
    }

    await this.killAndAwait();

    return `readiness timeout after ${this.readinessTimeoutMs}ms (see ${this.stderrPath})`;
  }

  /**
   * SIGKILLs the child's process group FIRST AND ONLY — never SIGTERM — then
   * awaits the child's exit before returning.
   *
   * Abrupt SIGKILL FIRST, so a restart models a real relay-gone event (crash /
   * scale-down / cold start) rather than a graceful drain. Next standalone's
   * SIGTERM handler would close the server and let an in-flight dispatch's
   * client-abort cleanup run — which LREMs the queued work out of Redis
   * (redis-relay `cancel`). Killing the process first means that cleanup never
   * runs, so durable coordination state survives the restart (P18) exactly as it
   * did under the old `next start` replacement. Awaiting the exit before
   * returning is what keeps a same-port restart from racing the dying listener.
   */
  private async killAndAwait(): Promise<void> {
    const child = this.child;
    if (child === undefined) {
      this.running = false;

      return;
    }

    if (child.pid !== undefined && child.exitCode === null && child.signalCode === null) {
      try {
        process.kill(-child.pid, "SIGKILL"); // whole process group
      } catch {
        try {
          process.kill(child.pid, "SIGKILL"); // group already gone — try the leader
        } catch {
          // ESRCH — already dead.
        }
      }
    }

    if (this.exitPromise !== undefined) {
      await this.exitPromise;
    }

    this.child = undefined;
    this.exitPromise = undefined;
    this.running = false;
  }
}
