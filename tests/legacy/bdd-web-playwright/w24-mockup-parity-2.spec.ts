/**
 * w24 — workspace mockup design-parity (exploratory run 2). Pins FOUR
 * DESIGN-PARITY findings (unstyled controls / missing placeholders / drifted
 * kicker text / missing background-fill hover) against the current build, one
 * `test()` per finding:
 *
 *   - w24.1 (G1, RED): the "+ New story" form's submit renders as bare text
 *     (`background: transparent` + `border-style: none`), there is NO
 *     Cancel/dismiss control, and the title input `placeholder` is EMPTY. The
 *     mockup ships a FILLED primary button + an outlined CANCEL button + a
 *     "Story title…" placeholder.
 *   - w24.2 (G2, RED): the chat dock Send button renders as bare text
 *     (`background: transparent`) and the input `placeholder` is EMPTY. The
 *     mockup ships a FILLED dark SEND button and a "Try an edit request…"
 *     placeholder. (The empty conversation body and the "+ New" header control
 *     are LEGITIMATE — green guards, not defects.)
 *   - w24.3 (F2b, RED): the `/product/scenarios` page kicker text drifted to
 *     "03—Requirements" — a non-existent section. The mockup (and the app's
 *     section numbering: Architecture=01, Product=02, Builds=04) labels it
 *     "02—Product / Scenarios".
 *   - w24.4 (F5b, RED): the anchor sidebar `feature-row` only inherits the
 *     global `a:hover` COLOR shift on hover — its `background-color` never
 *     changes, unlike the run-1 div rows (`scenario-row`) which gained a
 *     `--surface-subtle` bg hover. (The scenario-row bg hover is the green
 *     guard.)
 *
 * Oracle: these are DESIGN-PARITY defects, so the oracle is user-visible VALUES
 * and PRESENCE — computed background-color / border (styled-button affordance),
 * the input `placeholder` attribute, kicker text value, element presence, and
 * before/after hover computed-backgroundColor DELTAS. NOT PerformanceObserver /
 * layout-shift observers (wrong oracle for value/presence parity).
 *
 * Each test owns a fresh WorkspaceEnv container at the probe viewport 1440×900.
 * Green-guard assertions run FIRST (HARD, must pass — they prove the assertion
 * targets the right surface); the pinned-defect facets that follow use
 * expect.soft so every facet reports. The spec is validly RED when a pinned
 * facet fails against the current build while every green guard passes. The only
 * NEW testid introduced is `new-story-cancel` (w24.1), registered in WTID and
 * README-testids.md as a contract-only addition.
 */

import { expect, test, type Locator, type Page } from "@playwright/test";

