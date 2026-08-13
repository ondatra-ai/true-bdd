/**
 * Suite-level process registry (plan §4.1): every helper that spawns a
 * long-lived process (server, remote) appends its process-group id
 * here; global-teardown performs a final emergency cleanup so a
 * timed-out or crashed test can never leak a `next start` server or a
 * remote (and its claude descendants) past the suite.
 *
 * Test-scoped teardown remains the primary cleanup path — this
 * registry is the last line of defense.
 */

import fs from "node:fs";
import path from "node:path";

import { maySignalIdentity, processStartIdentity } from "./start-identity";
import { suiteContext } from "./suite-root";

export interface RegisteredProcess {
  pid: number;
  /** Process-group id to signal (negative-pid kill target). */
  pgid: number;
  /** Human-readable origin, e.g. "server:4519" or "remote:p2-fixture". */
  label: string;
  startedAt: string;
  /**
   * The OS process-start identity of the group leader at registration
   * (plan §4.1 / finding 9). A recorded identity that no longer matches the
   * live pid means the pid was recycled — the group must NOT be signalled.
   */
  startIdentity?: string;
}

export function registryPath(): string {
  return path.join(suiteContext().suiteRoot, "process-registry.jsonl");
}

/** Appends one entry (JSONL, append-only — safe across workers). */
export function registerProcess(entry: RegisteredProcess): void {
  fs.appendFileSync(registryPath(), `${JSON.stringify(entry)}\n`);
}

export function readRegistry(): RegisteredProcess[] {
  let raw: string;
  try {
    raw = fs.readFileSync(registryPath(), "utf8");
  } catch {
    return [];
  }

  const entries: RegisteredProcess[] = [];
  for (const line of raw.split("\n")) {
    const trimmed = line.trim();
    if (!trimmed) continue;

    try {
      entries.push(JSON.parse(trimmed) as RegisteredProcess);
    } catch {
      // A torn write from a killed process — skip, never crash cleanup.
    }
  }

  return entries;
}

/** True when the process group still has at least one live member. */
export function groupAlive(pgid: number): boolean {
  try {
    process.kill(-pgid, 0);
    return true;
  } catch {
    return false;
  }
}

function signalGroup(pgid: number, signal: NodeJS.Signals): void {
  try {
    process.kill(-pgid, signal);
  } catch {
    // ESRCH — already gone. Exactly what we want.
  }
}

const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

/**
 * TERM → bounded wait → KILL every registered process group that is still
 * alive AND still bears its recorded process-start identity (plan §4.1 /
 * finding 9). A group whose leader pid was recycled — a different or
 * missing start identity — is SKIPPED and warned, never signalled, so a
 * long AI suite can never kill an unrelated host process. Returns the
 * pgids that were actually signalled.
 */
export async function emergencyKillAll(gracePeriodMs = 5000): Promise<number[]> {
  const byPgid = new Map<number, RegisteredProcess>();
  for (const entry of readRegistry()) {
    if (!byPgid.has(entry.pgid)) {
      byPgid.set(entry.pgid, entry);
    }
  }

  const stale: number[] = [];
  for (const entry of byPgid.values()) {
    if (!groupAlive(entry.pgid)) {
      continue;
    }

    if (!maySignalIdentity(entry.startIdentity, processStartIdentity(entry.pid))) {
      console.warn(
        `[emergency-cleanup] skipping pgid ${entry.pgid} (${entry.label}): start identity mismatch or missing — pid ${entry.pid} may have been recycled`,
      );

      continue;
    }

    stale.push(entry.pgid);
  }

  for (const pgid of stale) {
    signalGroup(pgid, "SIGTERM");
  }

  const deadline = Date.now() + gracePeriodMs;
  while (Date.now() < deadline && stale.some(groupAlive)) {
    await sleep(200);
  }

  for (const pgid of stale.filter(groupAlive)) {
    signalGroup(pgid, "SIGKILL");
  }

  return stale;
}
