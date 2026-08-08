/**
 * Playwright configuration for the harness E2E suite (plan §4).
 *
 * This suite is a self-contained package under `tests/harness/` (its own
 * package.json + node_modules); the config sits at the suite root, so every
 * path below is relative to `tests/harness/`.
 *
 * - Production runtime only: global setup runs ONE host `next build` into a
 *   cached standalone bundle; each test starts its own `node server.js` from a
 *   per-instance clone of that bundle via ServerController — no docker per test.
 * - Parallel by default: `workers: 8` (override with PW_WORKERS),
 *   `fullyParallel: true`. Per-test isolation comes from dynamic host ports,
 *   per-test Redis key prefixes, and per-test scratch dirs — not from serialism.
 * - Projects by filename convention at the suite root:
 *     protocol  — p<N>-*.spec.ts  (never resolves or executes claude)
 *     workspace — w<N>-*.spec.ts  (file-as-source workspace UI; no claude;
 *                 ~3-minute timeout; shares global-setup with protocol/ai)
 *     ai        — a<N>-*.spec.ts  (real Claude; 30-minute timeout,
 *                 retries 0; depends on the ai-gate probe)
 * - No baseURL: every test owns its server and builds its own URLs /
 *   API contexts at runtime (plan §4.1).
 */

import { defineConfig } from "@playwright/test";

const MINUTE_MS = 60 * 1000;

export default defineConfig({
  testDir: ".",
  // Never collect specs from the suite's own deps or from fixture input trees
  // (which may ship host-project *.spec.ts of their own).
  testIgnore: ["**/node_modules/**", "**/fixtures/**", "**/test-results/**"],
  // Parallel by default; PW_WORKERS overrides. Per-test isolation comes from
  // dynamic ports + per-test Redis prefixes + per-test dirs (not serialism).
  workers: Number(process.env.PW_WORKERS ?? 8),
  fullyParallel: true,
  retries: 0,
  forbidOnly: !!process.env.CI,
  globalSetup: "./global-setup.ts",
  globalTeardown: "./global-teardown.ts",
  outputDir: "./test-results/artifacts",
  reporter: [["list"], ["./reporters/ai-banner-reporter.ts"]],
  use: {
    trace: "retain-on-failure",
  },
  projects: [
    {
      name: "protocol",
      testMatch: "**/p[0-9]*.spec.ts",
      timeout: 2 * MINUTE_MS,
    },
    {
      name: "workspace",
      testMatch: "**/w[0-9]*.spec.ts",
      timeout: 3 * MINUTE_MS,
    },
    // TEMPORARY (2026-08-06): the real-Claude `ai` project (and its gate) is
    // opt-in via RUN_AI=1 to keep plain `playwright test` runs fast. Remove
    // the guard to restore the a*-specs to the default suite.
    ...(process.env.RUN_AI
      ? [
          {
            name: "ai-gate",
            testMatch: "**/ai.gate.setup.ts",
            timeout: 5 * MINUTE_MS,
          },
        ]
      : []),
    {
      // Authoring-time only: captures the committed design gold-standard
      // screenshots from the booted prototype (tests/harness/goldens/). The
      // spec self-skips unless UPDATE_GOLDENS=1, so a plain `playwright test`
      // never silently rewrites goldens — run `npm run goldens:update`.
      name: "goldens",
      testMatch: "**/goldens.update.spec.ts",
      timeout: 10 * MINUTE_MS,
    },
    ...(process.env.RUN_AI
      ? [
          {
            name: "ai",
            testMatch: "**/a[0-9]*.spec.ts",
            timeout: 30 * MINUTE_MS,
            retries: 0,
            dependencies: ["ai-gate"],
          },
        ]
      : []),
  ],
});
