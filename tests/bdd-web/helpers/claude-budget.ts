/**
 * AI-call budget + claude-descendant tracking (plan §4.3 / §4.4).
 *
 * Every A-test spawns commands through a `true-bdd remote`; the command
 * child spawns `claude` as ITS child (the remote strips CLAUDECODE for
 * that subtree — plan §3.2/§4.4). This helper walks the process tree via
 * `ps` and:
 *
 *   - counts the DISTINCT `claude` processes that appear under a remote
 *     during a run (ClaudeCallBudget) — a "single-check" fixture that
 *     spawns unexpectedly many exposes a broken checklist filter that is
 *     silently walking the whole checklist (one evaluation call per
 *     prompt) before it becomes a multi-hour run;
 *   - waits for a live `claude` descendant to exist (A9 SIGINTs only
 *     AFTER a real Claude call is in flight);
 *   - waits for the subtree to be free of `claude` descendants (A9
 *     asserts no survivors after the interrupt).
 *
 * `claude` is identified by its command string containing "claude"; the
 * `true-bdd` remote/command binaries never do (the suite bin lives under
 * a per-invocation `harness-e2e-<id>/bin/true-bdd`). Descendant filtering
 * also keeps the ambient Claude Code session that may be running THIS
 * suite out of the count — it is never a child of a test's remote.
 */

import { execFileSync } from "node:child_process";

const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

export interface ProcInfo {
  pid: number;
  ppid: number;
  command: string;
}

/**
 * One `ps` snapshot of every process: pid, ppid, full command. macOS and
 * Linux both honour `-ax -o pid=,ppid=,command=`.
 */
export function psSnapshot(): ProcInfo[] {
  let raw: string;
  try {
    raw = execFileSync("ps", ["-ax", "-o", "pid=,ppid=,command="], {
      encoding: "utf8",
      maxBuffer: 16 * 1024 * 1024,
    });
  } catch {
    return [];
  }

  const procs: ProcInfo[] = [];
  for (const line of raw.split("\n")) {
    const trimmed = line.trim();
    if (trimmed === "") continue;

    const match = trimmed.match(/^(\d+)\s+(\d+)\s+(.*)$/);
    if (match === null) continue;

    procs.push({ pid: Number(match[1]), ppid: Number(match[2]), command: match[3] });
  }

  return procs;
}

/** Pids in `snapshot` transitively descended from `rootPid` (excl. root). */
export function descendantPids(snapshot: ProcInfo[], rootPid: number): Set<number> {
  const childrenByParent = new Map<number, number[]>();
  for (const proc of snapshot) {
    const list = childrenByParent.get(proc.ppid) ?? [];
    list.push(proc.pid);
    childrenByParent.set(proc.ppid, list);
  }

  const out = new Set<number>();
  const stack = [...(childrenByParent.get(rootPid) ?? [])];
  while (stack.length > 0) {
    const pid = stack.pop() as number;
    if (out.has(pid)) continue;

    out.add(pid);
    stack.push(...(childrenByParent.get(pid) ?? []));
  }

  return out;
}

/**
 * True when the process command is a `claude` invocation. Within a
 * remote's subtree the only two command shapes are the command child
 * (`.../bin/true-bdd <subcommand>`) and the Claude CLI — which may appear
 * as a native `.../claude ...` binary OR a node-wrapped
 * `node .../claude-code/cli.js`. Matching "claude" anywhere while
 * excluding the true-bdd binary distinguishes them robustly.
 */
export function isClaudeCommand(command: string): boolean {
  return /claude/i.test(command) && !command.includes("true-bdd");
}

/** The `claude` processes currently descended from `rootPid`. */
export function claudeDescendants(rootPid: number): ProcInfo[] {
  const snapshot = psSnapshot();
  const descendants = descendantPids(snapshot, rootPid);

  return snapshot.filter((proc) => descendants.has(proc.pid) && isClaudeCommand(proc.command));
}

/**
 * Polls a running remote's subtree and accumulates the DISTINCT claude
 * pids ever seen. Start before dispatch, stop after the run is terminal,
 * then assertWithinBudget with the per-test bound.
 */
export class ClaudeCallBudget {
  private readonly seen = new Set<number>();
  private timer: NodeJS.Timeout | undefined;

  constructor(
    private readonly rootPid: number,
    private readonly intervalMs = 350,
  ) {}

  start(): this {
    if (this.timer !== undefined) {
      return this;
    }

    this.timer = setInterval(() => {
      for (const proc of claudeDescendants(this.rootPid)) {
        this.seen.add(proc.pid);
      }
    }, this.intervalMs);
    // Do not keep the event loop alive on this watcher alone.
    this.timer.unref?.();

    return this;
  }

  stop(): void {
    if (this.timer !== undefined) {
      clearInterval(this.timer);
      this.timer = undefined;
    }
  }

  /** Distinct claude processes observed so far. */
  get count(): number {
    return this.seen.size;
  }

  /**
   * Throws when more than `bound` distinct claude processes were spawned
   * — the defence against a checklist filter that quietly walks the full
   * checklist. `label` names the test in the failure.
   */
  assertWithinBudget(bound: number, label: string): void {
    if (this.seen.size > bound) {
      throw new Error(
        `${label}: AI-call budget exceeded — observed ${this.seen.size} distinct claude ` +
          `process(es) under remote pid ${this.rootPid}, bound is ${bound}. A single-check ` +
          `fixture spawning this many suggests the checklist filter selected more than one prompt.`,
      );
    }
  }
}

/**
 * Resolves once at least one live `claude` descendant exists under
 * `rootPid` (A9: only signal a real, in-flight Claude call). Returns the
 * first such pid; throws on timeout.
 */
export async function waitForClaudeDescendant(rootPid: number, timeoutMs: number): Promise<number> {
  const deadline = Date.now() + timeoutMs;

  for (;;) {
    const live = claudeDescendants(rootPid);
    if (live.length > 0) {
      return live[0].pid;
    }

    if (Date.now() >= deadline) {
      throw new Error(
        `no live claude descendant appeared under remote pid ${rootPid} within ${timeoutMs}ms`,
      );
    }

    await sleep(250);
  }
}

/**
 * Resolves once NO `claude` descendant remains under `rootPid` (A9:
 * bounded cleanup after the interrupt). Throws with the survivors if the
 * subtree still holds a claude process at the deadline.
 */
export async function expectNoClaudeDescendants(rootPid: number, timeoutMs: number): Promise<void> {
  const deadline = Date.now() + timeoutMs;

  for (;;) {
    const live = claudeDescendants(rootPid);
    if (live.length === 0) {
      return;
    }

    if (Date.now() >= deadline) {
      throw new Error(
        `claude descendant(s) still alive under remote pid ${rootPid} after ${timeoutMs}ms: ` +
          live.map((proc) => `${proc.pid} (${proc.command.slice(0, 60)})`).join(", "),
      );
    }

    await sleep(250);
  }
}
