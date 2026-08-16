/**
 * w15 — Product-section design-judge pairs (task
 * `product-section-prototype-parity`, R8). The judge's BASELINE is the
 * COMMITTED gold standard (tests/bdd-web/goldens/<name>.png), captured from
 * the runnable prototype at test-AUTHORING time via `npm run goldens:update`:
 * each case pairs its golden (image 1) with a fresh screenshot of the
 * corresponding production route (image 2) at 1440x900 and submits both to
 * `codex exec` with a schema-forced JSON rubric of named STRUCTURE checks.
 * Both images use the rail + docked sidebar, so nav chrome is compared
 * DIRECTLY. The prototype is never booted during a judge run — a green
 * verdict means "prod matches the baseline the author signed off".
 *
 * Skip discipline (F10/F16 — critical): the ONLY authorized skip is `codex`
 * absent. EVERY other failure (missing golden, readiness, screenshot,
 * navigation, judge exit) is a TEST FAILURE with captured logs — never a
 * skip, never a hang. A missing golden names the recapture command.
 *
 * RED intent (R9): current prod is missing the brand header, icon rail, kicker/
 * title block, lines counter, the Feature pill, and the linked-title section
 * rows — the judge names each missing surface and returns `verdict:"fail"`.
 */

import path from "node:path";

import { expect, test, type Page } from "@playwright/test";

import {
  DESKTOP_VIEWPORT,
  FEATURE_DETAIL_CHECK_NAMES,
  FEATURE_DETAIL_PROFILE,
  GOLDEN_NAMES,
  PRODUCT_LANDING_CHECK_NAMES,
  PRODUCT_LANDING_PROFILE,
  REPO_ROOT,
  STORY_PAGE_CHECK_NAMES,
  STORY_PAGE_PROFILE,
  auditVerdict,
  codexOnPath,
  requireGolden,
  runDesignJudge,
} from "./helpers/design-conformance";
import { gotoWorkspace, wsRoutes } from "./helpers/ui";
import { WorkspaceEnv } from "./helpers/workspace-env";

const ARTIFACT_DIR = path.join(REPO_ROOT, "tmp", "design-judge");

let env: WorkspaceEnv | undefined;

test.afterEach(async () => {
  const info = test.info();
  info.setTimeout(info.timeout + 60_000);
  const current = env;
  env = undefined;
  if (current !== undefined) {
    await current.teardown(test.info());
  }
});

/** Navigates, waits for each route anchor + web fonts, then screenshots to `png`. */
async function capture(page: Page, url: string, anchors: string[], png: string): Promise<void> {
  await page.setViewportSize({ ...DESKTOP_VIEWPORT });
  await page.goto(url, { waitUntil: "load" });
  for (const anchor of anchors) {
    await expect(page.getByTestId(anchor)).toBeVisible();
  }
  await page.evaluate(async () => {
    await document.fonts.ready;
  });
  await page.screenshot({ path: png });
}

async function runPair(
  opts: {
    page: Page;
    testInfo: import("@playwright/test").TestInfo;
    label: string;
    golden: string;
    prodRoute: (sid: string) => string;
    prodAnchors: string[];
    profile: import("./helpers/design-conformance").JudgeProfile;
    checkNames: readonly string[];
  },
): Promise<void> {
  const { page, testInfo } = opts;

  // The committed gold standard — a missing file FAILS with the recapture
  // command named (never a skip).
  const baselinePng = requireGolden(opts.golden);

  // Production env + screenshot.
  env = await WorkspaceEnv.start(opts.label);
  const e = env;
  const prodPng = testInfo.outputPath(`prod-${opts.label}.png`);
  await capture(page, new URL(opts.prodRoute(e.sid), e.baseURL).href, opts.prodAnchors, prodPng);

  // Codex vision verdict.
  const { verdict, verdictPath, tracePath, exitCode } = await runDesignJudge({
    mockupPng: baselinePng,
    prodPng,
    artifactDir: ARTIFACT_DIR,
    profile: opts.profile,
    label: `${opts.label}-${testInfo.testId}`,
  });
  await testInfo.attach("codex-verdict", { path: verdictPath, contentType: "application/json" });
  await testInfo.attach("codex-trace", { path: tracePath, contentType: "text/plain" });

  expect(
    exitCode,
    `codex judge process exited nonzero (${exitCode}); its verdict is untrustworthy. trace: ${tracePath}`,
  ).toBe(0);

  const { failedChecks, problems } = auditVerdict(verdict, opts.checkNames);
  expect(
    problems,
    `codex judge (exit ${exitCode}) returned a structurally invalid verdict:\n${problems.join("\n")}\n` +
      `verdict: ${verdictPath}  trace: ${tracePath}`,
  ).toEqual([]);

  const report = failedChecks.map((c) => `${c.name}: ${c.note}`).join("\n");
  expect(
    failedChecks,
    `codex judge (exit ${exitCode}) reported product-parity violations:\n${report}\n` +
      `verdict: ${verdictPath}  trace: ${tracePath}`,
  ).toEqual([]);
}

test("w15.1 product landing conforms to the gold standard (codex vision judge, R8)", async ({ page }, testInfo) => {
  test.skip(!codexOnPath(), "codex CLI not on PATH — the design vision judge requires it");
  testInfo.setTimeout(12 * 60_000);

  await runPair({
    page,
    testInfo,
    label: "w15-product-landing",
    golden: GOLDEN_NAMES.productLanding,
    prodRoute: (sid) => wsRoutes.product(sid),
    prodAnchors: ["rail", "sidebar", "file-view"],
    profile: PRODUCT_LANDING_PROFILE,
    checkNames: PRODUCT_LANDING_CHECK_NAMES,
  });
});

test("w15.2 story page conforms to the gold standard (codex vision judge, R8)", async ({ page }, testInfo) => {
  test.skip(!codexOnPath(), "codex CLI not on PATH — the design vision judge requires it");
  testInfo.setTimeout(12 * 60_000);

  await runPair({
    page,
    testInfo,
    label: "w15-story-page",
    golden: GOLDEN_NAMES.storyPage,
    prodRoute: (sid) => wsRoutes.story(sid, "60.1"),
    prodAnchors: ["rail", "sidebar", "file-view"],
    profile: STORY_PAGE_PROFILE,
    checkNames: STORY_PAGE_CHECK_NAMES,
  });
});

test("w15.3 feature detail conforms to the gold standard (codex vision judge, R8)", async ({ page }, testInfo) => {
  test.skip(!codexOnPath(), "codex CLI not on PATH — the design vision judge requires it");
  testInfo.setTimeout(12 * 60_000);

  await runPair({
    page,
    testInfo,
    label: "w15-feature-detail",
    golden: GOLDEN_NAMES.featureDetail,
    prodRoute: (sid) => wsRoutes.feature(sid, "summaries"),
    prodAnchors: ["rail", "sidebar", "feature-stories-list"],
    profile: FEATURE_DETAIL_PROFILE,
    checkNames: FEATURE_DETAIL_CHECK_NAMES,
  });
});
