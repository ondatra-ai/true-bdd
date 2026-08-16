/**
 * proto-baseline.ts — boots the runnable file-as-source PROTOTYPE
 * (`services/bdd-web/design/proto-workspace/app`) live on a free port so the w15
 * design-judge pairs can screenshot the design-truth BASELINE straight from the
 * repo (task R8) rather than a committed PNG. The prototype is the UNCHANGED
 * oracle — production is made to match it; the prototype is never edited here.
 *
 * Concretely (planner Codex F11–F15 + round-2 #5):
 *   - Install (F14): ONCE, only when node_modules is absent/invalid; prefer
 *     `npm ci` (the dir ships a lockfile) for determinism; own bounded timeout +
 *     a captured log; an install error FAILS (never skips).
 *   - Launch (F11): the package scripts hardcode `-p 3999`, so spawn the local
 *     `next` binary directly with ONE explicit allocated port; retain the child
 *     handle immediately after spawn; read the actual listening URL from stdout.
 *   - Port + readiness (F12/F13): allocate via `allocatePort()`; on EADDRINUSE
 *     retry with a freshly allocated port, bounded; monitor the child's early
 *     `exit` during readiness (fail fast with output attached); readiness is
 *     PER-BASELINE-ROUTE (Next dev compiles routes lazily — `/product` ready
 *     does NOT imply `/story/60-1` or `/feature/summaries` compiled).
 *   - Teardown (F15): idempotent; kill the retained child TREE in a `finally` on
 *     EVERY readiness/setup failure; graceful terminate then bounded forced
 *     kill; safe if init rejected before returning a handle.
 *   - Skip vs fail: the helper NEVER skips — it returns a live
 *     `{ baseURL, stop() }` or throws with diagnostics; only the SPEC skips, and
 *     only for a missing `codex` CLI.
 */

import { spawn, type ChildProcess } from "node:child_process";
import fs from "node:fs";
import net from "node:net";
import path from "node:path";

import { REPO_ROOT } from "./design-conformance";

/** The prototype app directory (its own package.json + lockfile + node_modules). */
const PROTO_APP_DIR = path.join(REPO_ROOT, "services", "bdd-web", "design", "proto-workspace", "app");

/** Boot/teardown logs land under the repo's gitignored runtime dir, tmp/. */
export const PROTO_ARTIFACT_DIR = path.join(REPO_ROOT, "tmp", "proto-baseline");

/**
 * The exact baseline routes the w15 pairs screenshot — every one is warmed
 * during readiness because Next dev compiles routes lazily on first request.
 * `storySlugFromId` maps `60.1` → `60-1`; `summaries` is a seeded feature id.
 */
export const PROTO_BASELINE_ROUTES = {
  productLanding: "/product",
  story: "/story/60-1",
  feature: "/feature/summaries",
  // Warmed for the w9/w11 judges too — the prototype replaced the deleted
  // static mockups as the workspace-overview baseline.
  workspaceOverview: "/workspace-overview",
  // The sessions-home baseline (w19): the prototype's `/sessions` page — the
  // design-truth for production `/` (gradient top bar + wordmark/tagline +
  // `row-list` rows), warmed so the w19 judge screenshots a compiled route.
  sessions: "/sessions",
} as const;

const LAUNCH_ATTEMPTS = 4; // mirrors ServerController's bounded bind retries
const URL_WAIT_MS = 60_000; // window to read the listening URL from stdout
const READINESS_BUDGET_MS = 180_000; // shared across the warmed routes
const ROUTE_FETCH_TIMEOUT_MS = 25_000;
const POLL_INTERVAL_MS = 1_000;
const GRACEFUL_KILL_MS = 5_000;
// Kept BELOW every w15 test timeout (12 min) so a first-run `npm ci` self-kills
// before Playwright aborts the test — otherwise the installer would outlive the
// test and leak (round-1 F1).
const INSTALL_TIMEOUT_MS = 360_000;

export interface PrototypeServer {
  /** `http://127.0.0.1:<port>` of the live prototype (parsed from its stdout). */
  baseURL: string;
  /** Absolute path to the captured boot/run log (a diagnosable artifact). */
  logPath: string;
  /** Idempotent, tree-killing teardown. */
  stop: () => Promise<void>;
}

const delay = (ms: number): Promise<void> => new Promise((resolve) => setTimeout(resolve, ms));

