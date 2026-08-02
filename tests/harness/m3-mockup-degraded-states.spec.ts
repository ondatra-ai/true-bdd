/**
 * m3 — degraded and flagged states the current contract renders.
 *
 * A design deliverable must show the representative NON-happy states, not just
 * green paths. Status/lifecycle vocabularies are the frozen sets from
 * helpers/README-testids.md (inventory chip statuses; lifecycle chip values).
 */

import { expect, test } from "@playwright/test";

import { INVENTORY_STATUSES, MOCKUP_TID, openMockup } from "./helpers/mockups";

test.describe("m3 mockup degraded states", () => {
  // ── m3.1 — the full frozen 7-status inventory chip vocabulary ──
  test("m3.1 inventory chip vocabulary — all seven frozen statuses", async ({ page }) => {
    await openMockup(page, "workspace-overview");
    for (const status of INVENTORY_STATUSES) {
      await expect(
        page.locator(`[data-testid="${MOCKUP_TID.inventoryChip}"][data-status="${status}"]`).first(),
        `inventory chip for status "${status}"`,
      ).toBeVisible();
    }
  });

  // ── m3.2 — inventory truncation banner ──
  test("m3.2 inventory truncation banner", async ({ page }) => {
    await openMockup(page, "workspace-overview");
    const banner = page.getByTestId(MOCKUP_TID.truncationBanner);
    await expect(banner).toBeVisible();
    await expect(banner).toContainText(/truncat|degraded|budget/i);
  });

  // ── m3.3 — 504 cli_timeout unavailable state + removed-session absence ──
  test("m3.3 unavailable.html shows an explicit 504 cli_timeout state", async ({ page }) => {
    await openMockup(page, "unavailable");
    await expect(page.getByText(/504/).first()).toBeVisible();
    await expect(page.getByText(/cli_timeout/i).first()).toBeVisible();
    await expect(page.getByText(/unavailable|timed out|timeout/i).first()).toBeVisible();
  });

  test("m3.3 sessions.html: connected rows present; removed session absent, no disconnected markers", async ({
    page,
  }) => {
    await openMockup(page, "sessions");
    // Positive first: ≥1 connected row with a test-connection control.
    await expect(page.getByTestId(MOCKUP_TID.sessionRow).first()).toBeVisible();
    await expect(page.getByRole("button", { name: /test connection/i }).first()).toBeVisible();
    // Specific absence: a documented "removed" session id never appears as a row.
    await expect(page.getByText("sess-removed-01")).toHaveCount(0);
    // Removed sessions simply disappear — no disconnected/unreachable markers.
    await expect(page.getByText(/disconnected|unreachable/i)).toHaveCount(0);
  });

  // ── m3.4 — folder-conflict + architecture path-mismatch warnings ──
  test("m3.4 folder-conflict + architecture path-mismatch warnings", async ({ page }) => {
    await openMockup(page, "workspace-overview");
    const folder = page.getByTestId(MOCKUP_TID.folderConflict);
    await expect(folder).toBeVisible();
    await expect(folder).toContainText(/folder|conflict|owner|sibling/i);
    const pathMismatch = page.getByTestId(MOCKUP_TID.pathMismatch);
    await expect(pathMismatch).toBeVisible();
    await expect(pathMismatch).toContainText(/path|architecture|mismatch/i);
  });

  // ── m3.5 — epic AND story identity flags (both categories), attached ──
  test("m3.5 epic page shows BOTH an epic-scoped and a story-scoped identity flag", async ({ page }) => {
    await openMockup(page, "epic");

    // The epic-scoped flag is ATTACHED to the epic header (which names the epic).
    const header = page.getByTestId(MOCKUP_TID.epicHeader);
    await expect(header).toContainText("Inventory Spread Epic");
    const epicFlag = header.getByTestId(MOCKUP_TID.epicFlag).first();
    await expect(epicFlag).toBeVisible();
    await expect(epicFlag).toContainText(/mismatch|duplicate|noncanonical|id/i);

    // The story-scoped flag is ATTACHED to a story row (which carries a story id).
    const row = page.getByTestId(MOCKUP_TID.storyRow).filter({ hasText: /60\.\d/ }).first();
    const storyFlag = row.getByTestId(MOCKUP_TID.storyFlag).first();
    await expect(storyFlag).toBeVisible();
    await expect(storyFlag).toContainText(/mismatch|no.?acs|deprecated|empty.?internal|duplicate|id/i);
  });

  // ── m3.6 — ambiguous story page with match count ──
  test("m3.6 story-ambiguous.html shows a match count of MORE THAN ONE file", async ({ page }) => {
    await openMockup(page, "story-ambiguous");
    const notice = page.getByText(/\d+\s+match(ing|es)?/i).first();
    await expect(notice).toBeVisible();
    // Ambiguity means >1 match — "0 matching" / "1 matching file" would be a
    // false-pass (final review, finding 9).
    const text = (await notice.textContent()) ?? "";
    const count = Number(/(\d+)\s+match/i.exec(text)?.[1] ?? "0");
    expect(count, `ambiguity match count > 1 (got "${text.trim()}")`).toBeGreaterThan(1);
  });

  // ── m3.7 — invalid story page with a parse error ──
  test("m3.7 story-invalid.html shows a parse-error banner + the raw file CONTENT", async ({ page }) => {
    await openMockup(page, "story-invalid");
    await expect(
      page.getByText(/parse error|could not parse|malformed|invalid/i).first(),
    ).toBeVisible();
    // The raw (unparseable) file CONTENT is present — a bare "Raw" label with
    // no file text would be a false-pass (final review, finding 9). The
    // signature line is the malformed `i_want` entry (missing colon).
    const raw = page.locator("pre").filter({ hasText: /i_want Claude to do the thing/ }).first();
    await expect(raw).toBeVisible();
  });

  // ── m3.8 — non-success run detail ──
  test("m3.8 run-detail.html shows a non-success outcome badge", async ({ page }) => {
    await openMockup(page, "run-detail");
    const outcome = page.getByTestId(MOCKUP_TID.runOutcome);
    await expect(outcome).toBeVisible();
    await expect(outcome).toContainText(/not_fixed|error|max_attempts|interrupted|abandoned|failed/i);
    await expect(outcome).not.toHaveText(/^\s*ok\s*$/i);
  });

  // ── m3.9 — lifecycle chip vocabulary ──
  test("m3.9 story-detail lifecycle chips: created / applied x/y|unknown / refined not recorded", async ({
    page,
  }) => {
    await openMockup(page, "story-detail");
    const chip = (kind: string) =>
      page.locator(`[data-testid="${MOCKUP_TID.lifecycleChip}"][data-kind="${kind}"]`).first();

    await expect(chip("created"), "a created lifecycle chip").toBeVisible();

    const applied = chip("applied");
    await expect(applied, "an applied lifecycle chip").toBeVisible();
    await expect(applied).toContainText(/(\d+\/\d+)|unknown/i);

    const refined = chip("refined");
    await expect(refined, "a refined lifecycle chip").toBeVisible();
    await expect(refined).toContainText(/not recorded/i);
  });
});
