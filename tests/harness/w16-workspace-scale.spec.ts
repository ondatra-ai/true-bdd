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
    };
  });
  expect(
    resolved.row,
    "--wk-fs-row does not resolve on the production page — the app does not consume design/system/workspace.css",
  ).toMatch(/^\d+px$/);
  expect(resolved.body, "--wk-fs-body does not resolve on the production page").toMatch(/^\d+px$/);
  expect(resolved.sidebarW, "--wk-sidebar-w does not resolve on the production page").toMatch(/^\d+px$/);

  // Frame: sidebar column width is the design's fixed width.
  const sidebar = page.getByTestId(WTID.sidebar);
  const sidebarBox = (await sidebar.boundingBox())!;
  expect(
    Math.abs(sidebarBox.width - wk("--wk-sidebar-w")),
    `sidebar is ${sidebarBox.width}px wide — the design (--wk-sidebar-w) says ${wk("--wk-sidebar-w")}px`,
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
  // clips it below the fold at the 1440x900 reference viewport).
  const saveState = page.getByTestId(WTID.saveState);
  if ((await saveState.count()) > 0) {
    const box = (await saveState.first().boundingBox())!;
    expect(
      box.y + box.height,
      `save-state bottom edge sits at ${Math.round(box.y + box.height)}px — below the ${DESKTOP_VIEWPORT.height}px viewport`,
    ).toBeLessThanOrEqual(DESKTOP_VIEWPORT.height);
  }
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
});
