/**
 * w11 — workspace-overview CANVAS-PARITY judge pair (task
 * `workspace-overview-design-parity`, R6). The SECOND design-judge pair: it
 * REUSES the local codex vision judge (helpers/design-conformance.ts) with the
 * `CANVAS_PARITY_PROFILE` — a distinct named-check set (title_metadata,
 * actions_row, inventory_health, breadcrumb_trail) and a rubric that TOLERATES
 * the production icon rail + docked sidebar (exactly as w9's frame judge does)
 * and IGNORES all content/data values.
 *
 * Screenshots the design baseline (the runnable PROTOTYPE's `/workspace-overview`
 * page, booted from the repo — it replaced the deleted static mockups) and the
 * production `/home` page at the SAME 1440x900 desktop reference viewport, then
 * submits both to `codex exec` for a schema-forced JSON verdict.
 *
 * Skips cleanly with a NAMED reason when the `codex` CLI is unavailable.
 * RED intent: the current `/home` is a stub, so the judge returns
 * `verdict: "fail"` with the missing canvas surfaces named — an assertion
 * failure, not a crash. Once the canvas reaches parity this becomes the
 * permanent workspace-overview visual gate.
 */

import path from "node:path";

import { expect, test } from "@playwright/test";

import {
  CANVAS_PARITY_CHECK_NAMES,
  CANVAS_PARITY_PROFILE,
  DESKTOP_VIEWPORT,
  REPO_ROOT,
  auditVerdict,
  codexOnPath,
  runDesignJudge,
} from "./helpers/design-conformance";
import { PROTO_BASELINE_ROUTES, bootPrototype, stopPrototype } from "./helpers/proto-baseline";
import { WTID, gotoWorkspace, wsRoutes } from "./helpers/ui";
import { WorkspaceEnv } from "./helpers/workspace-env";

let env: WorkspaceEnv | undefined;

test.afterAll(async () => {
  await stopPrototype();
});

test.afterEach(async () => {
  const info = test.info();
  info.setTimeout(info.timeout + 60_000);
  const current = env;
  env = undefined;
  if (current !== undefined) {
    await current.teardown(test.info());
  }
});

test("w11.1 the production workspace-overview canvas conforms to the design mockup (codex vision judge, R6)", async ({
  page,
  context,
}, testInfo) => {
  test.skip(!codexOnPath(), "codex CLI not on PATH — the design vision judge requires it");
  testInfo.setTimeout(5 * 60_000);

  // ── Production screenshot at the desktop reference viewport ──
  env = await WorkspaceEnv.start("w11-overview-judge");
  const e = env;
  await page.setViewportSize({ ...DESKTOP_VIEWPORT });
  await gotoWorkspace(page, e.baseURL, wsRoutes.home(e.sid));
  // The persistent shell is the readiness signal (as in w9) — never gate on the
  // session-detail relay route (part of what this task adds). The overview reads
  // its own live data on mount once implemented.
  await expect(page.getByTestId(WTID.rail)).toBeVisible();
  await expect(page.getByTestId(WTID.sidebar)).toBeVisible();
  await expect(page.getByTestId(WTID.contentBreadcrumb)).toBeVisible();
  // Also require the overview CANVAS itself to have rendered before capture, so the
  // judge never screenshots a pre-mount frame (reviewer Codex r1 #5). The inventory
  // list renders its skeleton shape synchronously on mount — no wait for the live
  // read — and the judge tolerates data values, so this stays deterministic.
  await expect(page.getByTestId(WTID.overviewTitle)).toBeVisible();
  await expect(page.getByTestId(WTID.overviewActions)).toBeVisible();
  await expect(page.getByTestId(WTID.overviewInventory)).toBeVisible();

  const prodPng = testInfo.outputPath("prod-overview.png");
  // Settle the Poppins web font before capture so the comparison is deterministic.
  await page.evaluate(async () => {
    await document.fonts.ready;
  });
  await page.screenshot({ path: prodPng });

  // ── Design-baseline screenshot (the booted prototype) at the same viewport ──
  const proto = await bootPrototype();
  const mock = await context.newPage();
  await mock.setViewportSize({ ...DESKTOP_VIEWPORT });
  await mock.goto(`${proto.baseURL}${PROTO_BASELINE_ROUTES.workspaceOverview}`, { waitUntil: "load" });
  await expect(mock.getByTestId("mockup-canvas")).toBeVisible();
  const mockupPng = testInfo.outputPath("mockup-overview.png");
  await mock.evaluate(async () => {
    await document.fonts.ready;
  });
  await mock.screenshot({ path: mockupPng });
  await mock.close();

  // ── Codex vision verdict (canvas-parity profile) ──
  const artifactDir = path.join(REPO_ROOT, "tmp", "design-judge");
  const { verdict, verdictPath, tracePath, exitCode } = await runDesignJudge({
    mockupPng,
    prodPng,
    artifactDir,
    profile: CANVAS_PARITY_PROFILE,
    label: `w11-overview-judge-${testInfo.testId}`,
  });
  await testInfo.attach("codex-verdict", { path: verdictPath, contentType: "application/json" });
  await testInfo.attach("codex-trace", { path: tracePath, contentType: "text/plain" });

  // A nonzero judge exit means codex did not finish cleanly — any verdict it
  // wrote before erroring is untrustworthy and must NOT be accepted as green.
  expect(
    exitCode,
    `codex judge process exited nonzero (${exitCode}); its verdict is untrustworthy. trace: ${tracePath}`,
  ).toBe(0);

  // Runtime-validate the reply against the canvas-parity check set so a
  // schema-valid-but-incomplete verdict cannot let a missing surface be green.
  const { failedChecks, problems } = auditVerdict(verdict, CANVAS_PARITY_CHECK_NAMES);
  expect(
    problems,
    `codex judge (exit ${exitCode}) returned a structurally invalid verdict:\n${problems.join("\n")}\n` +
      `verdict: ${verdictPath}  trace: ${tracePath}`,
  ).toEqual([]);

  const report = failedChecks.map((c) => `${c.name}: ${c.note}`).join("\n");
  expect(
    failedChecks,
    `codex judge (exit ${exitCode}) reported canvas-parity violations:\n${report}\n` +
      `verdict: ${verdictPath}  trace: ${tracePath}`,
  ).toEqual([]);
});
