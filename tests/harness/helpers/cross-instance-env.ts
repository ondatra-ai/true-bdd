/**
 * Two-`next start`-instance environment for the serverless-rendezvous specs
 * (plan: connect-cli-to-vercel-harness → p17/p20/p21/p22).
 *
 * The core serverless proof: browser reads/mutations pinned to instance A and
 * CLI poll/reply pinned to instance B — TWO separate `next start` processes —
 * rendezvous through ONE shared Redis (same `REDIS_KEY_PREFIX`). Under the
 * in-process (globalThis) relay this is IMPOSSIBLE: each process has its own
 * registry/work queue, so any cross-instance operation fails. That failure is
 * the intended RED until the coder lands the Redis-backed relay.
 *
 * Mirrors ProtocolEnv's teardown discipline (remotes + child groups first,
 * fixture teardown, scoped Redis flush, both servers, repro manifest) but owns
 * TWO servers + TWO API contexts. Both servers get the SAME `REDIS_KEY_PREFIX`
 * (and `REDIS_URL`); ports are allocated (reserved) before spawn to avoid bind
 * races (ServerController.allocatePort + retry).
 */

import path from "node:path";

import type { TestInfo } from "@playwright/test";

import { HarnessApi } from "./api-client";
import { materializeFixture, runFixtureTeardown, type MaterializedFixture } from "./materializer";
import { processStartIdentity, RemoteProcess } from "./remote-process";
import { REDIS_URL, uniquePrefix, flushPrefix } from "./redis";
import { writeReproManifestOnFailure, type ReproManifestInput } from "./repro-manifest";
import { ServerController } from "./server-controller";
import { mkdirUnderSuiteRoot } from "./suite-root";

const CHILD_KILL_GRACE_MS = 2000;
const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

export interface StartRemoteOnBOptions {
  /** Extra env for the remote process. */
  env?: Record<string, string>;
  /** CLAUDECODE override; defaults to the sentinel. */
  claudecode?: string | null;
}

export class CrossInstanceEnv {
  readonly dir: string;
  readonly serverA: ServerController;
  readonly serverB: ServerController;
  readonly apiA: HarnessApi;
  readonly apiB: HarnessApi;
  /** Shared by A and B so they rendezvous in one Redis namespace. */
  readonly prefix: string;
  readonly remotes: RemoteProcess[] = [];
  readonly fixtures: MaterializedFixture[] = [];

  private readonly notes: ReproManifestInput = {};
  private fixtureCount = 0;
  private remoteCount = 0;

  private constructor(
    dir: string,
    serverA: ServerController,
    serverB: ServerController,
    apiA: HarnessApi,
    apiB: HarnessApi,
    prefix: string,
  ) {
    this.dir = dir;
    this.serverA = serverA;
    this.serverB = serverB;
    this.apiA = apiA;
    this.apiB = apiB;
    this.prefix = prefix;
  }

  /** Assigned after start(). */
  get portA(): number {
    return this.serverA.port;
  }

  get portB(): number {
    return this.serverB.port;
  }

  /**
   * Starts TWO `next start` instances sharing one Redis namespace (unique
   * prefix). `serverEnv` is applied to BOTH (e.g. HARNESS_DEPLOYED for p19-
   * style cases — not used by the cross-instance specs today, but available).
   */
  static async start(slug: string, options: { serverEnv?: Record<string, string> } = {}): Promise<CrossInstanceEnv> {
    const dir = mkdirUnderSuiteRoot(slug);
    const prefix = uniquePrefix(slug);
    const sharedEnv = { REDIS_URL, REDIS_KEY_PREFIX: prefix, ...options.serverEnv };

    const serverA = new ServerController({
      dbPath: path.join(dir, "db", "a-harness.db"),
      logDir: path.join(dir, "serverA"),
      env: sharedEnv,
    });
    const serverB = new ServerController({
      dbPath: path.join(dir, "db", "b-harness.db"),
      logDir: path.join(dir, "serverB"),
      env: sharedEnv,
    });

    // Start sequentially so bind-race retries don't race each other.
    await serverA.start();
    await serverB.start();

    const apiA = await HarnessApi.create(serverA.baseURL);
    const apiB = await HarnessApi.create(serverB.baseURL);

    return new CrossInstanceEnv(dir, serverA, serverB, apiA, apiB, prefix);
  }

  /** Materializes a fixture into this test's scope. */
  async materialize(name: string): Promise<MaterializedFixture> {
    this.fixtureCount += 1;
    const target = path.join(this.dir, `fixture-${this.fixtureCount}`, "host");
    const fixture = await materializeFixture(name, target);
    this.fixtures.push(fixture);

    this.notes.fixture = this.notes.fixture === undefined ? name : `${this.notes.fixture}, ${name}`;
    if (this.fixtures.length === 1) {
      this.notes.baseline = fixture.baseline;
      this.notes.treeRoot = fixture.target;
    }

    return fixture;
  }

