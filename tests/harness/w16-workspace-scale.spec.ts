/**
 * w16 — workspace design-SCALE + chrome-fidelity conformance (deterministic).
 *
 * Born from a passed-green-but-visibly-wrong episode (2026-08-04, analysis in
 * tmp/design-compare/why-tests-passed.md): the w8 token sweep gates only
 * palette/family membership and the w9/w15 vision judges gate only structural
 * PRESENCE, so production shipped every required element at ~143% of the
 * designed size (tokens.css editorial `--fs-body: 20px` instead of the
 * prototype's dense workspace scale) and every gate stayed green.
 *
 * The workspace scale is now promoted into the design system proper
 * (harness/design/system/workspace.css → `--wk-*`), and this spec pins the
 * production chrome to it. Expectations are parsed from that FILE (design
 * truth), then the live page must BOTH resolve the tokens (proving the app
 * consumes the layer, not coincidental numbers) and render each chrome
 * component at the token value.
 *
 * RED intent: current prod inherits the 20px editorial body scale, renders a
 * duplicated PRD entry, per-row underline rules, no scenario service tags, a
 * light chevron chat strip, a single-level breadcrumb, a hairline (not
 * strong) file-card outline with a sprawling empty tail, and a save-state
 * line clipped below the viewport. Every case fails on a wrong computed
 * style / count — an assertion failure, not a crash.
 */

import { expect, test, type Locator, type Page } from "@playwright/test";

import { DESKTOP_VIEWPORT, readWorkspaceScale } from "./helpers/design-conformance";
import { WTID, cssVarRgb, gotoWorkspace, wsRoutes } from "./helpers/ui";
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

/** The design-truth px scale, parsed from system/workspace.css. */
const WK = readWorkspaceScale();

/** Guards the parsed design file itself (a broken/renamed token must not make
 * a comparison vacuously pass against `undefined`). */
function wk(name: string): number {
  const value = WK[name];
  expect(value, `design token ${name} missing from system/workspace.css`).toBeGreaterThan(0);

  return value;
}

async function openProduct(page: Page, slug: string): Promise<void> {
  env = await WorkspaceEnv.start(slug);
  const e = env;
  await page.setViewportSize({ ...DESKTOP_VIEWPORT });
  await gotoWorkspace(page, e.baseURL, wsRoutes.product(e.sid));
  await expect(page.getByTestId(WTID.rail)).toBeVisible();
  await expect(page.getByTestId(WTID.sidebar)).toBeVisible();
  await expect(page.getByTestId(WTID.fileView)).toBeVisible();
  await page.evaluate(async () => {
    await document.fonts.ready;
  });
}

async function fontSizePx(target: Locator): Promise<number> {
  return target.evaluate((el) => Number.parseFloat(getComputedStyle(el).fontSize));
}

/** Asserts a component's computed font-size equals the named `--wk-*` token. */
async function expectScale(target: Locator, tokenName: string, label: string): Promise<void> {
  const expected = wk(tokenName);
  const actual = await fontSizePx(target.first());
  expect(
    Math.abs(actual - expected),
    `${label} renders at ${actual}px — the workspace design scale (${tokenName}) says ${expected}px`,
  ).toBeLessThan(0.6);
}

