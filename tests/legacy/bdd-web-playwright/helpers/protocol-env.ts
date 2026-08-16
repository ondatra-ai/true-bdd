/**
 * Per-test environment scaffold for the protocol specs (plan §4.1):
 * every test owns its OWN server (own DB file + port + logs), its own
 * materialized fixture copies, and its own remotes, all rooted in a
 * test-scoped directory under the suite root.
 *
 * Teardown order follows plan §4.1 exactly: remotes and their child
 * process groups (identity-validated — a recycled pid is never
 * signalled), then fixture-declared teardown commands, then the
 * server; finally the reproduction manifest is written when the test
 * did not end as expected. Every stage is fenced so one failure never
 * masks the others or the manifest.
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

export interface StartRemoteOptions {
  /** Extra environment entries (e.g. PWD for the symlink case). */
  env?: Record<string, string>;
  /** CLAUDECODE override; defaults to the sentinel (see RemoteProcess). */
  claudecode?: string | null;
}

export class ProtocolEnv {
  readonly dir: string;
  readonly server: ServerController;
  readonly api: HarnessApi;
  readonly remotes: RemoteProcess[] = [];
  readonly fixtures: MaterializedFixture[] = [];
  /** Unique Redis key prefix shared by this env's server (+ its remotes). */
  readonly prefix: string;

  private readonly notes: ReproManifestInput = {};
  private fixtureCount = 0;
  private remoteCount = 0;

  private constructor(dir: string, server: ServerController, api: HarnessApi, prefix: string) {
    this.dir = dir;
    this.server = server;
    this.api = api;
    this.prefix = prefix;
  }

  /**
   * Starts a fresh server (own port) in a test-scoped dir, injected with a
   * unique `REDIS_KEY_PREFIX` + `REDIS_URL` so the Redis-backed relay isolates
   * this test from sequential siblings (plan: connect-cli-to-vercel-harness).
   * Pass `serverEnv` to inject extra environment into the server process —
   * the degraded-inventory P9 case sets a tiny MAX_INVENTORY_REQUEST_BYTES
   * to exercise the remote's fit-to-budget path (plan §1a/§4.3); p19 sets
   * HARNESS_DEPLOYED=1 for the deployed-origin-policy server.
   */
  static async start(slug: string, options: { serverEnv?: Record<string, string> } = {}): Promise<ProtocolEnv> {
    const dir = mkdirUnderSuiteRoot(slug);
    const prefix = uniquePrefix(slug);
    const server = new ServerController({
      dbPath: path.join(dir, "db", "harness.db"),
      logDir: path.join(dir, "server"),
      env: {
        REDIS_URL,
        REDIS_KEY_PREFIX: prefix,
        ...options.serverEnv,
      },
    });
    await server.start();
    const api = await HarnessApi.create(server.baseURL);

    return new ProtocolEnv(dir, server, api, prefix);
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

  /** Spawns a remote against this test's server. */
  async startRemote(cwd: string, options: StartRemoteOptions = {}): Promise<RemoteProcess> {
    this.remoteCount += 1;
    const remote = await RemoteProcess.start({
      serverUrl: this.server.baseURL,
      cwd,
      logDir: path.join(this.dir, `remote-${this.remoteCount}`),
      env: options.env,
      claudecode: options.claudecode,
    });
    this.remotes.push(remote);

    return remote;
  }

  /** Records ids for the reproduction manifest (session/run/extras). */
  note(fields: Partial<ReproManifestInput>): void {
    Object.assign(this.notes, fields);
  }

  async teardown(testInfo: TestInfo): Promise<void> {
    const serverPid = this.server.pid;

    // 1. Remotes + their child groups (identity-validated).
    for (const remote of this.remotes) {
      try {
        await stopRemote(remote);
      } catch (error) {
        console.warn(`[protocol-env] remote cleanup failed (pid ${remote.pid}): ${String(error)}`);
      }
    }

    // 2. Fixture-declared teardown commands (best-effort by design).
    for (const fixture of this.fixtures) {
      const failures = await runFixtureTeardown(fixture);
      for (const failure of failures) {
        console.warn(`[protocol-env] ${failure}`);
      }
    }

    // 3. The server, then the API context.
    try {
      await this.server.stop();
    } catch (error) {
      console.warn(`[protocol-env] server stop failed: ${String(error)}`);
    }

    try {
      await this.api.dispose();
    } catch {
      // Disposing a dead context is uninteresting.
    }

    // 4. Scoped Redis cleanup — delete only this test's prefix (plan r2 #10).
    //    Best-effort: prefix uniqueness already isolates sequential tests.
    try {
      await flushPrefix(this.prefix);
    } catch (error) {
      console.warn(`[protocol-env] redis flushPrefix failed: ${String(error)}`);
    }

    // 5. Reproduction manifest — only when the test did not end as expected.
    await writeReproManifestOnFailure(testInfo, {
      ...this.notes,
      port: tryPort(this.server),
      dbPath: this.server.dbPath,
      pgids: [...this.remotes.map((remote) => remote.pgid), ...(serverPid === undefined ? [] : [serverPid])],
      logPaths: [
        this.server.stdoutPath,
        this.server.stderrPath,
        ...this.remotes.flatMap((remote) => [remote.stdoutPath, remote.stderrPath]),
      ],
    });
  }
}

/**
 * Kills one remote and its recorded child groups. The remote's own
 * group is signalled only while our ChildProcess handle says it is
 * alive AND its process-start identity still matches the one captured
 * at spawn — a recycled pid is never signalled. Recorded child groups
 * are validated the same way when the pids file carries an identity.
 */
async function stopRemote(remote: RemoteProcess): Promise<void> {
  const identityNow = processStartIdentity(remote.pid);
  const identityValid =
    remote.startIdentity === undefined || identityNow === undefined || identityNow === remote.startIdentity;

  if (remote.isRunning() && identityValid) {
    // SIGSTOPped remotes (P2/P5) would otherwise sit on the pending
    // SIGTERM until killTree escalates; wake the group first.
    try {
      remote.signalGroup("SIGCONT");
    } catch {
      // Group already gone.
    }

    await remote.killTree();

    return;
  }

  // The remote itself is gone (or unverifiable): clean only its
  // recorded child groups, each validated by start identity when the
  // pids file recorded one.
  const entries = remote.readChildrenPids().filter((entry) => {
    if (entry.startIdentity === undefined) {
      return true;
    }

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

  for (const entry of entries.filter((candidate) => groupAlive(candidate.pgid))) {
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
