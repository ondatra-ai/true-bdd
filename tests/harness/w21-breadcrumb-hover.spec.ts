/**
 * w21 — breadcrumb hover geometry (visual-sweep round 1, 2026-08-06). Pins ONE
 * finding against the current build:
 *
 *   - w21.1 (RED): `[content-breadcrumb-link]/hover-jiggle` — hovering a
 *     breadcrumb link re-weights the link text (`.ws-breadcrumb-link:hover`
 *     sets `font-weight: 650`), widening it; the wider text shoves the
 *     following "/" separator and every trailing crumb sideways, so a cursor
 *     pass along the trail wobbles the whole row. The pinned INTENT (from
 *     tmp/visual-sweep/20260806-crush-miniround/round-1/findings.md): hovering
 *     a breadcrumb link must not move ANY breadcrumb element's bounding box.
 *     Hover colour feedback is intended; geometry movement is not.
 *
 *   - w21.2 (GREEN guard, the finding's healthy counterpart): the hover
 *     restyle stays bound to the link-COLOUR token on the link
 *     (`--link-color-hover`), and the non-link current-page crumb is inert —
 *     hovering it changes neither colour nor geometry.
 *
 * Oracle: the hover defect is INPUT-driven geometry, so a
 * PerformanceObserver('layout-shift') (the w20 async-shift oracle) is the
 * WRONG tool here — it filters `hadRecentInput=true` entries, i.e. exactly the
 * hover shifts. The correct oracle is a direct before/after `boundingBox`
 * comparison of every breadcrumb child, settled with an `expect.poll`
 * stable-read wait (the cold Poppins font-swap reflow, w20.5, is a one-time
 * ~305ms width change that must settle before the baseline is captured).
 *
 * Probe target: `/sessions/<sid>/product` (the PRD file page). The defect is
 * page-agnostic — it is the GLOBAL `.ws-breadcrumb-link:hover { font-weight:
 * 650 }` rule (harness/src/app/globals.css) — so every page with a linkable
 * crumb reproduces it. The product trail `Sessions / Workspace overview /
 * prd.yaml` is chosen because "Workspace overview" is the LONGEST linkable
 * crumb: at the row font-size the font-weight 400→650 reflow widens it ~4px
 * (the visual-sweep probe measured 4.39px on this string), which clears the
 * ≤2px oracle tolerance. The shorter "Home" link the finding was first
 * noticed on (~1px widening) stays UNDER the tolerance and would false-green,
 * so it is intentionally not the probe target. The trail still gives every
 * observable wobble target: one hoverable link, a trailing "/" separator, and
 * a trailing current crumb. The breadcrumb contract uses the stable
 * `.ws-breadcrumb-link` / `.ws-breadcrumb-current` classes (no per-link
 * testid); locators are scoped to the `content-breadcrumb` testid container.
 *
 * Each test owns a fresh WorkspaceEnv container (cold font-swap on every run)
 * at the probe viewport 1440×900.
 */

import { expect, test, type Locator, type Page } from "@playwright/test";

import { WTID, cssVarRgb, gotoWorkspace, wsRoutes } from "./helpers/ui";
import { WorkspaceEnv } from "./helpers/workspace-env";

test.use({ viewport: { width: 1440, height: 900 } });

interface Box {
  x: number;
  y: number;
  width: number;
  height: number;
}

/** Reads a bounding box, throwing (not returning null) if the locator is gone. */
async function box(loc: Locator, label: string): Promise<Box> {
  const b = await loc.boundingBox();

  if (b === null) {
    throw new Error(`${label}: bounding box is null (element not rendered)`);
  }

  return b;
}

/** Computed `color` (rgb string) of the first matched element. */
async function colorOf(loc: Locator): Promise<string> {
  return loc.evaluate((el) => getComputedStyle(el).color);
}

/**
 * Waits until `loc`'s width stops moving between two reads ~150ms apart. The
 * cold Poppins→body font swap (w20.5) reflows breadcrumb text ~305ms after
 * first paint; measuring the baseline mid-swap would pin the swap, not the
 * hover defect.
 */
async function waitForGeometryToSettle(page: Page, loc: Locator, label: string): Promise<void> {
  await expect
    .poll(
      async () => {
        const first = (await loc.boundingBox())?.width ?? -1;
        await page.waitForTimeout(150);
        const second = (await loc.boundingBox())?.width ?? -1;

        return Math.abs(first - second);
      },
      { timeout: 10_000, message: `${label}: width settled (no cold font-swap reflow in progress)` },
    )
    .toBeLessThanOrEqual(0.5);
}

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

