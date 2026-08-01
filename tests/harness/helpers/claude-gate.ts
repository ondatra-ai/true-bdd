/**
 * Claude availability gate (plan §4.4) — used ONLY by the `ai`
 * project's setup. Protocol tests never resolve `claude`.
 *
 * The probe runs once per invocation from a FRESH EMPTY cwd, with
 * CLAUDECODE stripped (the probe is the only place the helper level
 * strips it — AI remotes run WITH the sentinel, see remote-process.ts),
 * a bounded timeout, and the engine-configured model.
 *
 * Build/browser/Go/SQLite/server failures are FAILURES, never skips;
 * only this probe's result may turn AI tests into (loud) skips. The
 * result is persisted to `<suiteRoot>/ai-gate.json` so both the
 * A-tests (skipUnlessClaudeAvailable) and the AI banner reporter can
 * read it.
 */

import { execFile } from "node:child_process";
import fs from "node:fs";
import path from "node:path";

import type { test as playwrightTest } from "@playwright/test";

import { suiteContext } from "./suite-root";

/** Fallback when the engine config cannot be read. */
const DEFAULT_MODEL = "claude-opus-4-8";

const DEFAULT_PROBE_TIMEOUT_MS = 3 * 60 * 1000;

export interface ClaudeGateResult {
  ok: boolean;
  /** Human-readable reason when !ok; "" when ok. */
  reason: string;
  /** Model the probe targeted (engine-configured). */
  model: string;
  /** Raw probe output (trimmed), for diagnostics. */
  output?: string;
  probedAt: string;
}

export function gateFilePath(): string {
  return path.join(suiteContext().suiteRoot, "ai-gate.json");
}

/**
 * Reads `engine.model` from the live engine config
 * (`<repo>/true-bdd/true-bdd.yaml`).
 */
export function engineModel(): string {
  try {
    const configPath = path.join(suiteContext().repoRoot, "true-bdd", "true-bdd.yaml");
    const raw = fs.readFileSync(configPath, "utf8");
    const match = raw.match(/^\s*model:\s*"?([^"\n]+)"?\s*$/m);

    return match ? match[1].trim() : DEFAULT_MODEL;
  } catch {
    return DEFAULT_MODEL;
  }
}

/**
 * Probes `claude -p 'reply with exactly: ok'` from a fresh empty tmp
 * cwd with CLAUDECODE stripped and a bounded timeout.
 */
export async function probeClaudeGate(
  options: { timeoutMs?: number } = {},
): Promise<ClaudeGateResult> {
  const timeoutMs = options.timeoutMs ?? DEFAULT_PROBE_TIMEOUT_MS;
  const model = engineModel();
  const probedAt = new Date().toISOString();

  const probeCwd = fs.mkdtempSync(path.join(suiteContext().suiteRoot, "claude-gate-"));

  const env: NodeJS.ProcessEnv = { ...process.env };
  delete env.CLAUDECODE; // ONLY the gate probe strips it (plan §4.4)

  return new Promise<ClaudeGateResult>((resolve) => {
    execFile(
      "claude",
      ["-p", "reply with exactly: ok", "--model", model],
      { cwd: probeCwd, env, timeout: timeoutMs, maxBuffer: 1024 * 1024 },
      (error, stdout, stderr) => {
        const output = `${stdout}\n${stderr}`.trim();

        if (error === null) {
          resolve({ ok: true, reason: "", model, output, probedAt });
          return;
        }

        const err = error as NodeJS.ErrnoException & { killed?: boolean };
        let reason: string;
        if (err.code === "ENOENT") {
          reason = "`claude` CLI not found on PATH";
        } else if (err.killed) {
          reason = `claude probe timed out after ${timeoutMs}ms`;
        } else {
          reason = `claude probe failed: ${err.message.split("\n")[0]}`;
        }

        resolve({ ok: false, reason, model, output, probedAt });
      },
    );
  });
}

export function writeGateResult(result: ClaudeGateResult): void {
  fs.writeFileSync(gateFilePath(), `${JSON.stringify(result, null, 2)}\n`);
}

export function readGateResult(): ClaudeGateResult | undefined {
  try {
    return JSON.parse(fs.readFileSync(gateFilePath(), "utf8")) as ClaudeGateResult;
  } catch {
    return undefined;
  }
}

/**
 * Call at the TOP of every A-test body:
 *
 *   test("A1 ...", async ({ page }) => {
 *     skipUnlessClaudeAvailable(test);
 *     ...
 *   });
 *
 * Skips (loudly, with the probe's reason) when the gate probe failed
 * or did not run. The AI banner reporter surfaces the same reason in
 * its end-of-run banner, so a skipped AI project can never read as
 * silently green.
 */
export function skipUnlessClaudeAvailable(test: typeof playwrightTest): void {
  const gate = readGateResult();

  test.skip(
    gate === undefined || !gate.ok,
    `claude gate: ${gate?.reason ?? "probe did not run (ai-gate project skipped?)"}`,
  );
}
