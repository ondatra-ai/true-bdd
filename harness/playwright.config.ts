/**
 * Playwright configuration for the harness E2E suite (plan §4).
 *
 * - Production runtime only: global setup runs ONE `next build`; each
 *   test starts its own `next start` server via ServerController.
 * - `workers: 1` is explicit — serial is configuration, not intention.
 * - Two projects by filename convention under tests/e2e/:
 *     protocol — p<N>-*.spec.ts  (never resolves or executes claude)
 *     ai       — a<N>-*.spec.ts  (real Claude; 30-minute timeout,
 *                retries 0; depends on the ai-gate probe)
 * - No baseURL: every test owns its server and builds its own URLs /
 *   API contexts at runtime (plan §4.1).
 */

import { defineConfig } from "@playwright/test";

const MINUTE_MS = 60 * 1000;

export default defineConfig({
  testDir: "./tests/e2e",
  workers: 1,
  fullyParallel: false,
  retries: 0,
  forbidOnly: !!process.env.CI,
  globalSetup: "./tests/e2e/global-setup.ts",
  globalTeardown: "./tests/e2e/global-teardown.ts",
  outputDir: "./test-results/artifacts",
  reporter: [["list"], ["./tests/e2e/reporters/ai-banner-reporter.ts"]],
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
      name: "ai-gate",
      testMatch: "**/ai.gate.setup.ts",
      timeout: 5 * MINUTE_MS,
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
