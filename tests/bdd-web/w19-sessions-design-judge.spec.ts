/**
 * w19 — sessions-home holistic design parity via the local codex vision judge
 * (task `home-sessions-list`, P7 holistic backup to the deterministic w18.10
 * gate). Boots the design-truth PROTOTYPE live (`bootPrototype()`), screenshots
 * its `/sessions` baseline, renders production `/` with one live session at the
 * desktop reference viewport, and submits both to `codex exec` with a
 * schema-forced JSON rubric (`SESSIONS_PARITY_PROFILE`) whose named checks are
 * `top_bar` / `wordmark_tagline` / `page_heading` / `row_list_anatomy`. The rubric
 * TOLERATES the prototype's Test-connection button and IGNORES all data/text
 * values.
 *
 * Skip discipline (mirrors w9/w11/w15): the ONLY authorized skip is `codex`
 * absent. EVERY other failure (prototype boot/readiness, screenshot, judge exit,
 * a structurally invalid verdict) is a TEST FAILURE with captured artifacts —
 * never a skip, never a hang. The retained `PrototypeServer` is stopped in
 * `afterAll` via `stopPrototype()` so the child is never leaked.
 *
 * RED intent: production `/` is the behavior-free placeholder, so the judge sees
 * no gradient top bar, no wordmark/tagline, no "Sessions" header, and no session
 * row list — it returns `verdict:"fail"` and the failed-checks assertion fails.
 */

import path from "node:path";

import { expect, test } from "@playwright/test";

import {
  DESKTOP_VIEWPORT,
  REPO_ROOT,
  SESSIONS_PARITY_CHECK_NAMES,
  SESSIONS_PARITY_PROFILE,
  auditVerdict,
  codexOnPath,
  runDesignJudge,
} from "./helpers/design-conformance";
import { PROTO_BASELINE_ROUTES, bootPrototype, stopPrototype } from "./helpers/proto-baseline";
import { gotoSessions, sessionRow } from "./helpers/ui";
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

test.afterAll(async () => {
  // The prototype is a module-level singleton shared by the design-judge suite;
  // stop it here so its child process tree is never leaked.
  await stopPrototype();
});

test("w19.1 the sessions home conforms to the prototype /sessions (codex vision judge, P7)", async ({
  page,
}, testInfo) => {
  test.skip(!codexOnPath(), "codex CLI not on PATH — the design vision judge requires it");
  // The proto helper's 180s readiness budget + up-to-360s `npm ci` exceed the
  // 3-min workspace default — mirror w15's 12-min per-test budget BEFORE booting.
  testInfo.setTimeout(12 * 60_000);

  await page.setViewportSize({ ...DESKTOP_VIEWPORT });

  // ── Baseline: the live-booted prototype's /sessions page ──
  const proto = await bootPrototype();
  await page.goto(`${proto.baseURL}${PROTO_BASELINE_ROUTES.sessions}`, { waitUntil: "load" });
  await page.evaluate(async () => {
    await document.fonts.ready;
  });
  const baselinePng = testInfo.outputPath("baseline-sessions.png");
  await page.screenshot({ path: baselinePng });

  // ── Production: `/` with one live session at the desktop viewport ──
  env = await WorkspaceEnv.start("w19-sessions-judge");
  const e = env;
  await gotoSessions(page, e.baseURL);
  // REQUIRE the live row before screenshotting (same 30s budget the deterministic
  // suite gives the initial row): a 5s best-effort could hand the vision judge a
  // half-rendered loading frame on slow CI and yield a MISLEADING design "fail". A
  // RED placeholder still fails — deterministically here, as a visibility timeout,
  // rather than as an unreliable vision verdict downstream.
  await expect(sessionRow(page, e.sid), "the production sessions row must render before the parity screenshot").toBeVisible(
    { timeout: 30_000 },
  );
  await page.evaluate(async () => {
    await document.fonts.ready;
  });
  const prodPng = testInfo.outputPath("prod-sessions.png");
  await page.screenshot({ path: prodPng });

  // ── Codex vision verdict ──
  const { verdict, verdictPath, tracePath, exitCode } = await runDesignJudge({
    mockupPng: baselinePng,
    prodPng,
    artifactDir: ARTIFACT_DIR,
    profile: SESSIONS_PARITY_PROFILE,
    label: `w19-sessions-${testInfo.testId}`,
  });
  await testInfo.attach("codex-verdict", { path: verdictPath, contentType: "application/json" });
  await testInfo.attach("codex-trace", { path: tracePath, contentType: "text/plain" });

  expect(
    exitCode,
    `codex judge process exited nonzero (${exitCode}); its verdict is untrustworthy. trace: ${tracePath}`,
  ).toBe(0);

  const { failedChecks, problems } = auditVerdict(verdict, SESSIONS_PARITY_CHECK_NAMES);
  expect(
    problems,
    `codex judge (exit ${exitCode}) returned a structurally invalid verdict:\n${problems.join("\n")}\n` +
      `verdict: ${verdictPath}  trace: ${tracePath}`,
  ).toEqual([]);

  const report = failedChecks.map((c) => `${c.name}: ${c.note}`).join("\n");
  expect(
    failedChecks,
    `codex judge (exit ${exitCode}) reported sessions-parity violations:\n${report}\n` +
      `verdict: ${verdictPath}  trace: ${tracePath}`,
  ).toEqual([]);
});
