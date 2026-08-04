/**
 * Global setup (plan §4.1, compose e2e variant): ONE harness image build and
 * ONE Go-binary build per invocation, into a unique mkdtemp suite root; Redis
 * comes up once. Everything else (per-test harness container, fixture copy,
 * browser context) is test-scoped.
 *
 * The harness image (`true-bdd-harness:e2e`) is built once here via
 * `docker compose build harness`; each test then launches its own container
 * from that image (ServerController). Set TRUE_BDD_E2E_SKIP_BUILD=1 to reuse the
 * existing image during local iteration (the Go binaries are always rebuilt —
 * they are fast and staleness there is subtle).
 */

import { spawn } from "node:child_process";
import fs from "node:fs";
import path from "node:path";

import { ensureRedisUp } from "./helpers/redis";
import { createSuiteContext, type SuiteContext } from "./helpers/suite-root";

export default async function globalSetup(): Promise<void> {
  // Goldens capture (`npm run goldens:update` → --project=goldens) only boots
  // the design prototype — no harness image, Go binaries, or Redis. The env
  // var is set exclusively by that npm script, scoped to the goldens project.
  if (process.env.UPDATE_GOLDENS === "1") {
    console.log("[harness-e2e] UPDATE_GOLDENS=1 — goldens capture run; skipping suite build + Redis setup");

    return;
  }

  const ctx = await createSuiteContext();

  console.log(`[harness-e2e] suite root: ${ctx.suiteRoot}`);
  console.log(`[harness-e2e] true-bdd binary: ${ctx.binPath}`);

  // Redis coordination backend: ONE container, brought up ONCE and healthcheck-
  // waited (never a fixed sleep — plan r1 #15). Each test names a unique
  // REDIS_KEY_PREFIX so sequential tests share Redis without cross-talk; the
  // container is stopped in global-teardown.
  await ensureRedisUp();

  if (process.env.TRUE_BDD_E2E_SKIP_BUILD === "1") {
    console.log("[harness-e2e] TRUE_BDD_E2E_SKIP_BUILD=1 — reusing existing harness image");
    return;
  }

  console.log("[harness-e2e] building harness image `true-bdd-harness:e2e` (once per invocation)...");
  await buildHarnessImage(ctx);
  console.log("[harness-e2e] harness image build complete");
}

/**
 * Builds the harness Docker image via `docker compose build harness`. A build
 * failure is a FAILURE, never a skip (mirrors the old `next build` discipline).
 */
async function buildHarnessImage(ctx: SuiteContext): Promise<void> {
  const logPath = path.join(ctx.suiteRoot, "harness-image-build.log");
  const logFd = fs.openSync(logPath, "w");

  const exitCode = await new Promise<number>((resolve, reject) => {
    const child = spawn(
      "docker",
      ["compose", "-f", "docker-compose.yml", "build", "harness"],
      { cwd: ctx.repoRoot, stdio: ["ignore", logFd, logFd], env: { ...process.env } },
    );

    child.once("error", reject);
    child.once("exit", (code) => resolve(code ?? 1));
  });

  fs.closeSync(logFd);

  if (exitCode !== 0) {
    const tail = fs.readFileSync(logPath, "utf8").split("\n").slice(-40).join("\n");
    throw new Error(
      `harness image build failed (exit ${exitCode}) — a build failure is a FAILURE, never a skip.\n` +
        `log: ${logPath}\n${tail}`,
    );
  }
}
