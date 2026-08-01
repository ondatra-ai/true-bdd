/**
 * Restartable harness-server controller (plan §4.1, compose e2e variant).
 *
 * Each test starts its OWN harness CONTAINER via `docker compose` (image
 * `true-bdd-harness:e2e`, built once in global-setup) on its own free host port
 * (127.0.0.1:<port> → container 4517), under a UNIQUE compose project name. That
 * per-instance isolation is what keeps the suite's three hard requirements working
 * on top of a container:
 *   - stop/start/restart on the SAME port within one test (P7 / A8 / P18) —
 *     restart recreates the container; coordination state survives in Redis;
 *   - two instances at once (P17 / P20 / P21) — two projects, two ports;
 *   - per-test env injection — REDIS_URL / REDIS_KEY_PREFIX and any serverEnv are
 *     written to a generated env-file and loaded via docker-compose.test.yml.
 *
 * The container reaches the suite-level (host-published) Redis via
 * host.docker.internal, so REDIS_URL's loopback host is rewritten on the way in.
 *
 * The public surface (port, baseURL, pid, isRunning, start/stop/restart, dbPath,
 * logDir, stdout/stderrPath) is unchanged from the `next start` era, so
 * ProtocolEnv / CrossInstanceEnv and every spec are untouched. `pid` no longer
 * maps to a host process (the server is a container) and returns undefined.
 */

import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { randomUUID } from "node:crypto";
import fs from "node:fs";
import net from "node:net";
import path from "node:path";

import { suiteContext } from "./suite-root";

const execFileAsync = promisify(execFile);

const READINESS_POLL_MS = 250;
const BIND_RETRIES = 3;
const COMPOSE_TIMEOUT_MS = 120_000;
const LOG_MAX_BUFFER = 8 * 1024 * 1024;
const SERVICE = "harness";

const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

/** Compose project prefix for every per-instance harness container. */
const PROJECT_PREFIX = "harness-e2e-";

/**
 * Best-effort suite-level sweep of leaked per-instance harness containers +
 * networks (a test that crashed before ServerController.stop()). Uses raw docker
 * (filtered by the project-prefixed name) rather than `compose down`, which would
 * demand the per-instance HARNESS_PORT/HARNESS_ENV_FILE interpolation vars.
 * Called once from global-teardown; never throws.
 */
export async function sweepHarnessContainers(): Promise<number> {
  const runDocker = async (args: string[]): Promise<string> => {
    const { stdout } = await execFileAsync("docker", args, {
      timeout: 60_000,
      maxBuffer: LOG_MAX_BUFFER,
    });
    return stdout;
  };

  let removed = 0;
  try {
    const ids = (await runDocker(["ps", "-aq", "--filter", `name=${PROJECT_PREFIX}`]))
      .split("\n")
      .map((line) => line.trim())
      .filter(Boolean);
    if (ids.length > 0) {
      await runDocker(["rm", "-f", ...ids]);
      removed = ids.length;
    }

    const networks = (await runDocker(["network", "ls", "--filter", `name=${PROJECT_PREFIX}`, "-q"]))
      .split("\n")
      .map((line) => line.trim())
      .filter(Boolean);
    if (networks.length > 0) {
      await execFileAsync("docker", ["network", "rm", ...networks], { timeout: 30_000 }).catch(() => {
        // Networks still attached to a container we could not remove — ignore.
      });
    }
  } catch {
    // Best-effort hygiene; a failed sweep must never mask the suite verdict.
  }

  return removed;
}

/**
 * Asks the kernel for a free 127.0.0.1 port. A bind race (docker grabbing a
 * just-freed port, or another probe winning it) is handled by
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

/**
 * Rewrites a host-side Redis URL so an in-container process can reach it:
 * loopback (127.0.0.1 / localhost) becomes host.docker.internal, which
 * docker-compose.test.yml maps to the host gateway. The port and scheme are
 * preserved; a non-loopback host (already a service/host name) is left as-is.
 */
export function containerRedisUrl(url: string): string {
  return url.replace(/\/\/(?:127\.0\.0\.1|localhost)(?=[:/]|$)/, "//host.docker.internal");
}

export interface ServerControllerOptions {
  /**
   * Directory for the server's local scratch (logs adjacency + repro manifest).
   * The relay holds NO database; coordination state lives in Redis. Kept for the
   * test artifact layout and the reproduction manifest.
   */
  dbPath: string;
  /** Directory for server.stdout.log / server.stderr.log + the container env-file. */
  logDir: string;
  /** Pin a port; omitted → allocated on first start(). */
  port?: number;
  /** Readiness ceiling per start attempt. Default 60s (container boot > next start). */
  readinessTimeoutMs?: number;
  /**
   * Extra environment written into the container's env-file — additive over the
   * fixed REDIS_URL/REDIS_KEY_PREFIX. The degraded-inventory P9 case sets a tiny
   * MAX_INVENTORY_REQUEST_BYTES this way; p19 sets HARNESS_DEPLOYED=1.
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
  /** Unique compose project so instances coexist and never collide. */
  private readonly project: string;
  private readonly envFilePath: string;
  private running = false;
  private startCount = 0;

