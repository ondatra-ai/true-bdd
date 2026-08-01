/**
 * m2 — clickable flows across the static mockups + breadcrumb trails.
 *
 * Every navigation hop is a real `click()` over file://; a dead / wrong / empty
 * href fails the landing assertion. Content signatures are drawn from the same
 * fixtures as m1 (p3-inventory-spread epic 60 / story 60.2; E2E-601 linked to
 * story 60.2 per scenarios.yaml).
 */

import { expect, test } from "@playwright/test";
import fs from "node:fs";
import { fileURLToPath } from "node:url";

import {
  BREADCRUMB_PAGES,
  MOCKUP_TID,
  expandSidebarTree,
  openMockup,
  sidebarSectionId,
} from "./helpers/mockups";

test.describe("m2 mockup navigation", () => {
  // ── m2.1 — sidebar → section → epic → story ──
  test("m2.1 sidebar → Product epic → story 60.2", async ({ page }) => {
    await openMockup(page, "workspace-overview");
    await expandSidebarTree(page);
    const sidebar = page.getByTestId(MOCKUP_TID.sidebar);
    await sidebar.locator('a[href$="epic.html"]').first().click();
    await expect(page).toHaveURL(/epic\.html$/);
    await expect(page.getByText("Inventory Spread Epic").first()).toBeVisible();

    // From the epic page, open story 60.2 from the grouped table (canvas).
    await page.getByTestId(MOCKUP_TID.canvas).getByRole("link", { name: /60\.2/ }).first().click();
    await expect(page).toHaveURL(/story-detail\.html$/);
    await expect(page.getByText("60.2").first()).toBeVisible();
  });

  // ── m2.2 — scenario list → scenario detail → linked story (destination verified) ──
  test("m2.2 scenarios → scenario-detail E2E-601 → linked story 60.2", async ({ page }) => {
    await openMockup(page, "scenarios");
    await page
      .getByTestId(MOCKUP_TID.canvas)
      .getByRole("link", { name: /E2E-601/ })
      .first()
      .click();
    await expect(page).toHaveURL(/scenario-detail\.html$/);
    await expect(page.getByText("E2E-601").first()).toBeVisible();
    await expect(page.getByText("mcp-service").first()).toBeVisible();

    // Click the linked story (E2E-601 originates from 60.2) and assert ARRIVAL —
    // an empty / fragment / wrong href would fail here.
    await page
      .getByTestId(MOCKUP_TID.canvas)
      .getByRole("link", { name: /60\.2/ })
      .first()
      .click();
    await expect(page).toHaveURL(/story-detail\.html$/);
    await expect(page.getByText("60.2").first()).toBeVisible();
    await expect(
      page.getByText(/give me a short summary of a shared Google Doc/i).first(),
    ).toBeVisible();
  });

  // ── m2.3 — runs → run detail; session-scoping surfaced in presentation ──
  test("m2.3 runs → run-detail carries the owning session id", async ({ page }) => {
    await openMockup(page, "runs");
    const firstRow = page.getByTestId(MOCKUP_TID.runRow).first();
    await expect(firstRow).toBeVisible();
    const listSession = await firstRow.getAttribute("data-session-id");
    expect(listSession, "run row carries data-session-id").toBeTruthy();

    await firstRow.locator('a[href*="run-detail"]').first().click();
    await expect(page).toHaveURL(/run-detail\.html/);
    await expect(page.getByTestId(MOCKUP_TID.runOutcome)).toBeVisible();

    // The run-detail header visibly carries the SAME owning session id.
    await expect(page.getByText(listSession!).first()).toBeVisible();
  });

  // ── m2.4 — breadcrumb trails on every workspace content page ──
  //
  // A tolerant per-page name for the CURRENT (last) crumb, so a single generic
  // breadcrumb cannot satisfy all 16 pages. Regexes (not exact labels) keep the
  // test robust to the coder's final wording while still differentiating pages.
  // Each page's current crumb must carry its DIFFERENTIATING term, so a single
  // generic breadcrumb cannot satisfy several pages (round 2, finding 2).
  const CURRENT_CRUMB_NAME: Record<string, RegExp> = {
    "workspace-overview": /overview|workspace/i,
    "prd-overview": /prd|product/i,
    epic: /epic|Inventory Spread/i,
    "story-detail": /60\.2|Summary For Shared Docs/i,
    "story-ambiguous": /ambiguous/i,
    "story-invalid": /invalid/i,
    scenarios: /scenario|requirement/i,
    "scenario-detail": /E2E-601/,
    service: /mcp-service|service/i,
    vocabulary: /vocabulary/i,
    runs: /runs?/i,
    "run-detail": /run/i,
    // The three prompt pages INHERIT the run-detail breadcrumb (plan m2.4) — the
    // prompt KIND is conveyed by the dialog (tested in m4), not the crumb.
    "prompt-choice": /run/i,
    "prompt-clarify": /run/i,
    "prompt-freetext": /run/i,
    unavailable: /unavailable/i,
  };

  for (const name of BREADCRUMB_PAGES) {
    test(`m2.4 ${name}: ordered breadcrumb; current crumb last & names the page`, async ({ page }) => {
      await openMockup(page, name);
      const crumb = page.getByTestId(MOCKUP_TID.breadcrumb);
      await expect(crumb).toHaveCount(1);
      await expect(crumb).toBeVisible();

      // Ordered crumb leaves: links (non-current) and the single current marker.
      const items = crumb.locator('a[href], [aria-current="page"]');
      const total = await items.count();
      expect(total, "breadcrumb has an ordered multi-crumb trail").toBeGreaterThanOrEqual(2);

      // Exactly one current crumb, marked aria-current="page" and NOT a link.
      const current = crumb.locator('[aria-current="page"]');
      await expect(current).toHaveCount(1);
      await expect(current).toBeVisible();

      // The current crumb is the LAST item in the ordered trail.
      const lastIsCurrent = await items.nth(total - 1).evaluate(
        (el) => el.getAttribute("aria-current") === "page",
      );
      expect(lastIsCurrent, "the current crumb is the last crumb").toBe(true);

      // Every crumb BEFORE the last is a link that RESOLVES to an existing local
      // mockup page (rejects empty/#, javascript:, remote, and broken paths —
      // round 2, finding 3).
      for (let i = 0; i < total - 1; i++) {
        const item = items.nth(i);
        const tag = await item.evaluate((el) => el.tagName.toLowerCase());
        expect(tag, `non-current crumb #${i} is a link`).toBe("a");
        const href = (await item.getAttribute("href")) ?? "";
        expect(href, `crumb #${i} has a real href (not empty/#)`).not.toMatch(/^#?$/);
        const resolved = new URL(href, page.url());
        expect(resolved.protocol, `crumb #${i} is a local file: link`).toBe("file:");
        expect(resolved.pathname, `crumb #${i} targets an .html page`).toMatch(/\.html$/);
        expect(resolved.href, `crumb #${i} navigates away from the current page`).not.toBe(page.url());
        expect(
          fs.existsSync(fileURLToPath(resolved.href)),
          `crumb #${i} target exists on disk: ${resolved.href}`,
        ).toBe(true);
      }

      // The current crumb names the current page.
      await expect(current, `current crumb names "${name}"`).toContainText(CURRENT_CRUMB_NAME[name]);
    });
  }

  // ── m2.4 (click) — a representative non-current crumb navigates to its target ──
  test("m2.4 story-detail: clicking the epic crumb navigates to the epic page", async ({ page }) => {
    await openMockup(page, "story-detail");
    const epicCrumb = page.getByTestId(MOCKUP_TID.breadcrumb).locator('a[href$="epic.html"]').first();
    await expect(epicCrumb).toBeVisible();
    await epicCrumb.click();
    await expect(page).toHaveURL(/epic\.html$/);
    await expect(page.getByText("Inventory Spread Epic").first()).toBeVisible();
  });

  // ── m2.5 — return to sessions leaves the workspace ──
  test("m2.5 sidebar return-to-sessions lands on the sessions list", async ({ page }) => {
    await openMockup(page, "workspace-overview");
    await page.getByTestId(MOCKUP_TID.sidebar).locator('a[href$="sessions.html"]').first().click();
    await expect(page).toHaveURL(/sessions\.html$/);
    await expect(page.getByTestId(MOCKUP_TID.sessionRow).first()).toBeVisible();
  });

  // ── m2.6 — story detail exposes the raw story file with REAL content ──
  test("m2.6 story-detail reveals the raw 60.2 file content on activation", async ({ page }) => {
    await openMockup(page, "story-detail");
    const rawControl = page
      .getByRole("button", { name: /raw/i })
      .or(page.getByRole("tab", { name: /raw/i }))
      .or(page.getByRole("link", { name: /raw/i }))
      .or(page.locator("summary").filter({ hasText: /raw/i }));
    await expect(rawControl.first()).toBeVisible();

    const raw = page.getByTestId(MOCKUP_TID.storyRaw);
    // The raw view starts hidden — activating the control reveals it (state change).
    await expect(raw).toBeHidden();
    await rawControl.first().click();
    await expect(raw).toBeVisible();

    // Specific signature lines from the actual 60.2 story file (fixture-exact).
    await expect(raw).toContainText("i_want");
    await expect(raw).toContainText(
      "Claude to give me a short summary of a shared Google Doc when I ask for one",
    );
    await expect(raw).toContainText("at least 500 words of body text");
  });

  // ── m2.7 — sidebar sections expand/collapse in place (native <details>) ──
  test("m2.7 Product section expands in place; the epic expands to its stories", async ({ page }) => {
    await openMockup(page, "workspace-overview");
    const product = page.getByTestId(sidebarSectionId("product"));
    await expect(product).toHaveCount(1);

    // Starts collapsed: the epic node link is hidden.
    const epicLink = product.locator('a[href$="epic.html"]').first();
    await expect(epicLink).toBeHidden();

    // Expand the Product section via its native <summary> toggle.
    await product.locator("summary").first().click();
    await expect(epicLink).toBeVisible();

    // The epic further expands to reveal its story nodes 60.1/60.2/60.3.
    const story602 = product.locator('a[href*="story-detail"]').filter({ hasText: "60.2" }).first();
    await expect(story602).toBeHidden();
    await product.locator("summary").filter({ hasText: /Inventory Spread Epic/i }).first().click();
    await expect(story602).toBeVisible();
    for (const id of ["60.1", "60.3"]) {
      await expect(product.getByText(id).first()).toBeVisible();
    }
  });

  // ── m2.7b — the ARCHITECTURE section also expands/collapses in place ──
  // (final review, finding 4: "each sidebar section expands" must not hold for
  // Product alone. Requirements is exempt — the brief mandates it FLAT, m1.5.)
  test("m2.7b Architecture section expands in place to its service + vocabulary nodes", async ({ page }) => {
    await openMockup(page, "workspace-overview");
    const architecture = page.getByTestId(sidebarSectionId("architecture"));
    await expect(architecture).toHaveCount(1);

    // Starts collapsed: the service node link is hidden.
    const serviceLink = architecture.locator('a[href$="service.html"]').first();
    await expect(serviceLink).toBeHidden();

    // Expand via the native <summary> toggle: both child nodes become visible.
    await architecture.locator("summary").first().click();
    await expect(serviceLink).toBeVisible();
    await expect(
      architecture.locator('a[href$="vocabulary.html"]').first(),
    ).toBeVisible();
  });

  // ── m2.7c — the REQUIREMENTS and RUNS sections are expandable too ──
  // (final review round 2: "expand each sidebar section in place" covers every
  // section — Requirements' content is a FLAT row list (m1.5), but the section
  // itself still collapses/expands like its siblings.)
  for (const [section, childHref] of [
    ["requirements", "scenario-detail.html"],
    ["runs", "runs.html"],
  ] as const) {
    test(`m2.7c the ${section} section expands in place to its child rows`, async ({ page }) => {
      await openMockup(page, "workspace-overview");
      const sec = page.getByTestId(sidebarSectionId(section));
      await expect(sec).toHaveCount(1);
      const childLink = sec.locator(`a[href$="${childHref}"]`).first();
      await expect(childLink).toBeHidden();
      await sec.locator("summary").first().click();
      await expect(childLink).toBeVisible();
    });
  }

  // ── m2.8 — breadcrumbs carry the SECTION level where a section page exists ──
  // (final review, finding 1: the brief's trail is session / section / epic /
  // story. Product-tree pages must crumb to the Product root, scenario detail
  // to the flat scenario list. Architecture leaves have no section root page
  // in this 17-page iteration — documented exception in SPEC.md §9.)
  for (const [name, section] of [
    ["epic", { label: /product/i, target: "prd-overview.html" }],
    ["story-detail", { label: /product/i, target: "prd-overview.html" }],
    ["story-ambiguous", { label: /product/i, target: "prd-overview.html" }],
    ["story-invalid", { label: /product/i, target: "prd-overview.html" }],
    ["scenario-detail", { label: /requirements|scenarios/i, target: "scenarios.html" }],
  ] as const) {
    test(`m2.8 ${name}: breadcrumb carries its section crumb`, async ({ page }) => {
      await openMockup(page, name);
      const crumb = page.getByTestId(MOCKUP_TID.breadcrumb);
      const sectionCrumb = crumb
        .locator("a[href]")
        .filter({ hasText: section.label })
        .first();
      await expect(sectionCrumb, "the section crumb exists and is a link").toBeVisible();
      const href = (await sectionCrumb.getAttribute("href")) ?? "";
      expect(new URL(href, page.url()).pathname.endsWith(section.target),
        `section crumb targets ${section.target} (got: ${href})`).toBe(true);
    });
  }
});
