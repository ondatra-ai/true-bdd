/**
 * Process-start identity (plan §4.1 / finding 9). A PID (and thus a PGID)
 * can be RECYCLED by an unrelated process once the original exits. Before
 * any teardown signals a recorded process group, it must prove the group
 * leader STILL bears the OS start identity captured when it was registered
 * — otherwise a long AI suite that outlived a PID could TERM/KILL a
 * stranger. This module is dependency-free so the decision predicate is
 * unit-testable without spawning anything.
 */

import { execFileSync } from "node:child_process";

/**
 * `ps -o lstart= -p <pid>` — the process's start timestamp, stable for its
 * life, so a recycled pid (a DIFFERENT process reusing the number) reports
 * a different value. Undefined when ps has no such live pid.
 */
export function processStartIdentity(pid: number): string | undefined {
  try {
    const out = execFileSync("ps", ["-o", "lstart=", "-p", String(pid)], {
      encoding: "utf8",
    }).trim();

    return out === "" ? undefined : out;
  } catch {
    return undefined;
  }
}

/**
 * Whether a recorded process group MAY be signalled: only when the leader
 * pid still bears the recorded start identity. A missing recorded identity,
 * a live pid ps cannot identify, or a MISMATCH (the pid was recycled) all
 * return false — the caller must skip and warn, never signal.
 */
export function maySignalIdentity(recorded: string | undefined, live: string | undefined): boolean {
  if (recorded === undefined || recorded === "") {
    return false;
  }
  if (live === undefined || live === "") {
    return false;
  }

  return recorded === live;
}
