/**
 * w7 — design system consumption + app-shell scroll (P7, Established facts).
 *
 * w7.1a is the ClickUp parity matrix's still-uncovered testable behaviors (most
 * measured behaviors ride existing cases: caret split-click → w1.4; flyout
 * placement → w1.2; chat in-layout reflow → w6.1; edit-in-place styles → w2.2).
 * Trailing hover quick-actions and the floating edit toolbar are intentionally
 * adapted/deferred (the brief calls the toolbar optional).
 */

import { expect, test } from "@playwright/test";

import {
  WTID,
  gotoWorkspace,
  railItem,
  sidebarCaret,
  sidebarGroup,
  sidebarSection,
  toRgb,
  wsRoutes,
} from "./helpers/ui";
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

test("w7.1 a documented tokens.css token is applied to a real element and tracks a live :root mutation (P7)", async ({
  page,
}) => {
  env = await WorkspaceEnv.start("w7-tokens");
  const e = env;

  await gotoWorkspace(page, e.baseURL, wsRoutes.architecture(e.sid));
  const rail = page.getByTestId(WTID.rail);
  await expect(rail).toBeVisible();

  // The rail background RESOLVES from --surface-inverse (not a hardcoded literal).
  const tokenValue = (
    await page.evaluate(() => getComputedStyle(document.documentElement).getPropertyValue("--surface-inverse"))
  ).trim();
  expect(tokenValue).not.toBe(""); // the token mirror is actually consumed
  expect(await rail.evaluate((el) => getComputedStyle(el).backgroundColor)).toBe(await toRgb(page, tokenValue));

  // Live-mutate the token at :root → the rail follows (defeats a hardcoded literal; mirrors m1.6c).
  await page.evaluate(() => document.documentElement.style.setProperty("--surface-inverse", "rgb(1, 2, 3)"));
  await expect(rail).toHaveCSS("background-color", "rgb(1, 2, 3)");
});

test("w7.1a ClickUp parity: rest/hover caret + glyph, guide line, bottom utilities, flyout float, chat header/input, wide chat (P7)", async ({
  page,
}) => {
  env = await WorkspaceEnv.start("w7-parity");
  const e = env;
  await page.setViewportSize({ width: 1920, height: 1080 });

  await gotoWorkspace(page, e.baseURL, wsRoutes.product(e.sid));
  const stories = sidebarGroup(sidebarSection(page, "product"), "Stories");
  const caret = sidebarCaret(stories);

  // No caret at rest; icon slot swaps to a caret on hover; glyph direction.
  await expect(caret).toBeHidden();
  await stories.hover();
  await expect(caret).toBeVisible();
  await expect(caret).toHaveText(/▾/); // expanded
  await caret.click();
  await expect(caret).toHaveText(/▸/); // collapsed
  await caret.click(); // re-expand for the guide-line check
  await expect(stories.getByTestId(WTID.sidebarGuideLine)).toBeVisible();

  // Rail utility items pinned at the rail BOTTOM.
  const rail = page.getByTestId(WTID.rail);
  const rb = (await rail.boundingBox())!;
  const ub = (await page.getByTestId(WTID.railUtilities).boundingBox())!;
  expect(ub.y + ub.height).toBeGreaterThan(rb.y + rb.height * 0.6);

  // Flyout floats immediately right of the rail, over content.
  await railItem(page, "architecture").hover();
  const fb = (await page.getByTestId(WTID.railFlyout).boundingBox())!;
  expect(fb.x).toBeGreaterThanOrEqual(rb.x + rb.width - 2);

  // Chat: own header row (new-chat control) + input pinned at the panel bottom; opens at the
  // prototype default width (ChatDialog.js DEFAULT_WIDTH=380 — the design baseline supersedes the
  // pre-prototype ClickUp "~40% at 1920" measurement; see w20.2 / task workspace-visual-jank).
  await gotoWorkspace(page, e.baseURL, wsRoutes.architecture(e.sid));
  await page.getByTestId(WTID.chatDockToggle).click();
  const panel = page.getByTestId(WTID.chatDockPanel);
  await expect(panel.getByTestId(WTID.chatDockHeader).getByTestId(WTID.chatDockNew)).toBeVisible();
  const pb = (await panel.boundingBox())!;
  const ib = (await panel.getByTestId(WTID.chatDockInput).boundingBox())!;
  expect(ib.y).toBeGreaterThan(pb.y + pb.height * 0.5); // input pinned low
  expect(Math.abs(pb.width - 380)).toBeLessThanOrEqual(2); // prototype default width
});

test("w7.2 the file view editor + gutter use a monospace font while the body resolves to Poppins", async ({ page }) => {
  env = await WorkspaceEnv.start("w7-monospace");
  const e = env;

  await gotoWorkspace(page, e.baseURL, wsRoutes.architecture(e.sid));
  const editorFont = await page.getByTestId(WTID.fileViewEditor).evaluate((el) => getComputedStyle(el).fontFamily);
  const gutterFont = await page.getByTestId(WTID.fileViewGutter).evaluate((el) => getComputedStyle(el).fontFamily);
  const bodyFont = await page.evaluate(() => getComputedStyle(document.body).fontFamily);

  expect(editorFont.toLowerCase()).toMatch(/mono/);
  expect(gutterFont.toLowerCase()).toMatch(/mono/);
  expect(bodyFont.toLowerCase()).toMatch(/poppins/);
});

test("w7.3 app-shell caps scroll (content pane owns it); the right-edge chat toggle stays clickable (Established fact)", async ({
  page,
}) => {
  env = await WorkspaceEnv.start("w7-scroll");
  const e = env;

  await gotoWorkspace(page, e.baseURL, wsRoutes.architecture(e.sid));
  // Scroll a long file: the content pane owns the scroll, the BODY does not scroll.
  await page.getByTestId(WTID.contentPane).evaluate((el) => el.scrollTo(0, el.scrollHeight));
  const bodyOverflow = await page.evaluate(
    () => document.documentElement.scrollHeight - document.documentElement.clientHeight,
  );
  expect(bodyOverflow).toBeLessThanOrEqual(2);

  // The right-edge chat toggle is NOT hidden under the body scrollbar — a hard click.
  const toggle = page.getByTestId(WTID.chatDockToggle);
  await toggle.click();
  await expect(page.getByTestId(WTID.chatDockPanel)).toBeVisible();
});
