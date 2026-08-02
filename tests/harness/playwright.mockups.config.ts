/**
 * Mockup-only Playwright config for the S&F workspace-UI DESIGN task
 * (`harness-workspace-ui-design`).
 *
 * Deliberately SEPARATE from `playwright.config.ts`:
 *   - the default config's `globalSetup` builds the harness Docker image and
 *     starts Redis on EVERY invocation regardless of `--project`; the static-HTML
 *     mockup specs must not pay (or depend on) that. This config has NO
 *     `globalSetup`/`globalTeardown`, NO `webServer`, and starts nothing.
 *   - the existing `protocol`/`ai` projects stay byte-for-byte untouched. The
 *     default config already never runs `m*` (no project's `testMatch` matches
 *     it), and this config only runs `m*` (never `p*`/`a*`), so the two suites
 *     are fully isolated.
 *
 * The mockups open over `file://` URLs (helpers/mockups.ts → mockupUrl); there is
 * no `baseURL`. The 1440×900 viewport pins the brief's desktop reference so every
 * mockup spec renders at 1440px (round 2, finding B4 / m1.8).
 *
 * Run: `cd tests/harness && npx playwright test -c playwright.mockups.config.ts`
 */

import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: ".",
  // Never collect from the suite's deps or fixture input trees.
  testIgnore: ["**/node_modules/**", "**/fixtures/**", "**/test-results/**"],
  workers: 1,
  fullyParallel: false,
  retries: 0,
  forbidOnly: !!process.env.CI,
  outputDir: "./test-results/mockups-artifacts",
  reporter: [["list"]],
  use: {
    trace: "retain-on-failure",
  },
  projects: [
    {
      name: "mockups",
      testMatch: "**/m[0-9]*.spec.ts",
      use: {
        viewport: { width: 1440, height: 900 },
      },
    },
  ],
});