  constructor(options: ServerControllerOptions) {
    this.dbPath = options.dbPath;
    this.logDir = options.logDir;
    this.portValue = options.port;
    this.portPinned = options.port !== undefined;
    this.readinessTimeoutMs = options.readinessTimeoutMs ?? 60_000;
    this.extraEnv = options.env ?? {};
    this.project = `${PROJECT_PREFIX}${randomUUID().slice(0, 12)}`;
    this.stdoutPath = path.join(this.logDir, "server.stdout.log");
    this.stderrPath = path.join(this.logDir, "server.stderr.log");
    this.envFilePath = path.join(this.logDir, "container.env");
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

  /**
   * The server is a container, not a host process, so there is no host pid.
   * Kept for interface compatibility (repro-manifest omits it when undefined).
   */
  get pid(): number | undefined {
    return undefined;
  }

  isRunning(): boolean {
    return this.running;
  }

  /**
   * Starts the harness container and readiness-polls GET / until it responds.
   * On a bind failure with a NON-pinned port a fresh port is allocated and the
   * start retried (bind race); with a pinned port the same port is retried after
   * a short delay (restart scenarios where the old publish lingers).
   */
  async start(): Promise<void> {
    if (this.running) {
      throw new Error("server already running — call stop() or restart()");
    }

    this.writeEnvFile();

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
        `harness container failed on port ${this.portValue} (attempt ${attempt + 1}): ${outcome}`,
      );

      if (!this.portPinned) {
        this.portValue = undefined; // pick a fresh port next attempt
      } else {
        await sleep(1000);
      }
    }

    throw lastError ?? new Error("server failed to start");
  }

  /** `docker compose down` the instance's project (removes container + network). */
  async stop(): Promise<void> {
    if (!this.running && this.portValue === undefined) {
      return;
    }

    await this.captureLogs();
    await this.down();
  }

  /** stop() + start() against the SAME port and env-file (P7/A8/P18). */
  async restart(): Promise<void> {
    const port = this.port; // throws if never started — restart implies a prior start
    await this.stop();
    this.portValue = port;
    await this.start();
  }

  private composeFile(): string {
    return path.join(suiteContext().repoRoot, "docker-compose.test.yml");
  }

  /** Env passed to the `docker compose` child so the override interpolates. */
  private composeProcessEnv(port: number): NodeJS.ProcessEnv {
    return {
      ...process.env,
      HARNESS_PORT: String(port),
      HARNESS_ENV_FILE: this.envFilePath,
    };
  }

  /** Writes the container env-file (only set keys → no empty-string ambiguity). */
  private writeEnvFile(): void {
    const env: Record<string, string> = { ...this.extraEnv };
    const rawRedis = env.REDIS_URL ?? "redis://127.0.0.1:6379";
    env.REDIS_URL = containerRedisUrl(rawRedis);

    const body = `${Object.entries(env)
      .map(([key, value]) => `${key}=${value}`)
      .join("\n")}\n`;
    fs.writeFileSync(this.envFilePath, body);
  }

  private async startOnce(port: number): Promise<"ready" | string> {
    this.startCount += 1;
    const banner = `\n--- start #${this.startCount} project=${this.project} port=${port} ---\n`;
    fs.appendFileSync(this.stdoutPath, banner);
    fs.appendFileSync(this.stderrPath, banner);

    try {
      await execFileAsync(
        "docker",
        ["compose", "-f", this.composeFile(), "-p", this.project, "up", "-d", "--force-recreate", SERVICE],
        { cwd: suiteContext().repoRoot, env: this.composeProcessEnv(port), timeout: COMPOSE_TIMEOUT_MS },
      );
    } catch (error) {
      const err = error as { stderr?: string; message?: string };
      await this.down();
      return `compose up failed: ${(err.stderr ?? err.message ?? "").trim()}`;
    }

    this.running = true;

    const deadline = Date.now() + this.readinessTimeoutMs;
    while (Date.now() < deadline) {
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

    await this.captureLogs();
    await this.down();

    return `readiness timeout after ${this.readinessTimeoutMs}ms (see ${this.stderrPath})`;
  }

  /** Appends the container's logs to the server log files (best-effort). */
  private async captureLogs(): Promise<void> {
    const port = this.portValue ?? 0;
    try {
      const { stdout, stderr } = await execFileAsync(
        "docker",
        ["compose", "-f", this.composeFile(), "-p", this.project, "logs", "--no-color", "--no-log-prefix", SERVICE],
        { cwd: suiteContext().repoRoot, env: this.composeProcessEnv(port), maxBuffer: LOG_MAX_BUFFER },
      );
      if (stdout) {
        fs.appendFileSync(this.stdoutPath, stdout);
      }
      fs.appendFileSync(this.stderrPath, stderr || stdout || "");
    } catch {
      // Logs are diagnostic only — never let their capture mask a verdict.
    }
  }

  private async down(): Promise<void> {
    const port = this.portValue ?? 0;
    const env = this.composeProcessEnv(port);
    const repoRoot = suiteContext().repoRoot;

    // Abrupt SIGKILL FIRST, so a restart models a real relay-gone event (crash /
    // scale-down / cold start) rather than a graceful drain. Next standalone's
    // SIGTERM handler would close the server and let an in-flight dispatch's
    // client-abort cleanup run — which LREMs the queued work out of Redis
    // (redis-relay `cancel`). Killing the process first means that cleanup never
    // runs, so durable coordination state survives the restart (P18) exactly as
    // it did under the old `next start` replacement.
    try {
      await execFileAsync(
        "docker",
        ["compose", "-f", this.composeFile(), "-p", this.project, "kill", SERVICE],
        { cwd: repoRoot, env, timeout: COMPOSE_TIMEOUT_MS },
      );
    } catch {
      // Not running (e.g. failed start) — nothing to kill.
    }

    try {
      await execFileAsync(
        "docker",
        ["compose", "-f", this.composeFile(), "-p", this.project, "down", "--remove-orphans"],
        { cwd: repoRoot, env, timeout: COMPOSE_TIMEOUT_MS },
      );
    } catch {
      // Best-effort teardown; global-teardown sweeps any leftover projects.
    }
    this.running = false;
  }
}
