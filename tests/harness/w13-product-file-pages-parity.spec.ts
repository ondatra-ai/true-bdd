/**
 * w13 — Product file-page parity (task `product-section-prototype-parity`,
 * R3/R4/R6/R7). Deterministic structural specs pinning the prototype's
 * GitHub-style file-card anatomy (kicker / title / muted subtitle / header bar
 * with path + `N lines` counter / gutter beside a monospace body), the story
 * page's kicker+title+Feature pill+deep breadcrumb, and — per the orchestrator's
 * R7 ruling — the scenarios registry TABLE.
 *
 * RED intent (R9): current prod renders a BARE `FileView` (path header + gutter
 * + editor only) on /product, /product/features, and /product/scenarios, and a
 * bare `Select feature…` button + `Home / Product / Stories / <id>` breadcrumb
 * on a story page. Every case fails on a MISSING testid / wrong text — an
 * assertion failure, not a crash.
 */

import { parse } from "yaml";
import { expect, test } from "@playwright/test";

import {
  WTID,
  gotoWorkspace,
  readEditor,
  saveState,
  scenarioTableRow,
  wsRoutes,
  writeEditor,
} from "./helpers/ui";
import { WorkspaceEnv, runToken } from "./helpers/workspace-env";

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

function escapeRe(value: string): string {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

/**
 * Asserts the prototype file-card anatomy (R3/R6, F5/F6): kicker OVER title OVER
 * muted subtitle, then a header BAR containing the doc path + an `N lines`
 * counter (N == the editor buffer's `split("\n").length` — the prototype
 * formula), with the gutter + editor siblings in the card body.
 */
async function assertFileCard(
  page: import("@playwright/test").Page,
  opts: { kicker: RegExp; title: string; docPath: string },
): Promise<void> {
  const kicker = page.getByTestId(WTID.fileViewKicker);
  const title = page.getByTestId(WTID.fileViewTitle);
  const meta = page.getByTestId(WTID.fileViewMeta);

  await expect(kicker).toBeVisible();
  await expect(kicker).toHaveText(opts.kicker);
  await expect(title).toHaveText(opts.title);

  // Kicker ABOVE title ABOVE meta (structural order, boundingBox).
  const kickerBox = (await kicker.boundingBox())!;
  const titleBox = (await title.boundingBox())!;
  expect(kickerBox.y).toBeLessThan(titleBox.y);

  // Muted subtitle: non-empty, color = --text-muted, BELOW the title (F6: a
  // plain body paragraph must not pass).
  await expect(meta).toBeVisible();
  await expect(meta).not.toBeEmpty();
  const metaBox = (await meta.boundingBox())!;
  expect(metaBox.y).toBeGreaterThan(titleBox.y);
  const metaColor = await meta.evaluate((el) => getComputedStyle(el).color);
  const mutedToken = await page.evaluate(() => {
    const probe = document.createElement("span");
    probe.style.color = getComputedStyle(document.documentElement).getPropertyValue("--text-muted").trim();
    document.body.appendChild(probe);
    const rgb = getComputedStyle(probe).color;
    probe.remove();

    return rgb;
  });
  expect(metaColor).toBe(mutedToken);

  // File-card header bar CONTAINS the path + the `N lines` counter (F5).
  const header = page.getByTestId(WTID.fileViewHeader);
  await expect(header).toBeVisible();
  await expect(header.getByTestId(WTID.fileViewPath)).toHaveText(opts.docPath);
  const lineCount = header.getByTestId(WTID.fileViewLineCount);
  await expect(lineCount).toHaveText(/^\d+\s+lines$/);
  const expectedLines = (await readEditor(page.getByTestId(WTID.fileViewEditor))).split("\n").length;
  await expect(lineCount).toHaveText(new RegExp(`^${expectedLines}\\s+lines$`));

  // Gutter + editor are siblings inside a card body (both present).
  await expect(page.getByTestId(WTID.fileViewGutter)).toBeVisible();
  await expect(page.getByTestId(WTID.fileViewEditor)).toBeVisible();
}

test("w13.1 the product landing renders prd.yaml as a GitHub-style file card (R3)", async ({ page }) => {
  env = await WorkspaceEnv.start("w13-product-landing");
  const e = env;

  await gotoWorkspace(page, e.baseURL, wsRoutes.product(e.sid));
  await assertFileCard(page, {
    kicker: /02\s*[—-]\s*PRODUCT/i,
    title: "prd.yaml",
    docPath: "docs/prd/prd.yaml",
  });

  // Architecture-untouched guard (round-3 fresh #2): the SHARED FileView must
  // NOT leak the product page-header onto the architecture route.
  await gotoWorkspace(page, e.baseURL, wsRoutes.architecture(e.sid));
  await expect(page.getByTestId(WTID.fileView)).toBeVisible();
  await expect(page.getByTestId(WTID.fileViewKicker)).toHaveCount(0);
  await expect(page.getByTestId(WTID.fileViewTitle)).toHaveCount(0);

  // Edit round-trip preserved (F17): appending a unique YAML comment still
  // persists valid YAML through the relay.
  await gotoWorkspace(page, e.baseURL, wsRoutes.product(e.sid));
  const editor = page.getByTestId(WTID.fileViewEditor);
  const token = runToken("edit");
  const current = await readEditor(editor);
  await writeEditor(editor, `${current}\n# ${token}\n`);
  await expect(editor).toContainText(token);
  await expect(saveState(page)).toHaveAttribute("data-save-state", "saved");
  const bytes = await e.waitForDocOnDisk("docs/prd/prd.yaml", (b) => b.includes(token));
  expect(parse(bytes)).toBeTruthy();
});

test("w13.2 a story page renders kicker + title, a Feature pill, and the deep breadcrumb (R4)", async ({ page }) => {
  env = await WorkspaceEnv.start("w13-story-page");
  const e = env;
  const seeded = e.seedFeatureOnDisk("pill");

  await gotoWorkspace(page, e.baseURL, wsRoutes.story(e.sid, "60.1"));

  // RED-first (round-1 F4): the kicker is absent in current prod, so it is the
  // FIRST failing assertion — not the already-present editor.
  await expect(page.getByTestId(WTID.fileViewKicker)).toHaveText(/02\s*[—-]\s*PRODUCT\s*\/\s*60\.1/i);

  const editor = page.getByTestId(WTID.fileViewEditor);
  await expect(editor).toBeVisible();
  const story = parse(await readEditor(editor)) as { story: { id: string; title: string; feature?: string } };
  await expect(page.getByTestId(WTID.fileViewTitle)).toHaveText(
    new RegExp(`^${escapeRe(story.story.id)}\\s*[—–-]\\s*${escapeRe(story.story.title)}$`),
  );

  // The Feature pill has DISTINCT anatomy (F9): three located parts inside the
  // collapsed toggle — a single unstyled text button must not pass.
  const toggle = page.getByTestId(WTID.featurePickerToggle);
  await expect(toggle.getByTestId(WTID.featurePillLabel)).toHaveText(/feature:/i);
  await expect(toggle.getByTestId(WTID.featurePillValue)).toHaveText(story.story.feature ?? "");
  await expect(toggle.getByTestId(WTID.featurePillChange)).toHaveText(/change/i);

  // Only ACTIVATING the control reveals the searchable picker.
  await expect(page.getByTestId(WTID.featurePickerInput)).toBeHidden();
  await toggle.click();
  await expect(page.getByTestId(WTID.featurePickerInput)).toBeVisible();

  // The pill (bearing CHANGE) is OPERABLE, not decorative (round-2 fresh #2):
  // picking a seeded feature writes the story's `feature:` to disk.
  await page.getByTestId(WTID.featurePickerInput).fill(seeded);
  await page.locator(`[data-testid="${WTID.featurePickerOption}"][data-feature="${seeded}"]`).click();
  const bytes = await e.waitForDocOnDisk("docs/prd/stories/60.1-summary-length-preference.yaml", (b) =>
    new RegExp(`feature:\\s*${seeded}`).test(b),
  );
  expect((parse(bytes) as { story: { feature: string } }).story.feature).toBe(seeded);

  // The deep breadcrumb trail: Sessions / Workspace overview / Product / <file>.
  const crumbs = page.getByTestId(WTID.contentBreadcrumb).locator('a, [aria-current="page"]');
  await expect(crumbs).toHaveText([
    "Sessions",
    "Workspace overview",
    "Product",
    "60.1-summary-length-preference.yaml",
  ]);
  await expect(page.getByTestId(WTID.contentBreadcrumb).locator('[aria-current="page"]')).toHaveText(
    "60.1-summary-length-preference.yaml",
  );
});

test("w13.3 the features page renders features.yaml as a file card (R6)", async ({ page }) => {
  env = await WorkspaceEnv.start("w13-features-page");
  const e = env;

  await gotoWorkspace(page, e.baseURL, wsRoutes.features(e.sid));
  await assertFileCard(page, {
    kicker: /02\s*[—-]\s*PRODUCT\s*\/\s*FEATURES/i,
    title: "features.yaml",
    docPath: "docs/prd/features.yaml",
  });
});

test("w13.4 the scenarios page renders the Requirements/Scenarios TABLE, one row per registry entry (R7)", async ({
  page,
}) => {
  env = await WorkspaceEnv.start("w13-scenarios-table");
  const e = env;

  await gotoWorkspace(page, e.baseURL, wsRoutes.scenarios(e.sid));

  // The scenarios surface is the TABLE anatomy (orchestrator R7 ruling), NOT a
  // FileView — absent in current prod (which renders scenarios.yaml as a file).
  const table = page.getByTestId(WTID.scenarioTable);
  await expect(table).toBeVisible();

  // Heading + flat-registry subtitle.
  await expect(page.getByRole("heading", { name: /Requirements\s*\/\s*Scenarios/i })).toBeVisible();
  await expect(page.getByText(/flat registry/i)).toBeVisible();

  // Four columns in order.
  await expect(table.getByRole("columnheader")).toHaveText([/scenario/i, /description/i, /service/i, /linked story/i]);

  // One row per docs/scenarios.yaml entry (data-driven from the served registry).
  const registry = parse(e.readDocOnDisk("docs/scenarios.yaml")) as {
    scenarios: Record<string, { description: string; service: string; user_stories?: Array<{ story: string }> }>;
  };
  const ids = Object.keys(registry.scenarios);
  await expect(page.getByTestId(WTID.scenarioTableRow)).toHaveCount(ids.length);
  for (const id of ids) {
    await expect(scenarioTableRow(page, id)).toBeVisible();
  }

  // EVERY row's full cell/link anatomy (round-2 #1: not just E2E-601 — a fixer
  // could otherwise render the other rows as empty shells). The SCENARIO cell is
  // a link (text = id); DESCRIPTION + SERVICE cells = the parsed values; the
  // LINKED STORY cell is a link to that scenario's story (derived from
  // user_stories → the story file's own id) — OR, for a scenario with EMPTY
  // user_stories (E2E-604, INT-901), the specified empty state: NO linked-story link.
  for (const id of ids) {
    const row = scenarioTableRow(page, id);
    const idLink = row.getByTestId(WTID.scenarioIdLink);
    await expect(idLink).toHaveText(id);
    // The SCENARIO cell is a real LINK, not a bare text node (reviewer R1/r2-1:
    // a regen must not silently degrade it to a `<span>` — a `<span href>` would
    // satisfy toHaveAttribute alone, so pin the anchor ROLE too, plus its
    // self-referential registry destination).
    await expect(idLink).toHaveRole("link");
    await expect(idLink).toHaveAttribute("href", /\/product\/scenarios$/);
    await expect(row.getByTestId(WTID.scenarioDescriptionCell)).toHaveText(registry.scenarios[id].description);
    await expect(row.getByTestId(WTID.scenarioServiceCell)).toHaveText(registry.scenarios[id].service);

    const userStories = registry.scenarios[id].user_stories ?? [];
    const storyLink = row.getByTestId(WTID.scenarioLinkedStoryLink);
    if (userStories.length > 0) {
      const linkedId = (parse(e.readDocOnDisk(userStories[0].story)) as { story: { id: string } }).story.id;
      // EXACT text (round-3 #1: an unanchored regex would admit "60.2 extra").
      await expect(storyLink).toHaveText(linkedId);
      await expect(storyLink).toHaveAttribute("href", new RegExp(`/product/stories/${escapeRe(linkedId)}$`));
    } else {
      await expect(storyLink).toHaveCount(0);
    }
  }
});

test("w13.5 an unknown story route renders a not-found body under the GENERIC (non-deep) breadcrumb (R4 fallback)", async ({
  page,
}) => {
  // Reviewer-added regeneratability guard (r2-2): a stale/broken story link is a
  // reachable state — StoryPage renders a "not found" body INSIDE the persistent
  // shell (it does NOT throw/404), so WorkspaceShell still derives a breadcrumb.
  // With no resolvable file basename, `breadcrumbCrumbs` must fall through to the
  // GENERIC `Home / Product / Stories / <id>` trail (NOT the deep Sessions/
  // Workspace-overview trail, NOT a crash). Only breadcrumb.test.ts (a gitignored
  // unit) pinned this; a regenerate-from-tests could otherwise lose the resilience.
  env = await WorkspaceEnv.start("w13-unknown-story");
  const e = env;

  await gotoWorkspace(page, e.baseURL, wsRoutes.story(e.sid, "99.9"));

  // The shell survives: the not-found body renders (no crash, no blank page).
  await expect(page.getByText(/story\s*99\.9\s*not found/i)).toBeVisible();

  // The breadcrumb is the GENERIC fallback trail, not the deep story trail: it
  // leads with "Home" (not "Sessions"), never mentions "Workspace overview",
  // links Home + Product, keeps the non-routable "Stories" crumb, and marks the
  // unknown id as the current page.
  const bc = page.getByTestId(WTID.contentBreadcrumb);
  await expect(bc).toBeVisible();
  await expect(bc.getByRole("link")).toHaveText(["Home", "Product"]);
  await expect(bc).toContainText("Stories");
  await expect(bc).not.toContainText(/Workspace overview/i);
  await expect(bc.locator('[aria-current="page"]')).toHaveText("99.9");
});
