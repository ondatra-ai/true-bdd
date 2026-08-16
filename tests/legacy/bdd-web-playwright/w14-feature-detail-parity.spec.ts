/**
 * w14 — feature-detail aggregation parity (task
 * `product-section-prototype-parity`, R5). Deterministic structural spec pinning
 * the prototype's kicker, the description-from-features.yaml subtitle, the three
 * RENAMED sections (User stories / Requirements / Unaligned requirements), and
 * the card-row anatomy where each row is a LINKED title on the left and a
 * `Feature:` pill control on the right.
 *
 * RED intent (R9): current prod headings are `Stories`/`Requirements`/
 * `Unaligned`; story rows are a bare `<span>id</span>` (no linked title);
 * Requirements rows are plain text with NO link and NO picker; unaligned rows
 * show no linked title and no `(none)` pill; there is no kicker. Every failure
 * is an assertion, not a crash.
 */

import { parse } from "yaml";
import { expect, test } from "@playwright/test";

import { WTID, gotoWorkspace, lineIndex, pickFeatureIn, wsRoutes } from "./helpers/ui";
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

/**
 * Asserts a row's card anatomy is a LINKED title on the LEFT and its picker on
 * the RIGHT (round-2 #2 — a stacked/reversed layout would otherwise pass): the
 * link precedes the picker horizontally AND they share a row (vertical overlap).
 */
async function assertRowLayout(left: import("@playwright/test").Locator, right: import("@playwright/test").Locator): Promise<void> {
  // Establish visibility first so a hidden node fails as a clean assertion, not a
  // `boundingBox() → null` TypeError (round-3 #2).
  await expect(left).toBeVisible();
  await expect(right).toBeVisible();
  const leftBox = (await left.boundingBox())!;
  const rightBox = (await right.boundingBox())!;
  expect(leftBox.x, "the linked title must sit LEFT of the picker").toBeLessThan(rightBox.x);
  const overlap =
    Math.min(leftBox.y + leftBox.height, rightBox.y + rightBox.height) - Math.max(leftBox.y, rightBox.y);
  expect(overlap, "the linked title and the picker must share a row (not stacked)").toBeGreaterThan(0);
}

test("w14.1 the feature detail page aggregates linked-title rows under three named sections (R5)", async ({ page }) => {
  env = await WorkspaceEnv.start("w14-feature-detail");
  const e = env;
  const seeded = e.seedFeatureOnDisk("reassign"); // unique reassign target (defeats a hardcoded picker)

  // Oracles read from the SERVED docs on disk (data-driven, anti-hardcode).
  const features = (parse(e.readDocOnDisk("docs/product/features.yaml")) as { features: Array<{ id: string; description: string }> })
    .features;
  const summariesDescription = features.find((f) => f.id === "summaries")!.description;
  const storyTitle = (parse(e.readDocOnDisk("docs/product/stories/60.1-summary-length-preference.yaml")) as {
    story: { title: string };
  }).story.title;
  const registry = (parse(e.readDocOnDisk("docs/scenarios.yaml")) as {
    scenarios: Record<string, { description: string }>;
  }).scenarios;

  await gotoWorkspace(page, e.baseURL, wsRoutes.feature(e.sid, "summaries"));

  // Kicker (absent in current prod).
  await expect(page.getByTestId(WTID.featurePageKicker)).toHaveText(
    /02\s*[—-]\s*PRODUCT\s*\/\s*FEATURES\s*\/\s*SUMMARIES/i,
  );

  // Title = the feature id; description EQUALS the served features.yaml record
  // (not merely non-empty), muted + below the title.
  const title = page.locator("h1");
  await expect(title).toHaveText("summaries");
  const description = page.getByTestId(WTID.featureDescription);
  await expect(description).toHaveText(summariesDescription);
  const titleBox = (await title.boundingBox())!;
  const descBox = (await description.boundingBox())!;
  expect(descBox.y).toBeGreaterThan(titleBox.y);
  const descColor = await description.evaluate((el) => getComputedStyle(el).color);
  const mutedToken = await page.evaluate(() => {
    const probe = document.createElement("span");
    probe.style.color = getComputedStyle(document.documentElement).getPropertyValue("--text-muted").trim();
    document.body.appendChild(probe);
    const rgb = getComputedStyle(probe).color;
    probe.remove();

    return rgb;
  });
  expect(descColor).toBe(mutedToken);

  // Three RENAMED section headings, EXACT text.
  await expect(page.getByRole("heading", { name: "User stories", exact: true })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Requirements", exact: true })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Unaligned requirements", exact: true })).toBeVisible();

  // Story row 60.1: a LINK (id + story title) to the story page + a row-scoped picker.
  const storyRow = page
    .getByTestId(WTID.featureStoriesList)
    .locator(`[data-testid="${WTID.featureStoryRow}"][data-story-id="60.1"]`);
  const storyLink = storyRow.getByRole("link");
  await expect(storyLink).toContainText("60.1");
  await expect(storyLink).toContainText(storyTitle);
  await expect(storyLink).toHaveAttribute("href", /\/product\/stories\/60\.1$/);
  await expect(storyRow.getByTestId(WTID.featurePicker)).toBeVisible();

  // Requirement row E2E-601: a LINK (id + description) to the scenarios page +
  // a row-scoped picker (current prod rows have NEITHER).
  const reqRow = page
    .getByTestId(WTID.featureScenariosList)
    .locator(`[data-testid="${WTID.featureScenarioRow}"][data-scenario-id="E2E-601"]`);
  const reqLink = reqRow.getByRole("link");
  await expect(reqLink).toContainText("E2E-601");
  await expect(reqLink).toContainText(registry["E2E-601"].description);
  await expect(reqLink).toHaveAttribute("href", /\/product\/scenarios/);
  await expect(reqRow.getByTestId(WTID.featurePicker)).toBeVisible();

  // Unaligned row INT-901 (no feature): a LINK + a `Feature: (none)` pill (F8).
  const unalignedRow = page
    .getByTestId(WTID.unalignedBucket)
    .locator(`[data-testid="${WTID.unalignedScenarioRow}"][data-scenario-id="INT-901"]`);
  const unalignedLink = unalignedRow.getByRole("link");
  await expect(unalignedLink).toContainText("INT-901");
  await expect(unalignedLink).toContainText(registry["INT-901"].description);
  await expect(unalignedLink).toHaveAttribute("href", /\/product\/scenarios/);
  await expect(
    unalignedRow.getByTestId(WTID.featurePickerToggle).getByTestId(WTID.featurePillValue),
  ).toHaveText(/\(none\)/i);

  // Each row is a linked title on the LEFT + its picker on the RIGHT (round-2 #2),
  // asserted for a representative story, requirement, and unaligned row — BEFORE
  // any reassignment re-buckets them.
  await assertRowLayout(storyLink, storyRow.getByTestId(WTID.featurePicker));
  await assertRowLayout(reqLink, reqRow.getByTestId(WTID.featurePicker));
  await assertRowLayout(unalignedLink, unalignedRow.getByTestId(WTID.featurePicker));

  // Reassignment works via BOTH row kinds (F7 — exercises the NEW Requirements-
  // row picker prod currently lacks): each picks the seeded feature, persists to
  // disk, and re-buckets the row out of `summaries` live.
  await pickFeatureIn(storyRow, seeded);
  const storyBytes = await e.waitForDocOnDisk("docs/product/stories/60.1-summary-length-preference.yaml", (b) =>
    new RegExp(`feature:\\s*${seeded}`).test(b),
  );
  expect((parse(storyBytes) as { story: { feature: string } }).story.feature).toBe(seeded);
  await expect(
    page.getByTestId(WTID.featureStoriesList).locator(`[data-testid="${WTID.featureStoryRow}"][data-story-id="60.1"]`),
  ).toHaveCount(0);

  await pickFeatureIn(reqRow, seeded);
  const scnBytes = await e.waitForDocOnDisk("docs/scenarios.yaml", (b) => {
    const doc = parse(b) as { scenarios: Record<string, { feature?: string }> };

    return doc.scenarios["E2E-601"]?.feature === seeded;
  });
  expect((parse(scnBytes) as { scenarios: Record<string, { feature: string }> }).scenarios["E2E-601"].feature).toBe(
    seeded,
  );
  await expect(
    page
      .getByTestId(WTID.featureScenariosList)
      .locator(`[data-testid="${WTID.featureScenarioRow}"][data-scenario-id="E2E-601"]`),
  ).toHaveCount(0);
});