test("w16.1 the app consumes the workspace density layer and the sidebar renders at its scale", async ({
  page,
}) => {
  await openProduct(page, "w16-sidebar-scale");

  // The live page must RESOLVE the promoted tokens — proof the app imports
  // system/workspace.css rather than hardcoding numbers that happen to match.
  const resolved = await page.evaluate(() => {
    const root = getComputedStyle(document.documentElement);

    return {
      row: root.getPropertyValue("--wk-fs-row").trim(),
      body: root.getPropertyValue("--wk-fs-body").trim(),
      sidebarW: root.getPropertyValue("--wk-sidebar-w").trim(),
      railW: root.getPropertyValue("--wk-rail-w").trim(),
    };
  });
  expect(
    resolved.row,
    "--wk-fs-row does not resolve on the production page — the app does not consume design/system/workspace.css",
  ).toMatch(/^\d+px$/);
  expect(resolved.body, "--wk-fs-body does not resolve on the production page").toMatch(/^\d+px$/);
  expect(resolved.sidebarW, "--wk-sidebar-w does not resolve on the production page").toMatch(/^\d+px$/);
  expect(resolved.railW, "--wk-rail-w does not resolve on the production page").toMatch(/^\d+px$/);

  // Frame: sidebar column + icon rail widths are the design's fixed widths.
  const sidebar = page.getByTestId(WTID.sidebar);
  const sidebarBox = (await sidebar.boundingBox())!;
  expect(
    Math.abs(sidebarBox.width - wk("--wk-sidebar-w")),
    `sidebar is ${sidebarBox.width}px wide — the design (--wk-sidebar-w) says ${wk("--wk-sidebar-w")}px`,
  ).toBeLessThan(1.5);
  const railBox = (await page.getByTestId(WTID.rail).boundingBox())!;
  expect(
    Math.abs(railBox.width - wk("--wk-rail-w")),
    `icon rail is ${railBox.width}px wide — the design (--wk-rail-w) says ${wk("--wk-rail-w")}px`,
  ).toBeLessThan(1.5);

  // Row/label scale (the 20px-editorial-body regression made ALL of these ~43% oversized).
  await expectScale(page.getByTestId(WTID.featureRow), "--wk-fs-row", "sidebar feature row");
  await expectScale(page.getByTestId(WTID.storyRow), "--wk-fs-row", "sidebar story row");
  await expectScale(page.getByTestId(WTID.scenarioRow), "--wk-fs-row", "sidebar scenario row");
  await expectScale(page.getByTestId(WTID.newStoryOpen), "--wk-fs-row", "sidebar `+ New story` control");
  await expectScale(page.getByTestId(WTID.sidebarSectionHeader), "--wk-fs-caption", "sidebar section header");

  // Brand meta: micro size AND the uppercase caption treatment (prototype
  // `.sidebar-brand__meta` — prod currently renders it mixed-case at 14px).
  const brandMeta = page.getByTestId(WTID.sidebarBrandMeta);
  await expectScale(brandMeta, "--wk-fs-micro", "sidebar brand meta line");
  await expect(brandMeta).toHaveCSS("text-transform", "uppercase");

  // CONSUMPTION proof — the chrome must render FROM the tokens, not from
  // coincidentally-equal hardcoded numbers (the exact false-positive the spec
  // header warns about). Live-mutate a representative type token and a
  // dimension token on :root and confirm the consuming chrome TRACKS the
  // change, then restore. A component hardcoding 13px / 280px instead of
  // `var(--wk-*)` would not move (reviewer round 1).
  const track = async (token: string, sentinel: string, read: () => Promise<number>): Promise<number> => {
    await page.evaluate(({ t, v }) => document.documentElement.style.setProperty(t, v), { t: token, v: sentinel });
    const measured = await read();
    await page.evaluate((t) => document.documentElement.style.removeProperty(t), token);

    return measured;
  };
  const rowAt41 = await track("--wk-fs-row", "41px", () => fontSizePx(page.getByTestId(WTID.featureRow).first()));
  expect(
    Math.abs(rowAt41 - 41),
    "sidebar feature row does not track --wk-fs-row (hardcoded size, not consuming the density layer)",
  ).toBeLessThan(0.6);
  const sidebarAt331 = await track("--wk-sidebar-w", "331px", async () => (await sidebar.boundingBox())!.width);
  expect(
    Math.abs(sidebarAt331 - 331),
    "sidebar does not track --wk-sidebar-w (hardcoded width, not consuming the density layer)",
  ).toBeLessThan(1.5);
  const railAt91 = await track("--wk-rail-w", "91px", async () => (await page.getByTestId(WTID.rail).boundingBox())!.width);
  expect(
    Math.abs(railAt91 - 91),
    "icon rail does not track --wk-rail-w (hardcoded width, not consuming the density layer)",
  ).toBeLessThan(1.5);
});

test("w16.2 the content chrome renders at the workspace scale (titles keep the editorial tokens)", async ({
  page,
}) => {
  await openProduct(page, "w16-content-scale");

  // Breadcrumb bar: row-scale text in a bar of the design's height.
  const breadcrumb = page.getByTestId(WTID.contentBreadcrumb);
  await expectScale(breadcrumb, "--wk-fs-row", "breadcrumb bar");
  const crumbBox = (await breadcrumb.boundingBox())!;
  expect(
    crumbBox.height,
    `breadcrumb bar is ${crumbBox.height}px tall — the design (--wk-breadcrumb-h) says at least ${wk("--wk-breadcrumb-h")}px`,
  ).toBeGreaterThanOrEqual(wk("--wk-breadcrumb-h") - 0.5);
  // ...and NOT an oversized bar: this whole spec exists because chrome shipped
  // ~143% oversized while every gate stayed green. The design height is a dense
  // 48px min-height; a reverted 70/100px bar must fail here (reviewer round 1).
  expect(
    crumbBox.height,
    `breadcrumb bar is ${crumbBox.height}px tall — oversized past the design (--wk-breadcrumb-h) ${wk("--wk-breadcrumb-h")}px`,
  ).toBeLessThanOrEqual(wk("--wk-breadcrumb-h") + 12);

  // File-page header block + card header bar.
  await expectScale(page.getByTestId(WTID.fileViewKicker), "--wk-fs-caption", "page kicker");
  await expectScale(page.getByTestId(WTID.fileViewMeta), "--wk-fs-row", "page subtitle");
  await expectScale(page.getByTestId(WTID.fileViewPath), "--wk-fs-body", "file-card path label");
  await expectScale(page.getByTestId(WTID.fileViewLineCount), "--wk-fs-caption", "file-card line counter");

  // Boundary of the two scales: the display title KEEPS the editorial token
  // (--fs-h3) — the dense scale must not creep into headings either.
  const h3 = await page.evaluate(() =>
    Number.parseFloat(getComputedStyle(document.documentElement).getPropertyValue("--fs-h3")),
  );
  expect(h3, "--fs-h3 did not resolve").toBeGreaterThan(0);
  const titleFs = await fontSizePx(page.getByTestId(WTID.fileViewTitle));
  expect(
    Math.abs(titleFs - h3),
    `page title renders at ${titleFs}px — the editorial token (--fs-h3) says ${h3}px`,
  ).toBeLessThan(0.6);
});

