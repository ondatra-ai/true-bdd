/**
 * m4 — the three CLI prompt dialog kinds as modal overlays over a run page.
 *
 * Each prompt page is a run-detail canvas with an OPEN native `<dialog open>`
 * overlay (role="dialog"), demonstrating "CLI prompts stay as modal dialogs over
 * any page". The dialogs render open with zero JS, so structure is asserted
 * statically (no reliance on a scripted open).
 */

import { expect, test } from "@playwright/test";

import { MOCKUP_TID, openMockup } from "./helpers/mockups";

function promptDialogOfKind(kind: string): string {
  return `[data-testid="${MOCKUP_TID.promptDialog}"][data-kind="${kind}"]`;
}

test.describe("m4 mockup prompt dialogs", () => {
  // ── m4.1 — choice dialog ──
  test("m4.1 choice dialog is titled and offers Apply / Refine / Exit", async ({ page }) => {
    await openMockup(page, "prompt-choice");
    const dialog = page.getByRole("dialog");
    await expect(dialog).toBeVisible();
    await expect(page.locator(promptDialogOfKind("choice"))).toHaveCount(1);
    // A visible dialog title marks the prompt.
    const title = dialog.getByRole("heading").first();
    await expect(title).toBeVisible();
    await expect(title).not.toHaveText(/^\s*$/);
    await expect(dialog.getByRole("button", { name: /apply/i })).toBeVisible();
    await expect(dialog.getByRole("button", { name: /refine/i })).toBeVisible();
    await expect(dialog.getByRole("button", { name: /exit/i })).toBeVisible();
  });

  // ── m4.2 — numbered clarify dialog ──
  test("m4.2 clarify dialog has sequentially numbered options + a single-line input", async ({ page }) => {
    await openMockup(page, "prompt-clarify");
    const dialog = page.getByRole("dialog");
    await expect(dialog).toBeVisible();
    await expect(page.locator(promptDialogOfKind("clarify"))).toHaveCount(1);
    const title = dialog.getByRole("heading").first();
    await expect(title).toBeVisible();
    await expect(title).not.toHaveText(/^\s*$/);

    // ≥2 options whose data-index is EXACTLY the sequence 1,2,…,N (no dupes/gaps/
    // fractions) and whose visible text surfaces that index.
    const options = dialog.locator("[data-index]");
    const count = await options.count();
    expect(count, "at least two numbered clarify options").toBeGreaterThanOrEqual(2);
    const indices: string[] = [];
    for (let i = 0; i < count; i++) {
      const idx = (await options.nth(i).getAttribute("data-index")) ?? "";
      indices.push(idx);
      await expect(options.nth(i), "the index is visible on the option").toContainText(idx);
    }
    const expectedSeq = Array.from({ length: count }, (_, i) => String(i + 1));
    expect(indices, "options are numbered 1..N in order").toEqual(expectedSeq);

    // A single-line answer input, NOT a multiline textarea.
    await expect(dialog.locator('input[type="text"], input:not([type])').first()).toBeVisible();
    await expect(dialog.locator("textarea")).toHaveCount(0);
  });

  // ── m4.3 — multiline freetext dialog ──
  test("m4.3 freetext dialog is titled and has a multiline textarea + submit", async ({ page }) => {
    await openMockup(page, "prompt-freetext");
    const dialog = page.getByRole("dialog");
    await expect(dialog).toBeVisible();
    await expect(page.locator(promptDialogOfKind("freetext"))).toHaveCount(1);
    const title = dialog.getByRole("heading").first();
    await expect(title).toBeVisible();
    await expect(title).not.toHaveText(/^\s*$/);
    await expect(dialog.locator("textarea").first()).toBeVisible();
    await expect(
      dialog.getByRole("button", { name: /submit|send|refine|answer/i }).first(),
    ).toBeVisible();
  });

  // ── m4.4 — the dialog overlays the run page ──
  for (const name of ["prompt-choice", "prompt-clarify", "prompt-freetext"]) {
    test(`m4.4 ${name}: the dialog overlays the underlying run-detail canvas`, async ({ page }) => {
      await openMockup(page, name);
      await expect(page.getByRole("dialog")).toBeVisible();
      // The underlying run content is present beneath the open dialog.
      await expect(page.getByTestId(MOCKUP_TID.canvas)).toBeVisible();
      await expect(page.getByTestId(MOCKUP_TID.runOutcome)).toBeVisible();
    });
  }
});
