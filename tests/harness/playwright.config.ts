/**
 * Playwright configuration for the harness E2E suite (plan §4).
 *
 * This suite is a self-contained package under `tests/harness/` (its own
 * package.json + node_modules); the config sits at the suite root, so every
 * path below is relative to `tests/harness/`.
 *
 * - Production runtime only: global setup runs ONE `next build`; each
 *   test starts its own server via ServerController.
 * - `workers: 1` is explicit — serial is configuration, not intention.
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
  workers: 1,
  fullyParallel: false,
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
    {
      name: "ai-gate",
      testMatch: "**/ai.gate.setup.ts",
      timeout: 5 * MINUTE_MS,
    },
    {
      // Authoring-time only: captures the committed design gold-standard
      // screenshots from the booted prototype (tests/harness/goldens/). The
      // spec self-skips unless UPDATE_GOLDENS=1, so a plain `playwright test`
      // never silently rewrites goldens — run `npm run goldens:update`.
      name: "goldens",
      testMatch: "**/goldens.update.spec.ts",
      timeout: 10 * MINUTE_MS,
    },
    {
      name: "ai",
      testMatch: "**/a[0-9]*.spec.ts",
      timeout: 30 * MINUTE_MS,
      retries: 0,
      dependencies: ["ai-gate"],
    },
  ],
});