test("w16.3 sidebar chrome fidelity: single PRD entry, unruled rows, visible guide line, service tags", async ({
  page,
}) => {
  await openProduct(page, "w16-sidebar-fidelity");
  const sidebar = page.getByTestId(WTID.sidebar);

  // Exactly ONE "PRD" entry (prod currently renders a PRD chip AND a PRD row).
  const prdCount = await sidebar.evaluate((root) => {
    const topmost = Array.from(root.querySelectorAll("*")).filter((el) => {
      const rect = el.getBoundingClientRect();
      if (rect.width === 0 || rect.height === 0) {
        return false;
      }
      const own = (el.textContent ?? "").trim();
      const parentOwn = (el.parentElement?.textContent ?? "").trim();

      return own === "PRD" && parentOwn !== "PRD";
    });

    return topmost.length;
  });
  expect(prdCount, "the sidebar must render exactly ONE `PRD` entry (found a duplicated chip/row)").toBe(1);

  // Rows are PLAIN — the baseline rules only section headers, never each row
  // (prototype `.sidebar-tree a { border-bottom: none }`).
  for (const [tid, label] of [
    [WTID.prdRow, "PRD row"],
    [WTID.featureRow, "feature row"],
    [WTID.storyRow, "story row"],
    [WTID.scenarioRow, "scenario row"],
  ] as const) {
    const ruled = await page.getByTestId(tid).first().evaluate((el) => {
      const hasRule = (node: Element): boolean => {
        const s = getComputedStyle(node);

        return s.borderBottomStyle !== "none" && Number.parseFloat(s.borderBottomWidth) > 0;
      };

      return hasRule(el) || Array.from(el.children).some(hasRule);
    });
    expect(ruled, `${label} must not carry a bottom border rule (baseline rows are plain)`).toBe(false);
  }

  // The nested-rows guide line is actually PAINTED (a testid that renders
  // 0px/transparent must not pass): hairline as a left border or as a 1-3px
  // filled strip.
  const hairline = await cssVarRgb(page, "--border-hairline");
  const guide = page.getByTestId(WTID.sidebarGuideLine).first();
  await expect(guide, "the sidebar guide line element must exist").toBeAttached();
  const guidePainted = await guide.evaluate((el, expected) => {
    const s = getComputedStyle(el);
    const rect = el.getBoundingClientRect();
    if (rect.height <= 0) {
      return false;
    }
    const borderPaints =
      s.borderLeftStyle !== "none" && Number.parseFloat(s.borderLeftWidth) > 0 && s.borderLeftColor === expected;
    const stripPaints = rect.width > 0 && rect.width <= 3 && s.backgroundColor === expected;

    return borderPaints || stripPaints;
  }, hairline);
  expect(guidePainted, "the sidebar guide line must visibly paint the hairline token").toBe(true);

  // Scenario rows carry the muted service annotation (prototype
  // `.sidebar-row-service`: micro size, --text-muted).
  const muted = await cssVarRgb(page, "--text-muted");
  const firstScenario = page.getByTestId(WTID.scenarioRow).first();
  const service = firstScenario.getByTestId(WTID.scenarioRowService);
  await expect(service, "scenario rows must carry a service annotation").toBeVisible();
  await expect(service).not.toBeEmpty();
  await expectScale(service, "--wk-fs-micro", "scenario-row service annotation");
  await expect(service).toHaveCSS("color", muted);
});

