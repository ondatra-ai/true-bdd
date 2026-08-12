/**
 * w9 — design LAYOUT conformance via the local codex vision judge (task
 * `design-conformance-tests`, R2).
 *
 * Compares the COMMITTED gold standard (tests/harness/goldens/
 * workspace-overview.png — captured from the prototype at test-authoring time
 * via `npm run goldens:update`) against a fresh screenshot of the production
 * workspace page at the SAME 1440x900 desktop reference viewport
 * (design/SPEC.md §6), then submits both to `codex exec` with a schema-forced
 * JSON rubric of concrete named layout checks derived from design/SPEC.md §1
 * (persistent sidebar/breadcrumb/canvas frame, sidebar fixed width,
 * breadcrumb hairline border, canvas padding). Content/data differences are
 * explicitly excluded from the verdict. A missing golden FAILS the spec with
 * the recapture command named — the prototype is never booted here.
 *
 * Skips cleanly with a NAMED reason when the `codex` CLI is unavailable (R3).
 * RED intent: the current production frame omits the persistent breadcrumb bar
 * the mockup mandates, so the judge returns `verdict: "fail"` with the failed
 * checks named — an assertion failure, not a crash.
 */

import { randomUUID } from "node:crypto";
import path from "node:path";

import { expect, test } from "@playwright/test";

import {
  DESKTOP_VIEWPORT,
  GOLDEN_NAMES,
  REPO_ROOT,
  auditVerdict,
  codexOnPath,
  requireGolden,
  runDesignJudge,
} from "./helpers/design-conformance";
import { WTID, fileView, gotoWorkspace, wsRoutes } from "./helpers/ui";
import { WorkspaceEnv } from "./helpers/workspace-env";

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

test("w9.1 the production workspace frame conforms to the design mockup (codex vision judge, R2)", async ({
  page,
}, testInfo) => {
  test.skip(!codexOnPath(), "codex CLI not on PATH — the design vision judge requires it");
  testInfo.setTimeout(5 * 60_000);

  // ── The committed gold standard (authoring-time capture) ──
  const mockupPng = requireGolden(GOLDEN_NAMES.workspaceOverview);

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

  // ── Codex vision verdict ──
  // Under the repo's gitignored runtime dir (tmp/), with a per-test label PLUS
  // a run-unique suffix: testId alone is stable across retries and repeated
  // invocations, so without the suffix a retry would clobber the previous
  // attempt's verdict/trace after they were already attached to the report.
  const artifactDir = path.join(REPO_ROOT, "tmp", "design-judge");
  const { verdict, verdictPath, tracePath, exitCode } = await runDesignJudge({
    mockupPng,
    prodPng,
    artifactDir,
    label: `w9-design-judge-${testInfo.testId}-${randomUUID().slice(0, 8)}`,
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
