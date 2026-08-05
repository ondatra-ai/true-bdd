/**
 * w11 — workspace-overview CANVAS-PARITY judge pair (task
 * `workspace-overview-design-parity`, R6). The SECOND design-judge pair: it
 * REUSES the local codex vision judge (helpers/design-conformance.ts) with the
 * `CANVAS_PARITY_PROFILE` — a distinct named-check set (title_metadata,
 * actions_row, inventory_health, breadcrumb_trail) and a rubric that TOLERATES
 * the production icon rail + docked sidebar (exactly as w9's frame judge does)
 * and IGNORES all content/data values.
 *
 * Compares the COMMITTED gold standard (tests/harness/goldens/
 * workspace-overview.png — captured from the prototype at test-authoring time
 * via `npm run goldens:update`) against a fresh screenshot of the production
 * `/home` page at the SAME 1440x900 desktop reference viewport, then submits
 * both to `codex exec` for a schema-forced JSON verdict. A missing golden
 * FAILS the spec with the recapture command named — the prototype is never
 * booted here.
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
  GOLDEN_NAMES,
  REPO_ROOT,
  auditVerdict,
  codexOnPath,
  requireGolden,
  runDesignJudge,
} from "./helpers/design-conformance";
import { WTID, gotoWorkspace, wsRoutes } from "./helpers/ui";
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

test("w11.1 the production workspace-overview canvas conforms to the design mockup (codex vision judge, R6)", async ({
  page,
}, testInfo) => {
  test.skip(!codexOnPath(), "codex CLI not on PATH — the design vision judge requires it");
  testInfo.setTimeout(5 * 60_000);

  // ── The committed gold standard (authoring-time capture) ──
  const mockupPng = requireGolden(GOLDEN_NAMES.workspaceOverview);

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
  // The w20.1 anti-jump fix renders inventory rows immediately with reserved-space
  // "…" placeholder chips, then the live read fills each chip ~a second later.
  // Judge the SETTLED canvas the rubric was always about. NOTE: the skeleton pass
  // already renders data-status="unknown" (non-empty), so polling on a non-empty
  // data-status false-passes on the placeholder frame (reviewer Codex r1 #2). The
  // pending marker is the visible "…" TEXT — wait until EVERY chip shows a real
  // label so the judge never screenshots placeholder chips (mirrors w10's idiom).
  const chips = page.getByTestId(WTID.overviewInventoryChip);
  await expect(chips.first()).toBeVisible();
  await expect
    .poll(
      async () => {
        const texts = await chips.evaluateAll((els) => els.map((el) => (el.textContent ?? "").trim()));

        return texts.length > 0 && texts.every((t) => t.length > 0 && t !== "…");
      },
      { timeout: 30_000, message: "every overview-inventory-chip resolved to a live label (not the pending …) before capture" },
    )
    .toBe(true);

  const prodPng = testInfo.outputPath("prod-overview.png");
  // Settle the Poppins web font before capture so the comparison is deterministic.
  await page.evaluate(async () => {
    await document.fonts.ready;
  });
  await page.screenshot({ path: prodPng });

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