test("w16.4 the chat toggle is the design's inverted vertical tab", async ({ page }) => {
  await openProduct(page, "w16-chat-toggle");

  const toggle = page.getByTestId(WTID.chatDockToggle);
  await expect(toggle).toBeVisible();

  // Inverted strip (prototype `.chat-dock__tab`): dark surface, light text —
  // prod currently ships a light gray strip with a bare chevron.
  await expect(toggle).toHaveCSS("background-color", await cssVarRgb(page, "--surface-inverse"));
  await expect(toggle).toHaveCSS("color", await cssVarRgb(page, "--text-inverse"));

  // Vertical uppercase text label at caption scale — not an icon/chevron.
  await expect(toggle).toHaveText(/chat/i);
  const style = await toggle.evaluate((el) => {
    const s = getComputedStyle(el);

    return { writingMode: s.writingMode, transform: s.textTransform };
  });
  expect(style.writingMode, "the chat tab label must be vertical (writing-mode: vertical-*)").toMatch(/^vertical-/);
  expect(style.transform).toBe("uppercase");
  await expectScale(toggle, "--wk-fs-caption", "chat tab label");

  // The vertical-tab treatment is scoped to the COLLAPSED tab only: the OPEN
  // panel reuses the same `chat-dock-toggle` testid for its "Close chat"
  // control, which must stay a NORMAL horizontal button inside the horizontal
  // header row — not inherit the vertical strip. Without this, re-broadening
  // the CSS selector back to the bare testid renders the header control as a
  // squeezed vertical strip and no deterministic test would catch it
  // (reviewer round 1 — protects the fixer's own scoping fix).
  await toggle.click();
  const closeChat = page.getByRole("button", { name: "Close chat" });
  await expect(closeChat).toBeVisible();
  const openStyle = await closeChat.evaluate((el) => ({
    writingMode: getComputedStyle(el).writingMode,
    inHeader: el.closest('[data-testid="chat-dock-header"]') !== null,
  }));
  expect(openStyle.writingMode, "the open-panel `Close chat` control must stay horizontal").toMatch(/^horizontal/);
  expect(openStyle.inHeader, "the `Close chat` control must sit inside the horizontal chat-dock header").toBe(true);
});

test("w16.5 the file card is a dark-outlined box that hugs its content; save-state stays on screen", async ({
  page,
}) => {
  await openProduct(page, "w16-file-card");

  const strong = await cssVarRgb(page, "--border-strong");

  // The card's visual outline draws the STRONG border token. The prototype
  // splits the outline across the header bar (top/left/right) and the editor
  // box (all four sides), so accept the outline from the card's structural
  // elements collectively — but every one of the four sides must be covered
  // by SOME strong edge, and prod's current all-hairline card must fail.
  const sides = await page.evaluate((expected) => {
    const header = document.querySelector('[data-testid="file-view-header"]');
    const gutter = document.querySelector('[data-testid="file-view-gutter"]');
    const candidates = new Set<Element>();
    for (const el of [header, header?.parentElement, gutter?.parentElement, gutter?.parentElement?.parentElement]) {
      if (el instanceof Element) {
        candidates.add(el);
      }
    }
    const covered = { top: false, right: false, bottom: false, left: false };
    for (const el of candidates) {
      const s = getComputedStyle(el);
      for (const side of ["top", "right", "bottom", "left"] as const) {
        const width = Number.parseFloat(s.getPropertyValue(`border-${side}-width`));
        const styleValue = s.getPropertyValue(`border-${side}-style`);
        const color = s.getPropertyValue(`border-${side}-color`);
        if (width > 0 && styleValue !== "none" && color === expected) {
          covered[side] = true;
        }
      }
    }

    return covered;
  }, strong);
  for (const side of ["top", "right", "bottom", "left"] as const) {
    expect(sides[side], `the file card's ${side} edge must draw the strong (dark) border token`).toBe(true);
  }

  // The card ENDS at its content: the gap between the last gutter line and
  // the bottom of the card body must stay within one padding + one line —
  // prod currently stretches the card to fill the pane (~140px empty tail).
  const lastLine = page.getByTestId(WTID.fileViewGutterLine).last();
  await expect(lastLine).toBeVisible();
  const tail = await lastLine.evaluate((el) => {
    const body = el.closest('[data-testid="file-view-gutter"]')?.parentElement;
    if (body === null || body === undefined) {
      return Number.NaN;
    }

    return body.getBoundingClientRect().bottom - el.getBoundingClientRect().bottom;
  });
  expect(Number.isNaN(tail), "could not locate the file-card body around the gutter").toBe(false);
  expect(
    tail,
    `the file card trails ${Math.round(tail)}px of empty space below the last line — the design card hugs its content`,
  ).toBeLessThanOrEqual(48);

  // The save-state line must be fully INSIDE the viewport (prod currently
  // clips it below the fold at the 1440x900 reference viewport). Asserted
  // UNCONDITIONALLY — a guarded `if (count > 0)` no-ops (vacuously passes) the
  // instant the element vanishes, defeating the on-screen check this case
  // exists for (reviewer round 1). The file view always renders save-state.
  const saveState = page.getByTestId(WTID.saveState);
  await expect(saveState).toHaveCount(1);
  await expect(saveState.first()).toBeVisible();
  const box = (await saveState.first().boundingBox())!;
  // Fully inside the viewport on EVERY edge — not merely the bottom (reviewer
  // round 2): the RED intent is a below-the-fold clip, but "fully INSIDE"
  // means all four edges stay within the 1440x900 reference frame.
  expect(box.x, `save-state left edge sits at ${Math.round(box.x)}px — off the left of the viewport`).toBeGreaterThanOrEqual(0);
  expect(box.y, `save-state top edge sits at ${Math.round(box.y)}px — above the viewport`).toBeGreaterThanOrEqual(0);
  expect(
    box.x + box.width,
    `save-state right edge sits at ${Math.round(box.x + box.width)}px — past the ${DESKTOP_VIEWPORT.width}px viewport`,
  ).toBeLessThanOrEqual(DESKTOP_VIEWPORT.width);
  expect(
    box.y + box.height,
    `save-state bottom edge sits at ${Math.round(box.y + box.height)}px — below the ${DESKTOP_VIEWPORT.height}px viewport`,
  ).toBeLessThanOrEqual(DESKTOP_VIEWPORT.height);
});

