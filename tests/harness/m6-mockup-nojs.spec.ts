/**
 * m6 — no-JS smoke (final review, finding 7).
 *
 * The plan (§Implementation B) requires every mockup page to render correctly
 * with JavaScript disabled: prompt dialogs are open via `<dialog open>`,
 * sidebar trees and the raw-file view are native `<details>/<summary>`,
 * navigation is plain `<a href>`, and `assets/mockups.js` is a non-essential
 * nicety. This spec re-proves the load-bearing signatures with JavaScript OFF
 * (native <details> toggling and link navigation are browser behaviors, not
 * scripts, so they must keep working here).
 */
import { expect, test } from "@playwright/test";

import { MOCKUP_TID, openMockup, sidebarSectionId } from "./helpers/mockups";

test.use({ javaScriptEnabled: false });

test.describe("m6 mockups render with JavaScript disabled", () => {
  test("m6.1 story-detail: frame, statement, AC block, and the native raw disclosure", async ({ page }) => {
    await openMockup(page, "story-detail");
    await expect(page.getByTestId(MOCKUP_TID.sidebar)).toBeVisible();
    await expect(page.getByTestId(MOCKUP_TID.breadcrumb)).toBeVisible();
    await expect(page.getByTestId(MOCKUP_TID.canvas)).toBeVisible();
    await expect(page.getByText("60.2").first()).toBeVisible();
    await expect(
      page.getByText(/short summary of a shared\s+Google Doc/i).first(),
    ).toBeVisible();
    await expect(
      page.locator(".ac-block").filter({ hasText: /at least 500 words of body text/i }).first(),
    ).toBeVisible();

    // The raw-file view opens through the NATIVE disclosure, no script needed.
    const raw = page.getByTestId(MOCKUP_TID.storyRaw);
    await expect(raw).toBeHidden();
    await page.locator(".raw-details > summary").first().click();
    await expect(raw).toBeVisible();
  });

  test("m6.2 prompt-choice: the dialog is open with its three controls", async ({ page }) => {
    await openMockup(page, "prompt-choice");
    const dialog = page.getByRole("dialog");
    await expect(dialog).toBeVisible();
    for (const name of [/apply/i, /refine/i, /exit/i]) {
      await expect(dialog.getByRole("button", { name }).first()).toBeVisible();
    }
  });

  test("m6.3 workspace-overview: sidebar sections expand natively without JS", async ({ page }) => {
    await openMockup(page, "workspace-overview");
    const product = page.getByTestId(sidebarSectionId("product"));
    const epicLink = product.locator('a[href$="epic.html"]').first();
    await expect(epicLink).toBeHidden();
    await product.locator("summary").first().click();
    await expect(epicLink).toBeVisible();
  });
});
