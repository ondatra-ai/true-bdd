/**
 * Global setup (plan §4.1): ONE production `next build` and ONE Go
 * binary build per invocation, into a unique mkdtemp suite root.
 * Everything else (server, DB, fixture copy, browser context) is
 * test-scoped.
 *
 * Set TRUE_BDD_E2E_SKIP_BUILD=1 to reuse the existing `.next` build
 * during local iteration (the Go binaries are always rebuilt — they
 * are fast and staleness there is subtle).
 */

import { spawn } from "node:child_process";
import fs from "node:fs";
import path from "node:path";

import { createSuiteContext, type SuiteContext } from "./helpers/suite-root";

export default async function globalSetup(): Promise<void> {
  const ctx = await createSuiteContext();

  console.log(`[harness-e2e] suite root: ${ctx.suiteRoot}`);
  console.log(`[harness-e2e] true-bdd binary: ${ctx.binPath}`);

  if (process.env.TRUE_BDD_E2E_SKIP_BUILD === "1") {
    console.log("[harness-e2e] TRUE_BDD_E2E_SKIP_BUILD=1 — reusing existing .next build");
    return;
  }

  console.log("[harness-e2e] running production `next build` (once per invocation)...");
  await buildHarness(ctx);
  console.log("[harness-e2e] next build complete");
}

async function buildHarness(ctx: SuiteContext): Promise<void> {
  const logPath = path.join(ctx.suiteRoot, "next-build.log");
  const logFd = fs.openSync(logPath, "w");

  const exitCode = await new Promise<number>((resolve, reject) => {
    const nextBin = path.join(ctx.harnessDir, "node_modules", ".bin", "next");
    const child = spawn(nextBin, ["build"], {
      cwd: ctx.harnessDir,
      stdio: ["ignore", logFd, logFd],
      env: { ...process.env },
    });

    child.once("error", reject);
    child.once("exit", (code) => resolve(code ?? 1));
  });

  fs.closeSync(logFd);

  if (exitCode !== 0) {
    const tail = fs.readFileSync(logPath, "utf8").split("\n").slice(-40).join("\n");
    throw new Error(
      `next build failed (exit ${exitCode}) — a build failure is a FAILURE, never a skip.\n` +
        `log: ${logPath}\n${tail}`,
    );
  }
}