test("w16.6 the product breadcrumb is a spaced multi-level trail ending in a distinct current crumb", async ({
  page,
}) => {
  await openProduct(page, "w16-breadcrumb-trail");

  const breadcrumb = page.getByTestId(WTID.contentBreadcrumb);
  await expect(breadcrumb).toBeVisible();

  // Multi-level: at least two parent LINKS (baseline: Sessions / Workspace
  // overview / prd.yaml) — prod currently renders a single "Home" link.
  const links = breadcrumb.locator("a");
  expect(await links.count(), "the product breadcrumb must offer at least two parent links").toBeGreaterThanOrEqual(2);

  // Dedicated separator elements between crumbs (prototype `.crumb-sep`) —
  // NOT a "/" glued into the crumb text ("Home /Product").
  const seps = breadcrumb.getByTestId(WTID.breadcrumbSep);
  expect(await seps.count(), "the breadcrumb must render dedicated separator elements").toBeGreaterThanOrEqual(2);

  // Every separator is visually SPACED from both neighbours (≥2px of air).
  const spaced = await breadcrumb.evaluate((root) => {
    const seps2 = Array.from(root.querySelectorAll('[data-testid="breadcrumb-sep"]'));
    const crumbs = Array.from(root.children).filter((el) => (el.textContent ?? "").trim() !== "");

    return seps2.every((sep) => {
      const sepBox = sep.getBoundingClientRect();
      const neighbours = crumbs.filter((el) => el !== sep);
      const before = neighbours
        .map((el) => el.getBoundingClientRect())
        .filter((b) => b.right <= sepBox.left + 0.5)
        .sort((a, b) => b.right - a.right)[0];
      const after = neighbours
        .map((el) => el.getBoundingClientRect())
        .filter((b) => b.left >= sepBox.right - 0.5)
        .sort((a, b) => a.left - b.left)[0];

      return (
        before !== undefined && after !== undefined && sepBox.left - before.right >= 2 && after.left - sepBox.right >= 2
      );
    });
  });
  expect(spaced, "each breadcrumb separator must have visible spacing on both sides").toBe(true);

  // The terminal crumb is the CURRENT document, visually distinct: bold, the
  // primary ink, and marked aria-current="page".
  const current = breadcrumb.locator('[aria-current="page"]');
  await expect(current, 'the current crumb must be marked aria-current="page"').toHaveCount(1);
  await expect(current).toHaveText(/prd\.yaml/i);
  await expect(current).toHaveCSS("color", await cssVarRgb(page, "--text-primary"));
  const weight = await current.evaluate((el) => Number.parseFloat(getComputedStyle(el).fontWeight));
  expect(weight, "the current crumb must be bold").toBeGreaterThanOrEqual(700);

  // The bare `/product` route IS the PRD file's own page, so its trail is
  // EXACTLY the three baseline crumbs — Sessions / Workspace overview /
  // prd.yaml — with NO self-referential "Product" parent crumb linking to the
  // current route. The ≥2-link / ≥2-sep checks above would happily accept the
  // rejected 4-crumb `Sessions / Workspace overview / Product / prd.yaml`
  // trail; that exact-shape property lived ONLY in the gitignored unit suite,
  // so a regenerate-from-tests could silently reintroduce the self-ref crumb.
  // Pin it here (reviewer round 1 — durable-spec audit).
  // Count ALL crumb elements — the direct nav children that aren't separators —
  // NOT just `<a>`/current: a non-routable crumb renders as a plain
  // `<span class="ws-breadcrumb-link">` (WorkspaceShell) and an `a`-only
  // selector would let an extra "Product" span slip through with the three
  // expected texts still matching (reviewer round 2).
  const crumbTexts = await breadcrumb.evaluate((root) =>
    Array.from(root.children)
      .filter((el) => el.getAttribute("data-testid") !== "breadcrumb-sep")
      .map((el) => (el.textContent ?? "").trim())
      .filter((text) => text !== ""),
  );
  expect(crumbTexts, "the bare /product breadcrumb must be exactly the three baseline crumbs").toEqual([
    "Sessions",
    "Workspace overview",
    "prd.yaml",
  ]);
  const currentHref = new URL(page.url()).pathname;
  const selfRefLinks = await breadcrumb.evaluate((root, current2) => {
    return Array.from(root.querySelectorAll("a"))
      .filter((a) => a.getAttribute("aria-current") !== "page")
      .filter((a) => new URL((a as HTMLAnchorElement).href).pathname === current2)
      .map((a) => (a.textContent ?? "").trim());
  }, currentHref);
  expect(
    selfRefLinks,
    "no parent breadcrumb link may point at the current route (self-referential crumb)",
  ).toEqual([]);
});

