/**
 * Restartable production-server controller (plan §4.1).
 *
 * Each test starts its OWN `next start` production server on its own
 * free port with its own SQLite DB file (env TRUE_BDD_HARNESS_DB),
 * bound to 127.0.0.1. The controller supports stop/start/restart on
 * the SAME port + DB within one test (P7 / A8), captures stdout and
 * stderr to files, retries on bind races, and kills by process group.
 *
 * Requires `next build` to have run (global-setup does this once).
 */

import { spawn, type ChildProcess } from "node:child_process";
import fs from "node:fs";
import net from "node:net";
import path from "node:path";

import { registerProcess } from "./process-registry";
import { processStartIdentity } from "./start-identity";
import { suiteContext } from "./suite-root";

const READINESS_POLL_MS = 250;
const STOP_GRACE_MS = 5000;
const BIND_RETRIES = 3;

const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

/**
 * Asks the kernel for a free 127.0.0.1 port. A bind race (another
 * process grabbing it before `next start` binds) is handled by
 * ServerController.start()'s retry loop.
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
  /** SQLite file for this server (TRUE_BDD_HARNESS_DB). */
  dbPath: string;
  /** Directory for server.stdout.log / server.stderr.log. */
  logDir: string;
  /** Pin a port; omitted → allocated on first start(). */
  port?: number;
  /** Readiness ceiling per start attempt. Default 30s. */
  readinessTimeoutMs?: number;
  /**
   * Extra environment entries merged into the server process env —
   * additive, applied over process.env but under the fixed DB/PORT keys.
   * The degraded-inventory P9 case sets a tiny MAX_INVENTORY_REQUEST_BYTES
   * this way to force the remote's fit-to-budget path (plan §1a/§4.3).
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
  private child: ChildProcess | undefined;
  private startCount = 0;

  constructor(options: ServerControllerOptions) {
    this.dbPath = options.dbPath;
    this.logDir = options.logDir;
    this.portValue = options.port;
    this.portPinned = options.port !== undefined;
    this.readinessTimeoutMs = options.readinessTimeoutMs ?? 30_000;
    this.extraEnv = options.env ?? {};
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

  get pid(): number | undefined {
    return this.child?.pid;
  }

  isRunning(): boolean {
    return this.child !== undefined && this.child.exitCode === null && this.child.signalCode === null;
  }

  /**
   * Starts `next start -H 127.0.0.1 -p <port>` and readiness-polls
   * GET / until it responds. On a bind failure with a NON-pinned port
   * a fresh port is allocated and the start retried (bind race);
   * with a pinned port the same port is retried after a short delay
   * (restart scenarios where the old socket lingers).
   */
  async start(): Promise<void> {
    if (this.isRunning()) {
      throw new Error("server already running — call stop() or restart()");
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
        `next start failed on port ${this.portValue} (attempt ${attempt + 1}): ${outcome}`,
      );

      if (!this.portPinned) {
        this.portValue = undefined; // pick a fresh port next attempt
      } else {
        await sleep(1000);
      }
    }

    throw lastError ?? new Error("server failed to start");
  }

  /** SIGTERM the server's process group, escalate to SIGKILL. */
  async stop(): Promise<void> {
    const child = this.child;
    if (child === undefined || child.pid === undefined) {
      return;
    }

    const pid = child.pid;
    const exited = new Promise<void>((resolve) => {
      if (child.exitCode !== null || child.signalCode !== null) {
        resolve();
        return;
      }

      child.once("exit", () => resolve());
    });

    signalGroup(pid, "SIGTERM");

    const timedOut = await Promise.race([
      exited.then(() => false),
      sleep(STOP_GRACE_MS).then(() => true),
    ]);

    if (timedOut) {
      signalGroup(pid, "SIGKILL");
      await exited;
    }

    this.child = undefined;
  }

  /** stop() + start() against the SAME port and DB (P7/A8). */
  async restart(): Promise<void> {
    const port = this.port; // throws if never started — restart implies a prior start
    await this.stop();
    this.portValue = port;
    await this.start();
  }

  private async startOnce(port: number): Promise<"ready" | string> {
    const { harnessDir } = suiteContext();
    this.startCount += 1;

    const stdoutFd = fs.openSync(this.stdoutPath, "a");
    const stderrFd = fs.openSync(this.stderrPath, "a");
    fs.writeSync(stdoutFd, `\n--- start #${this.startCount} port=${port} ---\n`);
    fs.writeSync(stderrFd, `\n--- start #${this.startCount} port=${port} ---\n`);

    const nextBin = path.join(harnessDir, "node_modules", ".bin", "next");
    const child = spawn(nextBin, ["start", "-H", "127.0.0.1", "-p", String(port)], {
      cwd: harnessDir,
      detached: true, // own process group → group-kill cannot touch the worker
      stdio: ["ignore", stdoutFd, stderrFd],
      env: {
        ...process.env,
        ...this.extraEnv,
        TRUE_BDD_HARNESS_DB: this.dbPath,
        PORT: String(port),
        NODE_ENV: "production",
      },
    });

    fs.closeSync(stdoutFd);
    fs.closeSync(stderrFd);

    if (child.pid === undefined) {
      return "spawn returned no pid";
    }

    this.child = child;
    registerProcess({
      pid: child.pid,
      pgid: child.pid,
      label: `server:${port}`,
      startedAt: new Date().toISOString(),
      // Record the OS start identity so the suite-level emergency cleanup
      // proves this pid was not recycled before signalling (finding 9).
      startIdentity: processStartIdentity(child.pid),
    });

    let exited = false;
    child.once("exit", () => {
      exited = true;
    });

    const deadline = Date.now() + this.readinessTimeoutMs;
    while (Date.now() < deadline) {
      if (exited) {
        this.child = undefined;
        return `process exited early (see ${this.stderrPath})`;
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

    await this.stop();

    return `readiness timeout after ${this.readinessTimeoutMs}ms`;
  }
}

function signalGroup(pgid: number, signal: NodeJS.Signals): void {
  try {
    process.kill(-pgid, signal);
  } catch {
    // ESRCH — group already gone.
  }
}