test("w14.2 a requirement/unaligned row link jumps to that scenario's EXACT line on the scenarios page (R5)", async ({
  page,
}) => {
  // Reviewer-added regeneratability guard: the requirement/unaligned-row title
  // links wire `requestJump(SCENARIOS_PATH, lineOfMapKey(...))` so landing on the
  // scenarios page scrolls to the EXACT scenario line (mirrors the sidebar's own
  // scenario jump + the prototype). w14.1 pins only the destination href, so a
  // regenerate-from-tests could drop the jump and stay green. This pins the jump
  // behaviorally (w3.3 flash pattern) for BOTH row kinds (separate JSX call sites).
  env = await WorkspaceEnv.start("w14-scenario-jump");
  const e = env;

  const scenarios = e.readDocOnDisk("docs/scenarios.yaml");
  // `lineOfMapKey` returns the 0-based buffer line of the `<id>:` key under
  // `scenarios:` (2-space indent) — the same 0-based index this regex finds.
  const e2eLine = lineIndex(scenarios, /^ {2}E2E-601:/);
  const intLine = lineIndex(scenarios, /^ {2}INT-901:/);

  // Requirement row (aligned to summaries) → scenarios page, flash on E2E-601.
  await gotoWorkspace(page, e.baseURL, wsRoutes.feature(e.sid, "summaries"));
  await page
    .getByTestId(WTID.featureScenariosList)
    .locator(`[data-testid="${WTID.featureScenarioRow}"][data-scenario-id="E2E-601"]`)
    .getByRole("link")
    .click();
  await expect(page).toHaveURL(new RegExp(`/sessions/${e.sid}/product/scenarios$`));
  const flash = page.getByTestId(WTID.fileViewFlash);
  await expect(flash).toBeVisible();
  await expect(flash).toHaveAttribute("data-line", String(e2eLine));

  // Unaligned row (no feature) → same exact-line jump for INT-901 (the second,
  // independent call site — a regen that wired only the requirement row fails here).
  await gotoWorkspace(page, e.baseURL, wsRoutes.feature(e.sid, "summaries"));
  await page
    .getByTestId(WTID.unalignedBucket)
    .locator(`[data-testid="${WTID.unalignedScenarioRow}"][data-scenario-id="INT-901"]`)
    .getByRole("link")
    .click();
  await expect(page).toHaveURL(new RegExp(`/sessions/${e.sid}/product/scenarios$`));
  await expect(page.getByTestId(WTID.fileViewFlash)).toHaveAttribute("data-line", String(intLine));
});
