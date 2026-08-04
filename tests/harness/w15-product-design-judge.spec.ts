/**
 * w15 — Product-section design-judge pairs (task
 * `product-section-prototype-parity`, R8). The judge's BASELINE is the runnable
 * PROTOTYPE booted live from the repo (helpers/proto-baseline.ts), NEVER a
 * committed PNG: each case screenshots the exact prototype route (image 1) and
 * the corresponding production route (image 2) at 1440x900 and submits both to
 * `codex exec` with a schema-forced JSON rubric of named STRUCTURE checks. Both
 * images use the rail + docked sidebar, so nav chrome is compared DIRECTLY.
 *
 * Skip discipline (F10/F16 — critical): the ONLY authorized skip is `codex`
 * absent. EVERY other failure (prototype install/boot, readiness, screenshot,
 * navigation, judge exit) is a TEST FAILURE with captured logs — never a skip,
 * never a hang. A rejected boot is cached, so it fails ALL THREE cases rather
 * than silently skipping (a broken baseline must not hide the judge).
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
  PRODUCT_LANDING_CHECK_NAMES,
  PRODUCT_LANDING_PROFILE,
  REPO_ROOT,
  STORY_PAGE_CHECK_NAMES,
  STORY_PAGE_PROFILE,
  auditVerdict,
  codexOnPath,
  runDesignJudge,
} from "./helpers/design-conformance";
import { PROTO_BASELINE_ROUTES, bootPrototype, stopPrototype } from "./helpers/proto-baseline";
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

// A clean boot must never be leaked (the failure-path finally in proto-baseline
// covers only a rejected boot).
test.afterAll(async () => {
  await stopPrototype();
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
    context: import("@playwright/test").BrowserContext;
    testInfo: import("@playwright/test").TestInfo;
    label: string;
    baselineRoute: string;
    baselineAnchors: string[];
    prodRoute: (sid: string) => string;
    prodAnchors: string[];
    profile: import("./helpers/design-conformance").JudgeProfile;
    checkNames: readonly string[];
  },
): Promise<void> {
  const { page, context, testInfo } = opts;

  // Boot the prototype (singleton — booted ONCE across the three cases). A
  // rejected boot throws here → this case FAILS with diagnostics (never skips).
  const proto = await bootPrototype();
  await testInfo.attach("prototype-boot-log", { path: proto.logPath, contentType: "text/plain" });

  // Production env + screenshot.
  env = await WorkspaceEnv.start(opts.label);
  const e = env;
  const prodPng = testInfo.outputPath(`prod-${opts.label}.png`);
  await capture(page, new URL(opts.prodRoute(e.sid), e.baseURL).href, opts.prodAnchors, prodPng);

  // Baseline (prototype) screenshot on a second page.
  const baselinePage = await context.newPage();
  const baselinePng = testInfo.outputPath(`baseline-${opts.label}.png`);
  try {
    await capture(baselinePage, `${proto.baseURL}${opts.baselineRoute}`, opts.baselineAnchors, baselinePng);
  } finally {
    await baselinePage.close();
  }

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

test("w15.1 product landing conforms to the prototype (codex vision judge, R8)", async ({ page, context }, testInfo) => {
  test.skip(!codexOnPath(), "codex CLI not on PATH — the design vision judge requires it");
  testInfo.setTimeout(12 * 60_000);

  await runPair({
    page,
    context,
    testInfo,
    label: "w15-product-landing",
    baselineRoute: PROTO_BASELINE_ROUTES.productLanding,
    baselineAnchors: ["mockup-canvas", "yaml-editor"],
    prodRoute: (sid) => wsRoutes.product(sid),
    prodAnchors: ["rail", "sidebar", "file-view"],
    profile: PRODUCT_LANDING_PROFILE,
    checkNames: PRODUCT_LANDING_CHECK_NAMES,
  });
});

test("w15.2 story page conforms to the prototype (codex vision judge, R8)", async ({ page, context }, testInfo) => {
  test.skip(!codexOnPath(), "codex CLI not on PATH — the design vision judge requires it");
  testInfo.setTimeout(12 * 60_000);

  await runPair({
    page,
    context,
    testInfo,
    label: "w15-story-page",
    baselineRoute: PROTO_BASELINE_ROUTES.story,
    baselineAnchors: ["mockup-canvas", "feature-strip", "yaml-editor"],
    prodRoute: (sid) => wsRoutes.story(sid, "60.1"),
    prodAnchors: ["rail", "sidebar", "file-view"],
    profile: STORY_PAGE_PROFILE,
    checkNames: STORY_PAGE_CHECK_NAMES,
  });
});

test("w15.3 feature detail conforms to the prototype (codex vision judge, R8)", async ({ page, context }, testInfo) => {
  test.skip(!codexOnPath(), "codex CLI not on PATH — the design vision judge requires it");
  testInfo.setTimeout(12 * 60_000);

  await runPair({
    page,
    context,
    testInfo,
    label: "w15-feature-detail",
    baselineRoute: PROTO_BASELINE_ROUTES.feature,
    baselineAnchors: [
      "mockup-canvas",
      "feature-stories-list",
      "feature-scenarios-list",
      "feature-unaligned-scenarios-list",
    ],
    prodRoute: (sid) => wsRoutes.feature(sid, "summaries"),
    prodAnchors: ["rail", "sidebar", "feature-stories-list"],
    profile: FEATURE_DETAIL_PROFILE,
    checkNames: FEATURE_DETAIL_CHECK_NAMES,
  });
});
