/**
 * w9 — design LAYOUT conformance via the local codex vision judge (task
 * `design-conformance-tests`, R2).
 *
 * Screenshots the design baseline (paths.yaml → design_system: the runnable
 * PROTOTYPE's `/workspace-overview` page, booted from the repo) and the
 * corresponding production workspace page at
 * the SAME 1440x900 desktop reference viewport (design/SPEC.md §6), then submits
 * both to `codex exec` with a schema-forced JSON rubric of concrete named layout
 * checks derived from design/SPEC.md §1 (persistent sidebar/breadcrumb/canvas
 * frame, sidebar fixed width, breadcrumb hairline border, canvas padding).
 * Content/data differences are explicitly excluded from the verdict.
 *
 * Skips cleanly with a NAMED reason when the `codex` CLI is unavailable (R3).
 * RED intent: the current production frame omits the persistent breadcrumb bar
 * the mockup mandates, so the judge returns `verdict: "fail"` with the failed
 * checks named — an assertion failure, not a crash.
 */

import path from "node:path";

import { expect, test } from "@playwright/test";

import {
  DESKTOP_VIEWPORT,
  REPO_ROOT,
  auditVerdict,
  codexOnPath,
  runDesignJudge,
} from "./helpers/design-conformance";
import { PROTO_BASELINE_ROUTES, bootPrototype, stopPrototype } from "./helpers/proto-baseline";
import { WTID, fileView, gotoWorkspace, wsRoutes } from "./helpers/ui";
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

test("w9.1 the production workspace frame conforms to the design mockup (codex vision judge, R2)", async ({
  page,
  context,
}, testInfo) => {
  test.skip(!codexOnPath(), "codex CLI not on PATH — the design vision judge requires it");
  testInfo.setTimeout(5 * 60_000);

  // ── Production screenshot at the desktop reference viewport ──
  env = await WorkspaceEnv.start("w9-design-judge");
  const e = env;
  await page.setViewportSize({ ...DESKTOP_VIEWPORT });
  await gotoWorkspace(page, e.baseURL, wsRoutes.architecture(e.sid));
  await expect(page.getByTestId(WTID.rail)).toBeVisible();
  await expect(page.getByTestId(WTID.sidebar)).toBeVisible();
  await expect(fileView(page)).toBeVisible();

  const prodPng = testInfo.outputPath("prod-workspace.png");
  // `font-display: swap` means the Poppins face can paint AFTER first layout;
  // screenshot the fallback and the judge sees the wrong typography. Settle the
  // web fonts before capturing so the comparison is deterministic (reviewer R1).
  await page.evaluate(async () => {
    await document.fonts.ready;
  });
  await page.screenshot({ path: prodPng });

  // ── Design-baseline screenshot (the booted prototype) at the same viewport ──
  const proto = await bootPrototype();
  const mock = await context.newPage();
  await mock.setViewportSize({ ...DESKTOP_VIEWPORT });
  await mock.goto(`${proto.baseURL}${PROTO_BASELINE_ROUTES.workspaceOverview}`, { waitUntil: "load" });
  await expect(mock.getByTestId("mockup-breadcrumb")).toBeVisible();
  const mockupPng = testInfo.outputPath("mockup-workspace.png");
  await mock.evaluate(async () => {
    await document.fonts.ready;
  });
  await mock.screenshot({ path: mockupPng });
  await mock.close();

  // ── Codex vision verdict ──
  // Under the Codex artifacts dir (paths.yaml → codex_artifacts, tmp/), but a
  // per-test label so concurrent/repeat runs never clobber one another's
  // verdict/trace (Codex r1 #9).
  const artifactDir = path.join(REPO_ROOT, "tmp", "design-judge");
  const { verdict, verdictPath, tracePath, exitCode } = await runDesignJudge({
    mockupPng,
    prodPng,
    artifactDir,
    label: `w9-design-judge-${testInfo.testId}`,
  });
  await testInfo.attach("codex-verdict", { path: verdictPath, contentType: "application/json" });
  await testInfo.attach("codex-trace", { path: tracePath, contentType: "text/plain" });

  // A nonzero judge exit means codex did not finish cleanly — any verdict it
  // wrote before erroring is untrustworthy and must NOT be accepted as green
  // (reviewer R1). A healthy run exits 0, so this never bites the happy path.
  expect(
    exitCode,
    `codex judge process exited nonzero (${exitCode}); its verdict is untrustworthy. trace: ${tracePath}`,
  ).toBe(0);

  // Runtime-validate the reply so a schema-valid-but-incomplete/contradictory
  // verdict (e.g. empty `checks`) cannot let a real deviation be born green.
  const { failedChecks, problems } = auditVerdict(verdict);
  expect(problems, `codex judge (exit ${exitCode}) returned a structurally invalid verdict:\n${problems.join("\n")}\nverdict: ${verdictPath}  trace: ${tracePath}`).toEqual([]);

  const report = failedChecks.map((c) => `${c.name}: ${c.note}`).join("\n");
  expect(
    failedChecks,
    `codex judge (exit ${exitCode}) reported design-conformance violations:\n${report}\n` +
      `verdict: ${verdictPath}  trace: ${tracePath}`,
  ).toEqual([]);
});