/** Allocates a free TCP port on the loopback interface. */
function allocatePort(): Promise<number> {
  return new Promise((resolve, reject) => {
    const srv = net.createServer();
    srv.once("error", reject);
    srv.listen(0, "127.0.0.1", () => {
      const address = srv.address();
      const port = typeof address === "object" && address !== null ? address.port : 0;
      srv.close(() => (port > 0 ? resolve(port) : reject(new Error("allocatePort: no port"))));
    });
  });
}

/** Signals a whole process GROUP (detached spawn → negative pid), best-effort. */
function killGroup(pid: number, signal: NodeJS.Signals): void {
  try {
    process.kill(-pid, signal);
  } catch {
    try {
      process.kill(pid, signal);
    } catch {
      /* already gone */
    }
  }
}

/** Graceful SIGTERM then bounded forced SIGKILL of the child tree; idempotent-safe. */
async function killChildTree(child: ChildProcess): Promise<void> {
  const pid = child.pid;
  if (pid === undefined) {
    return;
  }
  if (child.exitCode !== null || child.signalCode !== null) {
    return; // already exited
  }
  const exited = new Promise<void>((resolve) => child.once("exit", () => resolve()));
  killGroup(pid, "SIGTERM");
  const wentDown = await Promise.race([exited.then(() => true), delay(GRACEFUL_KILL_MS).then(() => false)]);
  if (!wentDown) {
    killGroup(pid, "SIGKILL");
    await Promise.race([exited, delay(GRACEFUL_KILL_MS)]);
  }
}

/**
 * The in-flight SETUP child (`npm ci` or `next build`), tracked module-side so
 * `stopPrototype` can force-kill it even while the boot singleton is still
 * pending — otherwise a test that times out mid-setup would leak the child AND
 * wedge teardown on the pending promise (round-1 F1).
 */
let activeInstall: ChildProcess | null = null;

/** Isolated production dist dir for golden captures (next.config.js →
 * PROTO_DIST_DIR): never collides with a developer's long-running `next dev`
 * on the default `.next`. Gitignored via harness/.gitignore. */
const GOLDEN_DIST_DIR = ".next-goldens";
const BUILD_TIMEOUT_MS = 5 * 60_000;

/** True when this process is a goldens capture run (`npm run goldens:update`):
 * the baseline must then be a PRODUCTION build — the same `next build`
 * pipeline the harness app ships with — because dev-mode rendering differs at
 * the sub-pixel level and contaminates the w17 pixel-parity goldens. */
function goldenCaptureMode(): boolean {
  return process.env.UPDATE_GOLDENS === "1";
}

/** Runs `next build` (into GOLDEN_DIST_DIR) for golden captures; throws on
 * failure with the build log named. */
async function buildPrototype(): Promise<void> {
  fs.mkdirSync(PROTO_ARTIFACT_DIR, { recursive: true });
  const logPath = path.join(PROTO_ARTIFACT_DIR, "proto-build.log");
  const logFd = fs.openSync(logPath, "w");
  const nextBin = path.join(PROTO_APP_DIR, "node_modules", ".bin", "next");

  await new Promise<void>((resolve, reject) => {
    const child = spawn(nextBin, ["build"], {
      cwd: PROTO_APP_DIR,
      detached: true, // own group → killable as a tree by stopPrototype
      stdio: ["ignore", logFd, logFd],
      env: { ...process.env, NEXT_TELEMETRY_DISABLED: "1", PROTO_DIST_DIR: GOLDEN_DIST_DIR },
    });
    activeInstall = child;
    const timer = setTimeout(() => killChildTree(child), BUILD_TIMEOUT_MS);
    child.once("error", (err) => {
      clearTimeout(timer);
      reject(new Error(`prototype \`next build\` failed to spawn: ${err.message}; log: ${logPath}`));
    });
    child.once("exit", (code, signal) => {
      clearTimeout(timer);
      if (code === 0) {
        resolve();
      } else {
        reject(new Error(`prototype \`next build\` exited (code ${code}, signal ${signal}); log: ${logPath}`));
      }
    });
  }).finally(() => {
    activeInstall = null;
    try {
      fs.closeSync(logFd);
    } catch {
      /* best effort */
    }
  });
}

/**
 * Runs `npm ci` ONCE when the prototype's deps are absent (F14: lockfile-only,
 * never `npm install` — a missing lockfile FAILS diagnostically rather than
 * mutating dependency state, round-1 F3). Fails, never skips.
 */
