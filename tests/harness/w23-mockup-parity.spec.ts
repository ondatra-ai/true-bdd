/**
 * w23 — workspace mockup design-parity (exploratory run 1). Pins FOUR
 * DESIGN-PARITY findings (missing / oversized / mis-styled / no-hover elements)
 * against the current build, one `test()` per finding:
 *
 *   - w23.1 (F1, RED): the Architecture file page renders NO page-header block
 *     (kicker + title + meta all absent) — it jumps straight to the file card,
 *     unlike every other file page (Product, Features) which pass the shared
 *     `header={{…}}` prop to FileView.
 *   - w23.2 (F2, RED): the Scenarios (`/product/scenarios`) and Builds
 *     (`/builds`) pages render a raw `<h1>` at the global 84px display size with
 *     NO kicker, instead of the shared 32px (`--fs-h3`) page-title + kicker used
 *     by every other page. A cross-page 32px invariant spans five routes.
 *   - w23.3 (F4, RED): the autosave `[save-state]` indicator renders as 14px
 *     lowercase plain text BELOW the file card, instead of the mockup's 11px
 *     UPPERCASE in-bar badge (inside `file-view-header`) with a leading
 *     `::before` dot.
 *   - w23.4 (F5, RED): the non-anchor sidebar rows (`scenario-row`, a `<div>`)
 *     and the "+ New story" `<button>` have NO hover affordance (no rule targets
 *     them, and the global `a:hover` only reaches anchors), and the primary
 *     "Refresh inventory" button's solid-variant hover is clobbered by the
 *     higher-specificity base `.ws-overview-btn:hover:not(:disabled)` (which
 *     re-asserts the at-rest solid values) — so it never inverts. The anchor-
 *     based `sidebar-group-name` / `feature-row` already inherit the global
 *     `a:hover` colour shift (asserted here too; they pass). The Home "Build
 *     tests" / "Build code" buttons DO invert on hover (green guard).
 *
 * F3 (home inventory-health `…`) is DEFERRED (backend/data-dependent, not a UI
 * defect) — NOT pinned here.
 *
 * Oracle: these are DESIGN-PARITY defects, so the oracle is user-visible VALUES
 * and PRESENCE — computed font-size / text-transform (`toHaveCSS`), `::before`
 * content (`getComputedStyle(::before)`), element presence, `boundingBox`
 * ordering, and before/after hover computed-style deltas. NOT PerformanceObserver
 * / layout-shift observers (wrong oracle for value/presence parity).
 *
 * Each test owns a fresh WorkspaceEnv container at the probe viewport 1440×900.
 * Every testid asserted below ALREADY exists in WTID (helpers/ui.ts). Green-guard
 * assertions run FIRST (and pass); the first failing assertion in each test is
 * the pinned defect, so the spec is validly RED against the current build.
 */

import { expect, test, type Locator, type Page } from "@playwright/test";

import {
  WTID,
  gotoWorkspace,
  saveState,
  sidebarCaret,
  sidebarGroup,
  sidebarSection,
  wsRoutes,
} from "./helpers/ui";
import { WorkspaceEnv } from "./helpers/workspace-env";

test.use({ viewport: { width: 1440, height: 900 } });

interface Box {
  x: number;
  y: number;
  width: number;
  height: number;
}

interface ComputedColors {
  color: string;
  backgroundColor: string;
}

/** Reads a bounding box, throwing (not returning null) if the locator is gone. */
async function box(loc: Locator, label: string): Promise<Box> {
  const b = await loc.boundingBox();
  if (b === null) {
    throw new Error(`${label}: bounding box is null (element not rendered)`);
  }
  return b;
}

/** Computed `color` + `backgroundColor` of the first matched element. */
async function readColors(loc: Locator): Promise<ComputedColors> {
  return loc.evaluate((el) => {
    const s = getComputedStyle(el);
    return { color: s.color, backgroundColor: s.backgroundColor };
  });
}

/**
 * Parks the cursor on the content pane (a neutral area, off every sidebar and
 * overview-action target) so the target is NOT hovered at baseline.
 */
async function parkCursor(page: Page): Promise<void> {
  await page.getByTestId(WTID.contentPane).hover();
  await page.waitForTimeout(150);
}