  /** Spawns a remote pointing at server B (the CLI-side instance). */
  async startRemoteOnB(cwd: string, options: StartRemoteOnBOptions = {}): Promise<RemoteProcess> {
    this.remoteCount += 1;
    const remote = await RemoteProcess.start({
      serverUrl: this.serverB.baseURL,
      cwd,
      logDir: path.join(this.dir, `remoteB-${this.remoteCount}`),
      env: options.env,
      claudecode: options.claudecode,
    });
    this.remotes.push(remote);

    return remote;
  }

  note(fields: Partial<ReproManifestInput>): void {
    Object.assign(this.notes, fields);
  }

  async teardown(testInfo: TestInfo): Promise<void> {
    // 1. Remotes + their child groups (identity-validated), same as ProtocolEnv.
    for (const remote of this.remotes) {
      try {
        await stopRemote(remote);
      } catch (error) {
        console.warn(`[cross-instance-env] remote cleanup failed (pid ${remote.pid}): ${String(error)}`);
      }
    }

    // 2. Fixture-declared teardown (best-effort).
    for (const fixture of this.fixtures) {
      for (const failure of await runFixtureTeardown(fixture)) {
        console.warn(`[cross-instance-env] ${failure}`);
      }
    }

    // 3. Both servers, then both API contexts.
    for (const server of [this.serverA, this.serverB]) {
      try {
        await server.stop();
      } catch (error) {
        console.warn(`[cross-instance-env] server stop failed: ${String(error)}`);
      }
    }
    for (const api of [this.apiA, this.apiB]) {
      try {
        await api.dispose();
      } catch {
        // Disposing a dead context is uninteresting.
      }
    }

    // 4. Scoped Redis cleanup (best-effort).
    try {
      await flushPrefix(this.prefix);
    } catch (error) {
      console.warn(`[cross-instance-env] redis flushPrefix failed: ${String(error)}`);
    }

    // 5. Reproduction manifest — only on unexpected outcome.
    await writeReproManifestOnFailure(testInfo, {
      ...this.notes,
      port: tryPort(this.serverA),
      dbPath: this.serverA.dbPath,
      pgids: [
        ...this.remotes.map((remote) => remote.pgid),
        ...(this.serverA.pid === undefined ? [] : [this.serverA.pid]),
        ...(this.serverB.pid === undefined ? [] : [this.serverB.pid]),
      ],
      logPaths: [
        this.serverA.stdoutPath,
        this.serverA.stderrPath,
        this.serverB.stdoutPath,
        this.serverB.stderrPath,
        ...this.remotes.flatMap((remote) => [remote.stdoutPath, remote.stderrPath]),
      ],
    });
  }
}

// ── remote stop helper (mirrors protocol-env's stopRemote) ──

async function stopRemote(remote: RemoteProcess): Promise<void> {
  const identityNow = processStartIdentity(remote.pid);
  const identityValid =
    remote.startIdentity === undefined || identityNow === undefined || identityNow === remote.startIdentity;

  if (remote.isRunning() && identityValid) {
    try {
      remote.signalGroup("SIGCONT");
    } catch {
      // Group already gone.
    }
    await remote.killTree();
    return;
  }

  const entries = remote.readChildrenPids().filter((entry) => {
    if (entry.startIdentity === undefined) return true;
    const now = processStartIdentity(entry.pgid);
    return now === undefined || now === entry.startIdentity;
  });

  for (const entry of entries) {
    trySignalGroup(entry.pgid, "SIGCONT");
    trySignalGroup(entry.pgid, "SIGTERM");
  }
  const deadline = Date.now() + CHILD_KILL_GRACE_MS;
  while (Date.now() < deadline && entries.some((entry) => groupAlive(entry.pgid))) {
    await sleep(100);
  }
  for (const entry of entries.filter((entry) => groupAlive(entry.pgid))) {
    trySignalGroup(entry.pgid, "SIGKILL");
  }
}

function tryPort(server: ServerController): number | undefined {
  try {
    return server.port;
  } catch {
    return undefined;
  }
}

function groupAlive(pgid: number): boolean {
  try {
    process.kill(-pgid, 0);
    return true;
  } catch {
    return false;
  }
}

function trySignalGroup(pgid: number, signal: NodeJS.Signals): void {
  try {
    process.kill(-pgid, signal);
  } catch {
    // ESRCH — already gone.
  }
}
