/**
 * w12 — shell parity for the Product section (task
 * `product-section-prototype-parity`, R1 + R2). Deterministic structural specs
 * that pin the branded sidebar + the icon/small-caps rail to the runnable
 * prototype's DOM contract.
 *
 * RED intent (R9): current prod ships a bare sidebar (no brand header, no
 * underlined section header, `+ New story` on the PAGE not the sidebar) and a
 * truncated-label rail with no icon/label sub-elements and a `Settings` bottom
 * utility. Every case fails on a MISSING testid / wrong computed style — an
 * assertion failure against current prod, not a crash.
 */

import { expect, test } from "@playwright/test";

import {
  WTID,
  cssVarRgb,
  gotoWorkspace,
  railItem,
  railItemIcon,
  railItemLabel,
  sidebarGroup,
  sidebarSection,
  wsRoutes,
  type WorkspaceSection,
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

/** The FULL rail label word the prototype presents (small-caps via CSS, not DOM caps). */
const RAIL_LABEL: Record<WorkspaceSection, string> = {
  home: "Home",
  architecture: "Architecture",
  product: "Product",
  builds: "Builds",
};

test("w12.1 the Product sidebar is branded: brand header + underlined section header + in-sidebar new-story (R1)", async ({
  page,
}) => {
  env = await WorkspaceEnv.start("w12-branded-sidebar");
  const e = env;
  await page.setViewportSize({ width: 1440, height: 900 });

  await gotoWorkspace(page, e.baseURL, wsRoutes.product(e.sid));
  const sidebar = page.getByTestId(WTID.sidebar);
  await expect(sidebar).toBeVisible();

  // Brand header ("TrueBDD") + a DISTINCT workspace-context meta line, both in
  // the sidebar — absent in current prod.
  const brand = sidebar.getByTestId(WTID.sidebarBrand);
  await expect(brand).toBeVisible();
  await expect(brand).toContainText("TrueBDD");
  const brandMeta = sidebar.getByTestId(WTID.sidebarBrandMeta);
  await expect(brandMeta).toBeVisible();
  await expect(brandMeta).not.toBeEmpty();
  // The meta is a SEPARATE element positioned BENEATH the brand name (round-1
  // F8: distinct node + vertical order, not merely different text).
  const brandName = brand.getByText("TrueBDD");
  expect(await brandMeta.textContent()).not.toBe(await brandName.textContent());
  const brandNameBox = (await brandName.boundingBox())!;
  const brandMetaBox = (await brandMeta.boundingBox())!;
  expect(brandMetaBox.y).toBeGreaterThan(brandNameBox.y);

  // Underlined "02—PRODUCT" section header inside the Product section.
  const product = sidebarSection(page, "product");
  const sectionHeader = product.getByTestId(WTID.sidebarSectionHeader);
  await expect(sectionHeader).toBeVisible();
  await expect(sectionHeader).toHaveText(/02\s*[—-]\s*PRODUCT/i);
  const underlined = await sectionHeader.evaluate((el) => {
    const cs = getComputedStyle(el);
    const borderBottom =
      cs.borderBottomStyle !== "none" && Number.parseFloat(cs.borderBottomWidth) > 0;
    const textUnderline = /underline/.test(cs.textDecorationLine || cs.textDecoration);

    return borderBottom || textUnderline;
  });
  expect(underlined, "section header must be underlined (border-bottom or text-decoration)").toBe(true);

  // PRD file row highlighted on /product.
  await expect(product.getByTestId(WTID.prdRow)).toHaveAttribute("data-selected", "true");

  // `+ New story` lives IN the sidebar Stories group (relocated from the page)
  // and still opens the new-story form.
  const stories = sidebarGroup(product, "Stories");
  const newStory = stories.getByTestId(WTID.newStoryOpen);
  await expect(newStory).toBeVisible();
  await newStory.click();
  await expect(page.getByTestId(WTID.newStoryForm)).toBeVisible();
});

test("w12.2 the rail shows icon + small-caps FULL labels, an inverted active tile, and a bottom Sessions entry (R2)", async ({
  page,
}) => {
  env = await WorkspaceEnv.start("w12-rail-parity");
  const e = env;
  await page.setViewportSize({ width: 1440, height: 900 });

  await gotoWorkspace(page, e.baseURL, wsRoutes.product(e.sid));
  await expect(page.getByTestId(WTID.rail)).toBeVisible();

  const sections: WorkspaceSection[] = ["home", "architecture", "product", "builds"];
  for (const section of sections) {
    // Icon glyph + a full-word label sub-element (absent in current prod).
    const icon = railItemIcon(page, section);
    const label = railItemLabel(page, section);
    await expect(icon).toBeVisible();
    // A real icon renders a glyph (text) OR an svg/img — NOT necessarily text, so
    // an svg/font icon is accepted too (round-1 F6: `not.toBeEmpty` wrongly
    // rejected non-text icons).
    const iconHasGlyph = await icon.evaluate(
      (el) => (el.textContent ?? "").trim().length > 0 || el.querySelector("svg, img") !== null,
    );
    expect(iconHasGlyph, `${section} rail icon must render a glyph or an svg/img`).toBe(true);
    await expect(label).toHaveText(RAIL_LABEL[section]); // FULL word, not "Arch"/"Prod"/"Bld"

    // Small-caps presentation = CSS uppercase transform + a --ls-label-derived
    // letter-spacing (F4: a title-case span with NO transform must not pass).
    const style = await label.evaluate((el) => {
      const cs = getComputedStyle(el);
      const rootLs = getComputedStyle(document.documentElement).getPropertyValue("--ls-label").trim();
      const em = Number.parseFloat(rootLs);
      const fontSize = Number.parseFloat(cs.fontSize);

      return {
        transform: cs.textTransform,
        letterSpacing: cs.letterSpacing,
        expectedPx: Number.isFinite(em) && /em$/.test(rootLs) ? em * fontSize : null,
      };
    });
    expect(style.transform).toBe("uppercase");
    if (style.expectedPx !== null) {
      const actual = Number.parseFloat(style.letterSpacing);
      expect(Number.isFinite(actual), `letter-spacing "${style.letterSpacing}" is not a length`).toBe(true);
      expect(
        Math.abs(actual - style.expectedPx),
        `letter-spacing (${style.letterSpacing}) must resolve from --ls-label (${style.expectedPx}px)`,
      ).toBeLessThan(0.6);
    } else {
      expect(style.letterSpacing).not.toBe("normal");
    }

    // Icon sits ABOVE or BEFORE the label (reading order).
    const iconBox = (await icon.boundingBox())!;
    const labelBox = (await label.boundingBox())!;
    const aboveOrBefore =
      iconBox.y + iconBox.height <= labelBox.y + 2 || iconBox.x + iconBox.width <= labelBox.x + 2;
    expect(aboveOrBefore, "the rail icon must sit above/before its label").toBe(true);
  }

  // The active entry (product) is an INVERTED tile on the dark rail: its bg
  // resolves from --surface-page and fg from --text-primary — the inverse of
  // the dark rail (--surface-inverse) AND distinct from an inactive entry.
  await expect(railItem(page, "product")).toHaveAttribute("aria-current", "page");
  const activeBg = await railItem(page, "product").evaluate((el) => getComputedStyle(el).backgroundColor);
  const activeFg = await railItem(page, "product").evaluate((el) => getComputedStyle(el).color);
  const inactiveBg = await railItem(page, "architecture").evaluate((el) => getComputedStyle(el).backgroundColor);
  const surfacePage = await cssVarRgb(page, "--surface-page");
  const textPrimary = await cssVarRgb(page, "--text-primary");
  const surfaceInverse = await cssVarRgb(page, "--surface-inverse");
  expect(activeBg).toBe(surfacePage);
  expect(activeFg).toBe(textPrimary);
  expect(activeBg).not.toBe(inactiveBg);
  expect(activeBg).not.toBe(surfaceInverse);

  // Sessions entry pinned in the LOWER rail region (a Sessions LINK to the
  // sessions route, NOT the current `Settings` button).
  const railUtilities = page.getByTestId(WTID.railUtilities);
  const sessions = railUtilities.getByRole("link", { name: /sessions/i });
  await expect(sessions).toBeVisible();
  await expect(sessions).toHaveAttribute("href", /\/(sessions)?$/);
  const railBox = (await page.getByTestId(WTID.rail).boundingBox())!;
  const utilBox = (await railUtilities.boundingBox())!;
  expect(utilBox.y + utilBox.height).toBeGreaterThan(railBox.y + railBox.height * 0.6);
});
