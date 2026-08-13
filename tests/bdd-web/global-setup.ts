/**
 * Global setup (perf plan Phase 1, host-process variant): ONE Go-binary build
 * and ONE harness standalone bundle per invocation, into a unique mkdtemp suite
 * root / a content-hash-keyed cache; Redis comes up once. Everything else
 * (per-test host server, fixture copy, browser context) is test-scoped.
 *
 * The harness app is built once here via a host `next build` and assembled into
 * a Next standalone bundle (helpers/harness-build.ts). Each test then clones that
 * bundle and launches its own `node server.js` (ServerController) — no docker
 * container per test. The bundle is content-hash cached, so a warm invocation
 * skips both `npm ci` and `next build`; TRUE_BDD_E2E_FORCE_BUILD=1 forces a rebuild.
 */

import { ensureHarnessDeps, ensureStandaloneBundle } from "./helpers/harness-build";
import { ensureRedisUp } from "./helpers/redis";
import { createSuiteContext } from "./helpers/suite-root";

export default async function globalSetup(): Promise<void> {
  // Goldens capture (`npm run goldens:update` → --project=goldens) only boots
  // the design prototype — no harness bundle, Go binaries, or Redis. The env
  // var is set exclusively by that npm script, scoped to the goldens project.
  if (process.env.UPDATE_GOLDENS === "1") {
    console.log("[harness-e2e] UPDATE_GOLDENS=1 — goldens capture run; skipping suite build + Redis setup");

    return;
  }

  const ctx = await createSuiteContext();

  console.log(`[harness-e2e] suite root: ${ctx.suiteRoot}`);
  console.log(`[harness-e2e] true-bdd binary: ${ctx.binPath}`);

  // Harness deps + standalone bundle BEFORE Redis so the suite context's
  // standalone path is populated before any helper reads it.
  await ensureHarnessDeps();
  const bundleDir = await ensureStandaloneBundle();
  console.log(`[harness-e2e] harness standalone bundle: ${bundleDir}`);

  // Redis coordination backend: ONE container, brought up ONCE and healthcheck-
  // waited (never a fixed sleep — plan r1 #15). Each test names a unique
  // REDIS_KEY_PREFIX so parallel/sequential tests share Redis without cross-talk;
  // the container is left running across local invocations (see global-teardown).
  await ensureRedisUp();
}