test("w21.1 hovering a breadcrumb link moves no breadcrumb element — geometry is hover-invariant", async ({
  page,
}) => {
  env = await WorkspaceEnv.start("w21-breadcrumb-hover");
  const e = env;

  await gotoWorkspace(page, e.baseURL, wsRoutes.product(e.sid));

  const bar = page.getByTestId(WTID.contentBreadcrumb);
  await expect(bar).toBeVisible();

  // Product trail = "Sessions" (link) / " / " / "Workspace overview" (link, the
  // longest linkable crumb) / " / " / "prd.yaml" (current). Hovering the long
  // link is what reliably drives the widening past the 2px tolerance.
  const link = bar.locator(".ws-breadcrumb-link", { hasText: "Workspace overview" });
  // The separator AFTER the hovered link (the 2nd of two on this trail).
  const sep = bar.getByTestId(WTID.breadcrumbSep).last();
  const current = bar.locator(".ws-breadcrumb-current");
  await expect(link).toBeVisible();
  await expect(sep).toBeVisible();
  await expect(current).toBeVisible();

  // Park the cursor on the content canvas (below the breadcrumb bar) so the
  // link is NOT hovered at baseline, then settle out the cold font-swap.
  await page.getByTestId(WTID.contentPane).hover();
  await waitForGeometryToSettle(page, link, "breadcrumb link baseline");

  const baseline = {
    link: await box(link, "breadcrumb link (baseline)"),
    sep: await box(sep, "breadcrumb separator (baseline)"),
    current: await box(current, "current-page crumb (baseline)"),
  };

  // Hover the link and let the :hover style apply (no CSS transition on the
  // rule, so a single rAF + short settle is enough).
  await link.hover();
  await page.waitForTimeout(150);

  const hovered = {
    link: await box(link, "breadcrumb link (hovered)"),
    sep: await box(sep, "breadcrumb separator (hovered)"),
    current: await box(current, "current-page crumb (hovered)"),
  };

  // INTENT: hover feedback must be COLOUR-ONLY. Hovering a breadcrumb link
  // must not move ANY breadcrumb element's bounding box — not the link's own
  // width (the re-weight that starts the wobble), nor the separator / trailing
  // crumb that the wider text shoves sideways. ≤2px tolerance matches the w20
  // visual-stability oracle; the defect moves the link ~4px and the trailing
  // crumbs proportionally.
  const linkGrowth = Math.abs(hovered.link.width - baseline.link.width);
  expect(
    linkGrowth,
    `hovered breadcrumb link width changed by ${linkGrowth}px — hover must be colour-only, not re-weight/resize the text`,
  ).toBeLessThanOrEqual(2);

  const sepShift = Math.abs(hovered.sep.x - baseline.sep.x);
  expect(
    sepShift,
    `the "/" separator slid ${sepShift}px sideways while the link was hovered — hovering a breadcrumb link must not move any trailing crumb`,
  ).toBeLessThanOrEqual(2);

  const currentShift = Math.abs(hovered.current.x - baseline.current.x);
  expect(
    currentShift,
    `the current-page crumb slid ${currentShift}px sideways while the link was hovered — hovering a breadcrumb link must not move any trailing crumb`,
  ).toBeLessThanOrEqual(2);
});

test("w21.2 hover restyle stays colour-token-bound on the link and the current crumb is inert (green guard)", async ({
  page,
}) => {
  env = await WorkspaceEnv.start("w21-breadcrumb-guard");
  const e = env;

  await gotoWorkspace(page, e.baseURL, wsRoutes.product(e.sid));

  const bar = page.getByTestId(WTID.contentBreadcrumb);
  await expect(bar).toBeVisible();

  const link = bar.locator(".ws-breadcrumb-link", { hasText: "Workspace overview" });
  const current = bar.locator(".ws-breadcrumb-current");
  await expect(link).toBeVisible();
  await expect(current).toBeVisible();

  await page.getByTestId(WTID.contentPane).hover();
  await waitForGeometryToSettle(page, link, "breadcrumb link baseline");

  const hoverColor = await cssVarRgb(page, "--link-color-hover");
  const primaryColor = await cssVarRgb(page, "--text-primary");

  // The link's hover restyle targets the link-COLOUR token (the intended
  // feedback channel) — the hovered link's computed colour resolves to
  // --link-color-hover. This is the healthy behaviour the fix must preserve.
  await link.hover();
  await page.waitForTimeout(150);
  const hoveredLinkColor = await colorOf(link);
  expect(hoveredLinkColor, "hovered breadcrumb link colour must resolve to --link-color-hover").toBe(hoverColor);

  // The current-page crumb is NOT a link: it has no :hover restyle. Hovering it
  // changes neither its colour (stays --text-primary) nor its geometry — the
  // non-link crumb must stay inert.
  await page.getByTestId(WTID.contentPane).hover(); // un-hover the link
  await page.waitForTimeout(150);

  const currentColorBefore = await colorOf(current);
  expect(currentColorBefore, "current-page crumb colour must be --text-primary at rest").toBe(primaryColor);
  const currentBoxBefore = await box(current, "current-page crumb (rest)");

  await current.hover();
  await page.waitForTimeout(150);

  const currentColorAfter = await colorOf(current);
  expect(currentColorAfter, "current-page crumb must have NO hover colour change (non-link crumbs are inert)").toBe(
    currentColorBefore,
  );

  const currentBoxAfter = await box(current, "current-page crumb (hovered)");
  const currentDrift =
    Math.abs(currentBoxAfter.x - currentBoxBefore.x) + Math.abs(currentBoxAfter.width - currentBoxBefore.width);
  expect(
    currentDrift,
    "current-page crumb geometry must be hover-inert (non-link crumbs have no hover effect)",
  ).toBeLessThanOrEqual(2);
});
