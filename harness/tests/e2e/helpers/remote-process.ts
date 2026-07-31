/**
 * Remote-process helper (plan §4.1, §4.4).
 *
 * Spawns `true-bdd remote --server <url>` with cwd = the materialized
 * fixture folder, detached into its own process group, and — BY
 * DEFAULT — with CLAUDECODE='playwright-sentinel'. Production
 * behavior is that the REMOTE strips CLAUDECODE for its children; a
 * child's successful real-Claude call under the sentinel PROVES the
 * stripping (plan §4.4). Only the claude gate probe strips the var at
 * the helper level.
 *
 * Records pid, pgid, and process-start identity (so a recycled pid is
 * never signalled by mistake), reads the remote's
 * `<folder>/tmp/true-bdd-remote-children.pids` file when present, and
 * offers killTree (TERM → KILL across the remote's and children's
 * process groups). Every spawn is also recorded in the suite-level
 * process registry for global-teardown's emergency cleanup.
 */

import { spawn, type ChildProcess } from "node:child_process";
import fs from "node:fs";
import path from "node:path";

import { registerProcess } from "./process-registry";
import { maySignalIdentity, processStartIdentity } from "./start-identity";
import { suiteContext } from "./suite-root";

// Re-exported so existing importers (protocol-env) keep their import path
// while the implementation lives in the shared, dependency-free module.
export { processStartIdentity } from "./start-identity";

/** Sentinel proving production CLAUDECODE-stripping (plan §4.4). */
export const CLAUDECODE_SENTINEL = "playwright-sentinel";

const KILL_GRACE_MS = 5000;

const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

export interface RemoteProcessOptions {
  /** Harness server URL the remote connects OUT to. */
  serverUrl: string;
  /** Host-project folder (materialized fixture target). */
  cwd: string;
  /** Directory for remote.stdout.log / remote.stderr.log. */
  logDir: string;
  /** Binary override; defaults to the suite-built `true-bdd`. */
  binPath?: string;
  /**
   * CLAUDECODE value for the remote's environment. Defaults to the
   * sentinel. Pass null to strip the variable entirely (not the
   * production-proof path — prefer the default).
   */
  claudecode?: string | null;
  /** Extra environment entries merged over the defaults. */
  env?: Record<string, string>;
}

export interface ChildPidEntry {
  /** Child process-group id (the signal target). */
  pgid: number;
  /** Process-start identity, when the file records one. */
  startIdentity?: string;
  /** Run id, when the file records one. */
  runId?: string;
  /** The raw line, for debugging / repro manifests. */
  raw: string;
}

export interface ExitStatus {
  code: number | null;
  signal: NodeJS.Signals | null;
}

export class RemoteProcess {
  readonly pid: number;
  readonly pgid: number;
  /** Captured at spawn; compare with processStartIdentity(pid). */
  readonly startIdentity: string | undefined;
  readonly cwd: string;
  readonly stdoutPath: string;
  readonly stderrPath: string;

  private readonly child: ChildProcess;
  private readonly exitPromise: Promise<ExitStatus>;

  private constructor(child: ChildProcess, options: RemoteProcessOptions) {
    if (child.pid === undefined) {
      throw new Error("remote spawn returned no pid");
    }

    this.child = child;
    this.pid = child.pid;
    this.pgid = child.pid; // detached ⇒ session/group leader
    this.cwd = options.cwd;
    this.stdoutPath = path.join(options.logDir, "remote.stdout.log");
    this.stderrPath = path.join(options.logDir, "remote.stderr.log");
    this.startIdentity = processStartIdentity(this.pid);
    this.exitPromise = new Promise<ExitStatus>((resolve) => {
      child.once("exit", (code, signal) => resolve({ code, signal }));
    });
  }

  static async start(options: RemoteProcessOptions): Promise<RemoteProcess> {
    const binPath = options.binPath ?? suiteContext().binPath;
    fs.mkdirSync(options.logDir, { recursive: true });

    const stdoutPath = path.join(options.logDir, "remote.stdout.log");
    const stderrPath = path.join(options.logDir, "remote.stderr.log");
    const stdoutFd = fs.openSync(stdoutPath, "a");
    const stderrFd = fs.openSync(stderrPath, "a");

    const env: NodeJS.ProcessEnv = {
      ...process.env,
      ...options.env,
    };

    const claudecode = options.claudecode === undefined ? CLAUDECODE_SENTINEL : options.claudecode;
    if (claudecode === null) {
      delete env.CLAUDECODE;
    } else {
      env.CLAUDECODE = claudecode;
    }

    const child = spawn(binPath, ["remote", "--server", options.serverUrl], {
      cwd: options.cwd,
      detached: true, // own process group (setpgid)
      stdio: ["ignore", stdoutFd, stderrFd],
      env,
    });

    fs.closeSync(stdoutFd);
    fs.closeSync(stderrFd);

    const remote = new RemoteProcess(child, options);
    registerProcess({
      pid: remote.pid,
      pgid: remote.pgid,
      label: `remote:${path.basename(options.cwd)}`,
      startedAt: new Date().toISOString(),
      // Record the OS start identity so the suite-level emergency cleanup
      // can prove this pid was not recycled before signalling (finding 9).
      startIdentity: remote.startIdentity,
    });

    return remote;
  }

  isRunning(): boolean {
    return this.child.exitCode === null && this.child.signalCode === null;
  }