test("w16.7 residual chrome fidelity: default scale, rail anatomy, full-bleed bands, label/border/padding details", async ({
  page,
}) => {
  // Born from the second comparison round (tmp/design-compare/
  // what-was-not-fixed.md): after the first fix every ASSERTED property was
  // green while ten unasserted details still diverged from the prototype.
  // This case pins them.
  await openProduct(page, "w16-residual-fidelity");
  const sidebar = page.getByTestId(WTID.sidebar);
  const sidebarBox = (await sidebar.boundingBox())!;

  // #1 — THE DEFAULT: the workspace shell's inherited font-size is the dense
  // --wk-fs-body, so future chrome is dense-by-default instead of
  // editorial-unless-overridden (the latent 20px-body regression trap).
  const shell = page.getByTestId(WTID.appShell);
  await expect(shell).toBeVisible();
  const shellFs = await fontSizePx(shell);
  expect(
    Math.abs(shellFs - wk("--wk-fs-body")),
    `the workspace shell's inherited font-size is ${shellFs}px — the dense default (--wk-fs-body) says ${wk("--wk-fs-body")}px; ` +
      "chrome must be dense-by-default, not editorial-unless-overridden",
  ).toBeLessThan(0.6);

  // #2 — rail anatomy: icon and label render at the promoted rail tokens, the
  // label is BOLD, and the item carries the design's vertical padding
  // (--space-3), giving the prototype's 76px-tall tiles.
  const railIcon = page.getByTestId(WTID.railItemIcon).first();
  const railLabel = page.getByTestId(WTID.railItemLabel).first();
  await expectScale(railIcon, "--wk-rail-icon-fs", "rail item icon");
  await expectScale(railLabel, "--wk-rail-label-fs", "rail item label");
  const labelWeight = await railLabel.evaluate((el) => Number.parseFloat(getComputedStyle(el).fontWeight));
  expect(labelWeight, "the rail item label must be bold (prototype `.rail-item__label`)").toBeGreaterThanOrEqual(700);
  const space3 = await page.evaluate(() =>
    Number.parseFloat(getComputedStyle(document.documentElement).getPropertyValue("--space-3")),
  );
  expect(space3, "--space-3 did not resolve").toBeGreaterThan(0);
  const railItem = page.locator('[data-testid="rail-item-product"]');
  const railPad = await railItem.evaluate((el) => ({
    top: Number.parseFloat(getComputedStyle(el).paddingTop),
    bottom: Number.parseFloat(getComputedStyle(el).paddingBottom),
  }));
  expect(
    Math.abs(railPad.top - space3),
    `rail item padding-top is ${railPad.top}px — the design (--space-3) says ${space3}px`,
  ).toBeLessThan(0.6);
  expect(
    Math.abs(railPad.bottom - space3),
    `rail item padding-bottom is ${railPad.bottom}px — the design (--space-3) says ${space3}px`,
  ).toBeLessThan(0.6);

  // #3 — full-bleed bands: the active PRD row's highlight band and the section
  // header's band span the sidebar edge-to-edge (prototype sections are
  // full-bleed; a padded/inset band must fail). The section header band paints
  // --surface-subtle either on the header itself or on a wrapping band row.
  const subtle = await cssVarRgb(page, "--surface-subtle");
  const prdBand = (await page.getByTestId(WTID.prdRow).boundingBox())!;
  expect(
    prdBand.width,
    `the PRD row band is ${Math.round(prdBand.width)}px wide inside a ${Math.round(sidebarBox.width)}px sidebar — ` +
      "the prototype band runs full-bleed",
  ).toBeGreaterThanOrEqual(sidebarBox.width - 2);
  const headerBand = await page.getByTestId(WTID.sidebarSectionHeader).evaluate((el, expected) => {
    let node: Element | null = el;
    while (node !== null && node.getAttribute("data-testid") !== "sidebar") {
      const s = getComputedStyle(node);
      if (s.backgroundColor === expected) {
        return { found: true, width: node.getBoundingClientRect().width };
      }
      node = node.parentElement;
    }

    return { found: false, width: 0 };
  }, subtle);
  expect(
    headerBand.found,
    "the section header must sit on a --surface-subtle band (prototype `.sidebar-section[open] > summary`)",
  ).toBe(true);
  expect(
    headerBand.width,
    `the section-header band is ${Math.round(headerBand.width)}px wide inside a ${Math.round(sidebarBox.width)}px sidebar — full-bleed expected`,
  ).toBeGreaterThanOrEqual(sidebarBox.width - 2);

  // #4 — group label treatment: every group label ends with the design's
  // trailing colon and is underlined (all three — the baseline underlines
  // Features:, Stories: AND Scenarios:).
  const groupNames = page.getByTestId(WTID.sidebarGroupName);
  const groupCount = await groupNames.count();
  expect(groupCount, "expected the three product group labels").toBeGreaterThanOrEqual(3);
  for (let i = 0; i < groupCount; i += 1) {
    const label = groupNames.nth(i);
    const text = ((await label.textContent()) ?? "").trim();
    expect(text.endsWith(":"), `group label "${text}" must carry the design's trailing colon`).toBe(true);
    const underlined = await label.evaluate((el) => {
      const check = (node: Element): boolean => {
        const s = getComputedStyle(node);

        return (
          (s.borderBottomStyle !== "none" && Number.parseFloat(s.borderBottomWidth) > 0) ||
          /underline/.test(s.textDecorationLine || s.textDecoration)
        );
      };

      return check(el) || (el.parentElement !== null && check(el.parentElement));
    });
    expect(underlined, `group label "${text}" must be underlined (all three are in the baseline)`).toBe(true);
  }

  // #5 — `+ New story` is a QUIET affordance: muted ink, regular weight (the
  // prototype renders it as muted row text, not a bold black control).
  const newStory = page.getByTestId(WTID.newStoryOpen);
  await expect(newStory).toHaveCSS("color", await cssVarRgb(page, "--text-muted"));
  const newStoryWeight = await newStory.evaluate((el) => Number.parseFloat(getComputedStyle(el).fontWeight));
  expect(newStoryWeight, "`+ New story` must not be bold (baseline renders it muted/regular)").toBeLessThan(700);

  // #6 — the card outline uses the design's --border-width (1px), not the
  // 2px strong-width variant: every strong-colored edge found on the card's
  // structural elements must be exactly --border-width thick.
  const strong = await cssVarRgb(page, "--border-strong");
  const borderWidth = await page.evaluate(() =>
    Number.parseFloat(getComputedStyle(document.documentElement).getPropertyValue("--border-width")),
  );
  expect(borderWidth, "--border-width did not resolve").toBeGreaterThan(0);
  const wrongWidths = await page.evaluate(({ expectedColor, expectedWidth }) => {
    const header = document.querySelector('[data-testid="file-view-header"]');
    const gutter = document.querySelector('[data-testid="file-view-gutter"]');
    const candidates = new Set<Element>();
    for (const el of [header, header?.parentElement, gutter?.parentElement, gutter?.parentElement?.parentElement]) {
      if (el instanceof Element) {
        candidates.add(el);
      }
    }
    const bad: string[] = [];
    for (const el of candidates) {
      const s = getComputedStyle(el);
      for (const side of ["top", "right", "bottom", "left"]) {
        const width = Number.parseFloat(s.getPropertyValue(`border-${side}-width`));
        const styleValue = s.getPropertyValue(`border-${side}-style`);
        const color = s.getPropertyValue(`border-${side}-color`);
        if (width > 0 && styleValue !== "none" && color === expectedColor && Math.abs(width - expectedWidth) > 0.1) {
          bad.push(`${side}: ${width}px`);
        }
      }
    }

    return bad;
  }, { expectedColor: strong, expectedWidth: borderWidth });
  expect(
    wrongWidths,
    `card outline edges must be exactly --border-width (${borderWidth}px); off-width strong edges: ${wrongWidths.join(", ")}`,
  ).toEqual([]);

  // #7/#8 — gutter and editor spacing follow the prototype's tokens: the
  // gutter carries --space-2 on ALL four edges (prototype `.yaml-editor__gutter
  // { padding: var(--space-2) }` — the w17 pixel gate exposed the digits
  // ghosting vertically when only the x-edges were pinned); editor padding
  // --space-2 top / --space-3 left.
  const space2 = await page.evaluate(() =>
    Number.parseFloat(getComputedStyle(document.documentElement).getPropertyValue("--space-2")),
  );
  expect(space2, "--space-2 did not resolve").toBeGreaterThan(0);
  const gutterPad = await page.getByTestId(WTID.fileViewGutter).evaluate((el) => ({
    top: Number.parseFloat(getComputedStyle(el).paddingTop),
    right: Number.parseFloat(getComputedStyle(el).paddingRight),
    bottom: Number.parseFloat(getComputedStyle(el).paddingBottom),
    left: Number.parseFloat(getComputedStyle(el).paddingLeft),
  }));
  for (const side of ["top", "right", "bottom", "left"] as const) {
    expect(
      Math.abs(gutterPad[side] - space2),
      `gutter padding-${side} is ${gutterPad[side]}px — the design (--space-2) says ${space2}px`,
    ).toBeLessThan(0.6);
  }
  const editorPad = await page.getByTestId(WTID.fileViewEditor).evaluate((el) => ({
    top: Number.parseFloat(getComputedStyle(el).paddingTop),
    left: Number.parseFloat(getComputedStyle(el).paddingLeft),
  }));
  expect(
    Math.abs(editorPad.top - space2),
    `editor padding-top is ${editorPad.top}px — the design (--space-2) says ${space2}px`,
  ).toBeLessThan(0.6);
  expect(
    Math.abs(editorPad.left - space3),
    `editor padding-left is ${editorPad.left}px — the design (--space-3) says ${space3}px`,
  ).toBeLessThan(0.6);

  // #9 — the tree-row BOX MODEL (prototype `.sidebar-tree a`): padding
  // --space-1 / --space-3 / --space-1 / --space-4. The w17 pixel gate exposed
  // every sidebar row's text ghosting at a shifted x/y against the golden —
  // matching font sizes at the wrong indents is still the wrong design.
  const space1 = await page.evaluate(() =>
    Number.parseFloat(getComputedStyle(document.documentElement).getPropertyValue("--space-1")),
  );
  const space4 = await page.evaluate(() =>
    Number.parseFloat(getComputedStyle(document.documentElement).getPropertyValue("--space-4")),
  );
  expect(space1, "--space-1 did not resolve").toBeGreaterThan(0);
  expect(space4, "--space-4 did not resolve").toBeGreaterThan(0);
  for (const [tid, rowLabel] of [
    [WTID.featureRow, "feature row"],
    [WTID.storyRow, "story row"],
    [WTID.scenarioRow, "scenario row"],
  ] as const) {
    const pad = await page.getByTestId(tid).first().evaluate((el) => {
      const s = getComputedStyle(el);

      return {
        top: Number.parseFloat(s.paddingTop),
        right: Number.parseFloat(s.paddingRight),
        bottom: Number.parseFloat(s.paddingBottom),
        left: Number.parseFloat(s.paddingLeft),
      };
    });
    expect(
      Math.abs(pad.top - space1),
      `${rowLabel} padding-top is ${pad.top}px — the design (--space-1) says ${space1}px`,
    ).toBeLessThan(0.6);
    expect(
      Math.abs(pad.bottom - space1),
      `${rowLabel} padding-bottom is ${pad.bottom}px — the design (--space-1) says ${space1}px`,
    ).toBeLessThan(0.6);
    expect(
      Math.abs(pad.right - space3),
      `${rowLabel} padding-right is ${pad.right}px — the design (--space-3) says ${space3}px`,
    ).toBeLessThan(0.6);
    expect(
      Math.abs(pad.left - space4),
      `${rowLabel} padding-left is ${pad.left}px — the design (--space-4) says ${space4}px`,
    ).toBeLessThan(0.6);
  }

  // The PRD row is a TREE row like its siblings (prototype renders it as a
  // `.sidebar-tree a` at --wk-fs-row, bold only via its active state — NOT
  // the 15px `.sidebar-flat-link`, which the live prototype never applies to
  // PRD; measured 13px/700 on the running baseline). An earlier revision of
  // this assertion pinned --wk-fs-label by mistake and the task-blind fixer
  // faithfully implemented the divergence — fixer round-3 escalation.
  await expectScale(page.getByTestId(WTID.prdRow), "--wk-fs-row", "PRD row");
});