async function ensureInstalled(): Promise<void> {
  const nextBin = path.join(PROTO_APP_DIR, "node_modules", ".bin", "next");
  if (fs.existsSync(nextBin)) {
    return; // deps already present + valid enough to launch
  }
  const lockfile = path.join(PROTO_APP_DIR, "package-lock.json");
  if (!fs.existsSync(lockfile)) {
    throw new Error(
      `prototype deps are absent AND no package-lock.json at ${lockfile} — cannot run a deterministic \`npm ci\``,
    );
  }
  fs.mkdirSync(PROTO_ARTIFACT_DIR, { recursive: true });
  const logPath = path.join(PROTO_ARTIFACT_DIR, "proto-npm-ci.log");
  const logFd = fs.openSync(logPath, "w");

  await new Promise<void>((resolve, reject) => {
    const child = spawn("npm", ["ci"], {
      cwd: PROTO_APP_DIR,
      detached: true, // own group → killable as a tree by stopPrototype
      stdio: ["ignore", logFd, logFd],
      env: { ...process.env, NEXT_TELEMETRY_DISABLED: "1" },
    });
    activeInstall = child;
    const timer = setTimeout(() => killChildTree(child), INSTALL_TIMEOUT_MS);
    child.once("error", (err) => {
      clearTimeout(timer);
      reject(new Error(`npm ci failed to spawn: ${err.message}; log: ${logPath}`));
    });
    child.once("exit", (code, signal) => {
      clearTimeout(timer);
      if (code === 0) {
        resolve();
      } else {
        reject(new Error(`npm ci exited (code ${code}, signal ${signal}); log: ${logPath}`));
      }
    });
  }).finally(() => {
    activeInstall = null;
    try {
      fs.closeSync(logFd);
    } catch {
      /* best effort */
    }
  });
}

async function fetchOk(url: string): Promise<{ ok: boolean; detail: string }> {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), ROUTE_FETCH_TIMEOUT_MS);
  try {
    const res = await fetch(url, { signal: controller.signal, headers: { accept: "text/html" } });
    if (res.status !== 200) {
      return { ok: false, detail: `HTTP ${res.status}` };
    }
    const body = await res.text();
    // A real compiled page ships the App-Router flight payload / an <html> doc;
    // anything shorter is a not-yet-ready shell.
    if (/__next_f|__NEXT_DATA__|<html/i.test(body)) {
      return { ok: true, detail: "200" };
    }

    return { ok: false, detail: `200 but body not a page (${body.length}b)` };
  } catch (err) {
    return { ok: false, detail: `fetch error: ${(err as Error).message}` };
  } finally {
    clearTimeout(timer);
  }
}

/** Boots the prototype once; retries on EADDRINUSE with a fresh port. */
async function startPrototype(): Promise<PrototypeServer> {
  await ensureInstalled();
  const production = goldenCaptureMode();
  if (production) {
    await buildPrototype();
  }
  fs.mkdirSync(PROTO_ARTIFACT_DIR, { recursive: true });
  const nextBin = path.join(PROTO_APP_DIR, "node_modules", ".bin", "next");

  let lastError: Error | undefined;
  for (let attempt = 1; attempt <= LAUNCH_ATTEMPTS; attempt += 1) {
    const port = await allocatePort();
    const logPath = path.join(PROTO_ARTIFACT_DIR, `proto-boot.attempt-${attempt}.log`);
    const logFd = fs.openSync(logPath, "w");

    let output = "";
    let exited: { code: number | null; signal: NodeJS.Signals | null } | null = null;

    const child = spawn(
      nextBin,
      [production ? "start" : "dev", "-H", "127.0.0.1", "-p", String(port)],
      {
        cwd: PROTO_APP_DIR,
        detached: true, // own process group → tree kill on teardown
        stdio: ["ignore", "pipe", "pipe"],
        env: {
          ...process.env,
          BROWSER: "none",
          NEXT_TELEMETRY_DISABLED: "1",
          ...(production ? { PROTO_DIST_DIR: GOLDEN_DIST_DIR } : {}),
        },
      },
    );
    // Retain + wire the handle IMMEDIATELY (F11/F15) so a crash mid-readiness is
    // observed and the child is always killable.
    const onData = (buf: Buffer): void => {
      try {
        fs.writeSync(logFd, buf);
      } catch {
        /* log best-effort */
      }
      output = (output + buf.toString()).slice(-8_000);
    };
    child.stdout?.on("data", onData);
    child.stderr?.on("data", onData);
    child.once("exit", (code, signal) => {
      exited = { code, signal };
    });

    try {
      const baseURL = await waitForListeningURL(child, () => exited, () => output, port);
      await waitForRoutesReady(baseURL, child, () => exited, () => output);

      return {
        baseURL,
        logPath,
        stop: makeStop(child, logFd),
      };
    } catch (err) {
      await killChildTree(child);
      try {
        fs.closeSync(logFd);
      } catch {
        /* best effort */
      }
      const retryable = /EADDRINUSE/i.test(output);
      lastError = new Error(
        `prototype boot attempt ${attempt}/${LAUNCH_ATTEMPTS} (port ${port}) failed: ${(err as Error).message}\n` +
          `log: ${logPath}\n--- output tail ---\n${output.slice(-1_500)}`,
      );
      if (!retryable) {
        throw lastError;
      }
    }
  }

  throw lastError ?? new Error("prototype boot: exhausted launch attempts");
}