/**
 * Asserts hovering `target` produces a computed color-OR-background change (the
 * mockup's hover affordance). Reads the baseline with the cursor parked OFF the
 * target, then hovers and re-reads. `soft` ⇒ expect.soft so every RED target
 * reports; a hard expect (green guard) fails loudly if the affordance regresses.
 */
async function assertHoverAffordance(page: Page, target: Locator, label: string, soft: boolean): Promise<void> {
  await expect(target).toBeVisible();
  await parkCursor(page);
  const baseline = await readColors(target);
  await target.hover();
  await page.waitForTimeout(150);
  const hovered = await readColors(target);
  const changed = hovered.color !== baseline.color || hovered.backgroundColor !== baseline.backgroundColor;
  if (soft) {
    expect.soft(changed, `${label}: hover must change computed color or background-color`).toBe(true);
  } else {
    expect(changed, `${label}: hover must change computed color or background-color`).toBe(true);
  }
}

/**
 * Expands a sidebar group if collapsed so its child rows are visible for the
 * hover probes. The caret is hover-revealed, so the group is hovered first to
 * surface it (matches w1.4's interaction model).
 */
async function ensureGroupExpanded(group: Locator): Promise<void> {
  await group.hover();
  const caret = sidebarCaret(group);
  await expect(caret).toBeVisible();
  if ((await caret.getAttribute("data-expanded")) === "false") {
    await caret.click();
  }
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

// ── w23.1 (F1) ──────────────────────────────────────────────────────────────

test("w23.1 (F1) Architecture file page renders the shared kicker + title + meta header block", async ({
  page,
}) => {
  env = await WorkspaceEnv.start("w23-arch-title");
  const e = env;

  // GREEN GUARD (run FIRST, must PASS): on /product the shared page-header block
  // (kicker / title / meta) renders — proves the assertions below target the
  // shared FileView header pattern, not the architecture route alone.
  await gotoWorkspace(page, e.baseURL, wsRoutes.product(e.sid));
  await expect(page.getByTestId(WTID.fileViewKicker), "product page renders the kicker").toBeVisible();
  await expect(page.getByTestId(WTID.fileViewTitle), "product page renders the title").toBeVisible();
  await expect(page.getByTestId(WTID.fileViewMeta), "product page renders the meta").toBeVisible();

  // PINNED DEFECT: on /architecture the SAME header block is absent (FileView is
  // rendered WITHOUT the `header` prop). The first failing assertion is the
  // absent kicker (toBeVisible on a zero-count locator).
  await gotoWorkspace(page, e.baseURL, wsRoutes.architecture(e.sid));
  const kicker = page.getByTestId(WTID.fileViewKicker);
  const title = page.getByTestId(WTID.fileViewTitle);
  const meta = page.getByTestId(WTID.fileViewMeta);
  await expect(kicker, "architecture page renders the kicker").toBeVisible();
  await expect(title, "architecture page renders the title").toBeVisible();
  await expect(meta, "architecture page renders the meta").toBeVisible();
  await expect(title, "architecture title is the filename").toHaveText("architecture.yaml");
  await expect(title, "architecture title is the 32px (--fs-h3) page-title size").toHaveCSS("font-size", "32px");
  await expect(kicker, "architecture kicker names the section").toContainText(/Architecture/i);
  // The title block sits ABOVE the file card's header bar.
  const titleBox = await box(title, "file-view-title");
  const headerBox = await box(page.getByTestId(WTID.fileViewHeader), "file-view-header");
  expect(titleBox.y, "file-view-title sits ABOVE the file-view-header bar").toBeLessThan(headerBox.y);
});

// ── w23.2 (F2) ──────────────────────────────────────────────────────────────

test("w23.2 (F2) Scenarios & Builds page titles are 32px with a kicker (not 84px, not bare)", async ({ page }) => {
  env = await WorkspaceEnv.start("w23-titles-kicker");
  const e = env;

  // GREEN GUARD (passes): Builds keeps a non-empty description paragraph under
  // its header (already present in prod).
  await gotoWorkspace(page, e.baseURL, wsRoutes.builds(e.sid));
  await expect(page.locator("h1").first(), "builds renders a primary h1").toBeVisible();
  const buildsDesc = page.getByTestId(WTID.buildsLanding).locator("p");
  await expect(buildsDesc, "builds keeps a non-empty description paragraph").toBeVisible();
  await expect(buildsDesc, "builds description is non-empty").not.toBeEmpty();

  // Cross-page 32px invariant — expect.soft, one labelled assertion per route so
  // EVERY route reports. The file pages use `file-view-title`; scenarios/builds
  // use the lone page `h1` (both pages render exactly one h1).
  const titleRoutes: Array<{ name: string; route: string; titleKind: "file" | "h1" }> = [
    { name: "product", route: wsRoutes.product(e.sid), titleKind: "file" },
    { name: "features", route: wsRoutes.features(e.sid), titleKind: "file" },
    { name: "architecture", route: wsRoutes.architecture(e.sid), titleKind: "file" },
    { name: "scenarios", route: wsRoutes.scenarios(e.sid), titleKind: "h1" },
    { name: "builds", route: wsRoutes.builds(e.sid), titleKind: "h1" },
  ];
  for (const { name, route, titleKind } of titleRoutes) {
    await gotoWorkspace(page, e.baseURL, route);
    const title = titleKind === "file" ? page.getByTestId(WTID.fileViewTitle) : page.locator("h1").first();
    await expect
      .soft(title, `[${name}] page primary title computed font-size is 32px (--fs-h3)`)
      .toHaveCSS("font-size", "32px");
  }

  // PINNED DEFECT (hard): the scenarios page `h1` is 32px (currently 84px) and a
  // kicker element sits ABOVE it (currently absent).
  await gotoWorkspace(page, e.baseURL, wsRoutes.scenarios(e.sid));
  const scenariosH1 = page.locator("h1").first();
  await expect(scenariosH1, "scenarios h1 computed font-size is 32px (not 84px)").toHaveCSS("font-size", "32px");
  const scenariosKicker = page.getByTestId(WTID.fileViewKicker);
  await expect(scenariosKicker, "scenarios page renders a kicker above the h1").toBeVisible();
  let kBox = await box(scenariosKicker, "scenarios kicker");
  let hBox = await box(scenariosH1, "scenarios h1");
  expect(kBox.y, "scenarios kicker sits ABOVE the h1").toBeLessThan(hBox.y);

  // PINNED DEFECT (hard): the builds page `h1` is 32px (currently 84px) and a
  // kicker element sits ABOVE it (currently absent).
  await gotoWorkspace(page, e.baseURL, wsRoutes.builds(e.sid));
  const buildsH1 = page.locator("h1").first();
  await expect(buildsH1, "builds h1 computed font-size is 32px (not 84px)").toHaveCSS("font-size", "32px");
  const buildsKicker = page.getByTestId(WTID.fileViewKicker);
  await expect(buildsKicker, "builds page renders a kicker above the h1").toBeVisible();
  kBox = await box(buildsKicker, "builds kicker");
  hBox = await box(buildsH1, "builds h1");
  expect(kBox.y, "builds kicker sits ABOVE the h1").toBeLessThan(hBox.y);
});

// ── w23.3 (F4) ──────────────────────────────────────────────────────────────

test("w23.3 (F4) Autosave save-state is an in-bar 11px UPPERCASE badge with a leading dot (not stray text below the card)", async ({
  page,
}) => {
  env = await WorkspaceEnv.start("w23-autosave-badge");
  const e = env;

  await gotoWorkspace(page, e.baseURL, wsRoutes.product(e.sid));

  const save = saveState(page);
  const header = page.getByTestId(WTID.fileViewHeader);
  const editor = page.getByTestId(WTID.fileViewEditor);

  // GREEN invariant (must stay true): save-state still reflects the idle state
  // at load (loadDoc sets saveState "idle" after the initial doc_read).
  await expect(save).toBeVisible();
  await expect(save, "save-state carries data-save-state=idle at load").toHaveAttribute("data-save-state", "idle");

  // PINNED DEFECT — placement: the indicator sits INSIDE / aligned to the
  // file-card header bar (its vertical CENTER within the header bar's span) and
  // ABOVE the editor body. Currently it renders BELOW the card.
  const saveBox = await box(save, "save-state");
  const headerBox = await box(header, "file-view-header");
  const editorBox = await box(editor, "file-view-editor");
  const saveCenterY = saveBox.y + saveBox.height / 2;
  expect(saveCenterY, "save-state center sits within the file-card header bar's vertical span").toBeGreaterThanOrEqual(
    headerBox.y,
  );
  expect(saveCenterY, "save-state center sits within the file-card header bar's vertical span").toBeLessThanOrEqual(
    headerBox.y + headerBox.height,
  );
  expect(saveBox.y, "save-state sits ABOVE the editor body").toBeLessThan(editorBox.y);

  // PINNED DEFECT — style: an 11px UPPERCASE badge with a leading `::before`
  // dot. The current `.ws-save-state` is 14px (--fs-caption), no text-transform,
  // no `::before`.
  await expect(save, "save-state font-size is 11px (--wk-fs-micro)").toHaveCSS("font-size", "11px");
  await expect(save, "save-state text is UPPERCASE").toHaveCSS("text-transform", "uppercase");
  // The mockup's dot is a generated `::before` pseudo (`content: ""` in
  // proto-extra.css `.file-view__status::before`); the meaningful pin is that a
  // `::before` IS generated (computed content !== "none"), not that the content
  // literal is non-empty — `content: ""` resolves to the empty string.
  const before = await save.evaluate((el) => getComputedStyle(el, "::before").content);
  expect(before, "save-state generates a ::before pseudo-element (the leading dot)").not.toBe("none");
});

// ── w23.4 (F5) ──────────────────────────────────────────────────────────────

test("w23.4 (F5) Sidebar links, +New story, and Refresh inventory show a hover affordance", async ({ page }) => {
  env = await WorkspaceEnv.start("w23-hover-affordance");
  const e = env;

  // GREEN GUARD (must PASS): on /home the Build tests / Build code buttons invert
  // on hover (background + color invert via `.ws-overview-btn:hover`).
  await gotoWorkspace(page, e.baseURL, wsRoutes.home(e.sid));
  await expect(page.getByTestId(WTID.overviewActionBuildTests)).toBeVisible();
  await expect(page.getByTestId(WTID.overviewActionBuildCode)).toBeVisible();
  await assertHoverAffordance(page, page.getByTestId(WTID.overviewActionBuildTests), "overview-action-build-tests", false);
  await assertHoverAffordance(page, page.getByTestId(WTID.overviewActionBuildCode), "overview-action-build-code", false);

  // RED target (on /home): the primary "Refresh inventory" button must invert on
  // hover. The solid variant's own `:hover` is clobbered by the higher-
  // specificity base `.ws-overview-btn:hover:not(:disabled)` re-asserting the
  // at-rest solid values, so no inversion occurs. Soft so it reports alongside
  // the sidebar targets.
  await assertHoverAffordance(page, page.getByTestId(WTID.overviewActionRefresh), "overview-action-refresh", true);

  // Sidebar hover-affordance sweep (on /product). Expand the Features + Scenarios
  // groups first so their child rows are visible for the hover probes. The
  // anchor-based `sidebar-group-name` and `feature-row` already inherit the
  // global `a:hover` colour shift (they pass); the `<div>` `scenario-row` and the
  // `<button>` `new-story-open` have NO hover rule and stay inert (the real
  // defects). Every target is soft so all report.
  await gotoWorkspace(page, e.baseURL, wsRoutes.product(e.sid));
  const product = sidebarSection(page, "product");
  await ensureGroupExpanded(sidebarGroup(product, "Features"));
  await ensureGroupExpanded(sidebarGroup(product, "Scenarios"));

  await assertHoverAffordance(page, page.getByTestId(WTID.sidebarGroupName).first(), "sidebar-group-name", true);
  await assertHoverAffordance(page, page.getByTestId(WTID.featureRow).first(), "feature-row", true);
  await assertHoverAffordance(page, page.getByTestId(WTID.scenarioRow).first(), "scenario-row", true);
  await assertHoverAffordance(page, page.getByTestId(WTID.newStoryOpen), "new-story-open", true);
});