import {
  WTID,
  gotoWorkspace,
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
 * Asserts hovering `target` produces a computed BACKGROUND-COLOR change (the
 * mockup's background-fill hover affordance — distinct from w23's color-OR-
 * background helper). Reads the baseline backgroundColor with the cursor parked
 * OFF the target, then hovers and re-reads. `soft` ⇒ expect.soft so every RED
 * target reports; a hard expect (green guard) fails loudly if the affordance
 * regresses.
 */
async function assertBackgroundHoverChange(
  page: Page,
  target: Locator,
  label: string,
  soft: boolean,
): Promise<void> {
  await expect(target).toBeVisible();
  await parkCursor(page);
  const baseline = await target.evaluate((el) => getComputedStyle(el).backgroundColor);
  await target.hover();
  await page.waitForTimeout(150);
  const hovered = await target.evaluate((el) => getComputedStyle(el).backgroundColor);
  const changed = hovered !== baseline;
  if (soft) {
    expect.soft(changed, `${label}: hover must change computed background-color`).toBe(true);
  } else {
    expect(changed, `${label}: hover must change computed background-color`).toBe(true);
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

// ── w24.1 (G1) ──────────────────────────────────────────────────────────────

test("w24.1 (G1) New-story form: styled submit, a Cancel control, and a non-empty title placeholder", async ({
  page,
}) => {
  env = await WorkspaceEnv.start("w24-new-story-form");
  const e = env;

  await gotoWorkspace(page, e.baseURL, wsRoutes.product(e.sid));

  // Open the "+ New story" form.
  await page.getByTestId(WTID.newStoryOpen).click();
  const form = page.getByTestId(WTID.newStoryForm);

  // GREEN GUARD (must PASS): the form opened and is visible — proves the
  // assertions below target the new-story form surface.
  await expect(form, "new-story form is visible after opening").toBeVisible();

  // PINNED DEFECT (soft, each facet reports):
  //  1. new-story-submit is a STYLED button affordance (filled bg OR visible
  //     border), not bare text. Prod: transparent bg + border-style none => RED.
  const submit = page.getByTestId(WTID.newStorySubmit);
  const styled = await submit.evaluate((el) => {
    const s = getComputedStyle(el);
    const bg = s.backgroundColor;
    const filled = bg !== "rgba(0, 0, 0, 0)" && bg !== "transparent";
    const hasBorder = s.borderTopStyle !== "none" && parseFloat(s.borderTopWidth) > 0;
    return filled || hasBorder;
  });
  expect
    .soft(styled, "new-story-submit is a styled button affordance (filled bg or visible border)")
    .toBe(true);

  //  2. A Cancel/dismiss control EXISTS inside the form. Prod has none => RED.
  await expect
    .soft(form.getByTestId(WTID.newStoryCancel), "new-story form has a Cancel/dismiss control")
    .toBeVisible();

  //  3. new-story-title has a NON-EMPTY placeholder. Prod: empty => RED.
  const title = page.getByTestId(WTID.newStoryTitle);
  const placeholder = await title.getAttribute("placeholder");
  expect
    .soft((placeholder ?? "").trim().length, "new-story-title placeholder is non-empty")
    .toBeGreaterThan(0);

  // Reference box/readColors so the copied helpers remain self-contained.
  await box(form, "new-story-form");
  await readColors(submit);
});

// ── w24.2 (G2) ──────────────────────────────────────────────────────────────

test("w24.2 (G2) Chat dock: Send is a filled button and the input has a non-empty placeholder", async ({
  page,
}) => {
  env = await WorkspaceEnv.start("w24-chat-dock");
  const e = env;

  await gotoWorkspace(page, e.baseURL, wsRoutes.product(e.sid));

  // Open the chat dock.
  await page.getByTestId(WTID.chatDockToggle).click();
  const panel = page.getByTestId(WTID.chatDockPanel);
  await expect(panel, "chat dock panel is visible after opening").toBeVisible();

  // GREEN GUARDS (HARD, must PASS — these are NOT defects, do not flag them):
  // the empty conversation body and the "+ New" header control are legitimate
  // (a real chat starts empty).
  await expect(page.getByTestId(WTID.chatDockHistory), "chat dock history body is visible (empty is legitimate)").toBeVisible();
  await expect(page.getByTestId(WTID.chatDockNew), "chat dock +New header control is visible").toBeVisible();

  // PINNED DEFECT (soft, each facet reports):
  //  1. chat-dock-send computed background-color is a SOLID FILL (not
  //     transparent) — a real primary button. Prod: transparent => RED.
  const send = page.getByTestId(WTID.chatDockSend);
  const filled = await send.evaluate((el) => {
    const bg = getComputedStyle(el).backgroundColor;
    return bg !== "rgba(0, 0, 0, 0)" && bg !== "transparent";
  });
  expect.soft(filled, "chat-dock-send has a solid background fill (not transparent)").toBe(true);

  //  2. chat-dock-input has a NON-EMPTY placeholder. Prod: empty => RED.
  const input = page.getByTestId(WTID.chatDockInput);
  const placeholder = await input.getAttribute("placeholder");
  expect
    .soft((placeholder ?? "").trim().length, "chat-dock-input placeholder is non-empty")
    .toBeGreaterThan(0);
});

// ── w24.3 (F2b) ─────────────────────────────────────────────────────────────

test("w24.3 (F2b) Scenarios kicker is the '02—Product / Scenarios' label (not the drifted '03—Requirements')", async ({
  page,
}) => {
  env = await WorkspaceEnv.start("w24-scenarios-kicker");
  const e = env;

  await gotoWorkspace(page, e.baseURL, wsRoutes.scenarios(e.sid));
  const kicker = page.getByTestId(WTID.fileViewKicker);

  // GREEN GUARD (HARD, must PASS): the kicker is present (run-1 added it).
  await expect(kicker, "scenarios page renders a kicker").toBeVisible();

  // PINNED DEFECT (soft, each facet reports): the kicker text is the
  // Product/Scenarios label (case-insensitive, dash-flexible — the kicker may
  // be uppercased via CSS). Scenarios live UNDER Product; section numbering is
  // Architecture=01, Product=02, Builds=04 — there is no "03".
  await expect
    .soft(kicker, "scenarios kicker is the Product/Scenarios label")
    .toHaveText(/02\s*[—-]\s*Product\s*\/\s*Scenarios/i);

  // AND the kicker must NOT contain the drifted label. Prod text is
  // "03—Requirements" => both facets RED.
  await expect
    .soft(kicker, "scenarios kicker does not carry the drifted '03'/'Requirements' label")
    .not.toHaveText(/03|Requirements/i);
});

// ── w24.4 (F5b) ─────────────────────────────────────────────────────────────

test("w24.4 (F5b) Anchor sidebar feature-rows show a background-FILL hover (consistent with the div rows)", async ({
  page,
}) => {
  env = await WorkspaceEnv.start("w24-anchor-row-hover");
  const e = env;

  await gotoWorkspace(page, e.baseURL, wsRoutes.product(e.sid));

  // Expand the Features + Scenarios groups so their child rows are visible for
  // the hover probes.
  const product = sidebarSection(page, "product");
  await ensureGroupExpanded(sidebarGroup(product, "Features"));
  await ensureGroupExpanded(sidebarGroup(product, "Scenarios"));

  // GREEN GUARD (HARD, must PASS): the run-1 div-row background hover still
  // works — hovering a scenario-row CHANGES its background-color (run-1 gave
  // div rows a --surface-subtle bg hover).
  await assertBackgroundHoverChange(
    page,
    page.getByTestId(WTID.scenarioRow).first(),
    "scenario-row (div) background hover",
    false,
  );

  // PINNED DEFECT (soft): hovering a feature-row CHANGES its background-color.
  // Prod: the anchor rows only inherit the global a:hover COLOR shift; the
  // background never changes => RED.
  await assertBackgroundHoverChange(
    page,
    page.getByTestId(WTID.featureRow).first(),
    "feature-row (anchor) background hover",
    true,
  );
});