function makeStop(child: ChildProcess, logFd: number): () => Promise<void> {
  let stopped = false;

  return async () => {
    if (stopped) {
      return;
    }
    stopped = true;
    await killChildTree(child);
    try {
      fs.closeSync(logFd);
    } catch {
      /* best effort */
    }
  };
}

/** Resolves the real base URL from the child's stdout (Next may pick a port). */
async function waitForListeningURL(
  child: ChildProcess,
  getExited: () => { code: number | null; signal: NodeJS.Signals | null } | null,
  getOutput: () => string,
  fallbackPort: number,
): Promise<string> {
  void child;
  const deadline = Date.now() + URL_WAIT_MS;
  while (Date.now() < deadline) {
    const exited = getExited();
    if (exited !== null) {
      throw new Error(
        `next dev exited before listening (code ${exited.code}, signal ${exited.signal})`,
      );
    }
    const match = getOutput().match(/https?:\/\/(?:localhost|127\.0\.0\.1|\[::\]|0\.0\.0\.0):(\d+)/i);
    if (match !== null) {
      return `http://127.0.0.1:${match[1]}`;
    }
    await delay(200);
  }

  // No banner parsed — fall back to the explicit port we asked for; the route
  // readiness poll below will surface a clear error if that is wrong.
  return `http://127.0.0.1:${fallbackPort}`;
}

/** Warms every baseline route (lazy compile) until it serves a real page. */
async function waitForRoutesReady(
  baseURL: string,
  child: ChildProcess,
  getExited: () => { code: number | null; signal: NodeJS.Signals | null } | null,
  getOutput: () => string,
): Promise<void> {
  void child;
  const deadline = Date.now() + READINESS_BUDGET_MS;
  for (const route of Object.values(PROTO_BASELINE_ROUTES)) {
    const url = `${baseURL}${route}`;
    let last = "no response";
    let ready = false;
    while (Date.now() < deadline) {
      const exited = getExited();
      if (exited !== null) {
        throw new Error(
          `next dev exited early (code ${exited.code}, signal ${exited.signal}) while warming ${route}`,
        );
      }
      const result = await fetchOk(url);
      if (result.ok) {
        ready = true;
        break;
      }
      last = result.detail;
      await delay(POLL_INTERVAL_MS);
    }
    if (!ready) {
      throw new Error(
        `timed out warming baseline route ${route} (last: ${last})\n--- output tail ---\n${getOutput().slice(-800)}`,
      );
    }
  }
}

// ── Module-level lazy singleton (boots ONCE, shared across the w15 cases) ──

let singleton: Promise<PrototypeServer> | null = null;

/**
 * Boots the prototype once and returns the SAME promise thereafter. A rejected
 * boot is CACHED (not retried) so every w15 case fails identically rather than
 * re-paying a doomed boot — and `startPrototype` has already torn down any child
 * it spawned before rejecting, so the cached rejection leaks nothing.
 */
export function bootPrototype(): Promise<PrototypeServer> {
  if (singleton === null) {
    singleton = startPrototype();
  }

  return singleton;
}

/**
 * Idempotent teardown for the spec's `test.afterAll`: stops a healthy singleton
 * (a clean boot must never be leaked), no-ops if boot never started or rejected.
 */
export async function stopPrototype(): Promise<void> {
  // Force-kill any in-flight installer FIRST so a boot still pending in
  // `ensureInstalled` cannot wedge teardown (its promise then rejects and the
  // `await pending` below is caught) — and the installer never leaks (F1).
  if (activeInstall?.pid !== undefined) {
    killGroup(activeInstall.pid, "SIGKILL");
  }
  if (singleton === null) {
    return;
  }
  const pending = singleton;
  singleton = null;
  let server: PrototypeServer | undefined;
  try {
    server = await pending;
  } catch {
    return; // boot rejected/killed — startPrototype already killed any child
  }
  await server.stop();
}