  /** Signal the remote PROCESS only (SIGSTOP/SIGCONT/SIGKILL for P2/A7/A8). */
  signal(sig: NodeJS.Signals): void {
    process.kill(this.pid, sig);
  }

  /** Signal the remote's whole PROCESS GROUP (Ctrl+C semantics — A9). */
  signalGroup(sig: NodeJS.Signals): void {
    process.kill(-this.pgid, sig);
  }

  /** Resolves with the exit status; rejects on timeout. */
  async waitForExit(timeoutMs: number): Promise<ExitStatus> {
    const result = await Promise.race([
      this.exitPromise,
      sleep(timeoutMs).then(() => undefined),
    ]);

    if (result === undefined) {
      throw new Error(`remote (pid ${this.pid}) did not exit within ${timeoutMs}ms`);
    }

    return result;
  }

  /**
   * Parses the remote's children pids file(s) under `<cwd>/tmp/`.
   *
   * v2 (plan §1.6): child pid files become PER-OWNER
   * `true-bdd-remote-children.<owner_id>.jsonl` (fixing the same-folder
   * clobbering bug where two remotes rewrote one shared file). This parser
   * globs every such per-owner file AND still reads the legacy single
   * `true-bdd-remote-children.pids`, so it keeps working across the
   * migration. Each entry is a JSONL object (PGID + process-start identity
   * + run id); a whitespace-separated fallback keeps older formats parsing.
   */
  readChildrenPids(): ChildPidEntry[] {
    const tmpDir = path.join(this.cwd, "tmp");

    const files: string[] = [path.join(tmpDir, "true-bdd-remote-children.pids")];
    try {
      for (const name of fs.readdirSync(tmpDir)) {
        if (name.startsWith("true-bdd-remote-children.") && name.endsWith(".jsonl")) {
          files.push(path.join(tmpDir, name));
        }
      }
    } catch {
      // tmp/ may not exist yet.
    }

    const entries: ChildPidEntry[] = [];
    for (const file of files) {
      let raw: string;
      try {
        raw = fs.readFileSync(file, "utf8");
      } catch {
        continue;
      }

      for (const line of raw.split("\n")) {
        const trimmed = line.trim();
        if (!trimmed) continue;

        const parsed = parsePidLine(trimmed);
        if (parsed !== undefined) {
          entries.push(parsed);
        }
      }
    }

    return entries;
  }

  /**
   * TERM → bounded wait → KILL across the remote's own group and every
   * child group listed in the pids file — but ONLY groups whose leader
   * still bears its recorded start identity (finding 9): killTree now USES
   * the identities it parses. The remote's own group is validated against
   * the identity captured at spawn; each child group against the identity
   * recorded in the pids file. A recycled/unverifiable pid is skipped, not
   * signalled. Safe to call twice.
   */
  async killTree(): Promise<void> {
    const candidates: { pgid: number; recorded: string | undefined; identifyPid: number }[] = [
      { pgid: this.pgid, recorded: this.startIdentity, identifyPid: this.pid },
      ...this.readChildrenPids().map((entry) => ({
        pgid: entry.pgid,
        recorded: entry.startIdentity,
        identifyPid: entry.pgid,
      })),
    ];

    const seen = new Set<number>();
    const targets: number[] = [];
    for (const candidate of candidates) {
      if (seen.has(candidate.pgid)) {
        continue;
      }
      seen.add(candidate.pgid);

      // When the pids file recorded NO identity for a child (older format),
      // fall back to the prior permissive behavior for that child only; the
      // remote's own group and identity-bearing children are strict.
      if (candidate.recorded === undefined) {
        targets.push(candidate.pgid);

        continue;
      }

      if (maySignalIdentity(candidate.recorded, processStartIdentity(candidate.identifyPid))) {
        targets.push(candidate.pgid);
      }
    }

    for (const pgid of targets) {
      trySignalGroup(pgid, "SIGTERM");
    }

    const deadline = Date.now() + KILL_GRACE_MS;
    while (Date.now() < deadline && targets.some(groupAlive)) {
      await sleep(200);
    }

    for (const pgid of targets.filter(groupAlive)) {
      trySignalGroup(pgid, "SIGKILL");
    }
  }
}

function parsePidLine(line: string): ChildPidEntry | undefined {
  if (line.startsWith("{")) {
    try {
      const doc = JSON.parse(line) as Record<string, unknown>;
      const pgid = Number(doc.pgid ?? doc.PGID);
      if (!Number.isInteger(pgid) || pgid <= 0) return undefined;

      return {
        pgid,
        startIdentity: stringOrUndefined(doc.start_identity ?? doc.startIdentity),
        runId: stringOrUndefined(doc.run_id ?? doc.runId),
        raw: line,
      };
    } catch {
      return undefined;
    }
  }

  const first = line.split(/\s+/, 1)[0];
  const pgid = Number(first);
  if (!Number.isInteger(pgid) || pgid <= 0) return undefined;

  return { pgid, raw: line };
}

function stringOrUndefined(value: unknown): string | undefined {
  return typeof value === "string" && value !== "" ? value : undefined;
}

function groupAlive(pgid: number): boolean {
  try {
    process.kill(-pgid, 0);
    return true;
  } catch {
    return false;
  }
}

function trySignalGroup(pgid: number, sig: NodeJS.Signals): void {
  try {
    process.kill(-pgid, sig);
  } catch {
    // ESRCH — already gone.
  }
}
