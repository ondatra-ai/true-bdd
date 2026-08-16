/**
 * w5 — product docs, features, and feature-driven alignment (P18–P25).
 *
 * Anti-placeholder discipline: parsed oracles read exact node keys/values scoped
 * to the right node (never a spanning regex); rendered-content cases seed a
 * unique per-run token; derived-view assertions require the content to be a
 * descendant of the correct row.
 */

import { parse } from "yaml";
import { expect, test } from "@playwright/test";

import {
  WTID,
  createFeatureIn,
  gotoWorkspace,
  pickFeatureIn,
  railItem,
  readEditor,
  sidebarFeatureRow,
  sidebarGroup,
  sidebarScenarioRow,
  sidebarSection,
  sidebarStoryRow,
  writeEditor,
  wsRoutes,
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

test("w5.1 PRD, features, story, scenarios each render as a file page with verbatim content + token (P18)", async ({
  page,
}) => {
  env = await WorkspaceEnv.start("w5-file-pages");
  const e = env;
  const cases: Array<[string, string, string]> = [
    [wsRoutes.product(e.sid), "docs/product/product.yaml", e.seedTokenInDoc("docs/product/product.yaml")],
    [wsRoutes.features(e.sid), "docs/product/features.yaml", e.seedTokenInDoc("docs/product/features.yaml")],
    [
      wsRoutes.story(e.sid, "60.2"),
      "docs/product/stories/60.2-summary-shared-docs.yaml",
      e.seedTokenInDoc("docs/product/stories/60.2-summary-shared-docs.yaml"),
    ],
    [wsRoutes.scenarios(e.sid), "docs/scenarios.yaml", e.seedTokenInDoc("docs/scenarios.yaml")],
  ];

  for (const [route, docPath, token] of cases) {
    await gotoWorkspace(page, e.baseURL, route);
    await expect(page.getByTestId(WTID.fileViewPath)).toHaveText(docPath);
    await expect(page.getByTestId(WTID.fileViewEditor)).toContainText(token);
  }
});

test("w5.1a Product outline: four groups in order, fixture counts, header/rows navigate, live add (P18a)", async ({
  page,
}) => {
  env = await WorkspaceEnv.start("w5-outline");
  const e = env;

  await gotoWorkspace(page, e.baseURL, wsRoutes.product(e.sid));
  const product = sidebarSection(page, "product");

  const groups = product.getByTestId(WTID.sidebarGroup);
  await expect(groups).toHaveCount(4);
  await expect(groups.nth(0)).toHaveAttribute("data-group", "PRD");
  await expect(groups.nth(1)).toHaveAttribute("data-group", "Features");
  await expect(groups.nth(2)).toHaveAttribute("data-group", "Stories");
  await expect(groups.nth(3)).toHaveAttribute("data-group", "Scenarios");

  await expect(sidebarGroup(product, "Stories").getByTestId(WTID.storyRow)).toHaveCount(3);
  await expect(sidebarGroup(product, "Features").getByTestId(WTID.featureRow)).toHaveCount(3);
  await expect(sidebarGroup(product, "Scenarios").getByTestId(WTID.scenarioRow)).toHaveCount(5);

  // Features HEADER → features.yaml file page; a feature ROW → its aggregation page.
  await sidebarGroup(product, "Features").getByTestId(WTID.sidebarGroupName).click();
  await expect(page).toHaveURL(new RegExp(`/sessions/${e.sid}/product/features$`));
  await gotoWorkspace(page, e.baseURL, wsRoutes.product(e.sid));
  await sidebarFeatureRow(sidebarSection(page, "product"), "summaries").click();
  await expect(page).toHaveURL(new RegExp(`/sessions/${e.sid}/product/features/summaries$`));

  // Live add via the P22 create-story UI → a new Stories row without reload.
  await gotoWorkspace(page, e.baseURL, wsRoutes.product(e.sid));
  await page.getByTestId(WTID.newStoryOpen).click();
  const form = page.getByTestId(WTID.newStoryForm);
  await form.getByTestId(WTID.newStoryTitle).fill("Live added story");
  await pickFeatureIn(form, "summaries");
  await form.getByTestId(WTID.newStorySubmit).click();
  await expect(sidebarGroup(sidebarSection(page, "product"), "Stories").getByTestId(WTID.storyRow)).toHaveCount(4);

  // Within-file typing adds a Scenarios row live (typing CAN drive this).
  await gotoWorkspace(page, e.baseURL, wsRoutes.scenarios(e.sid));
  const editor = page.getByTestId(WTID.fileViewEditor);
  const scnId = `E2E-${Math.floor(Math.random() * 9000) + 1000}`;
  const current = await readEditor(editor);
  await writeEditor(
    editor,
    current.replace(/\n {2}INT-901:/, `\n  ${scnId}:\n    description: "live"\n    service: "mcp-service"\n    user_stories: []\n  INT-901:`),
  );
  await expect(sidebarScenarioRow(sidebarSection(page, "product"), scnId)).toBeVisible();
});

test("w5.2 stories are flat siblings; no epic testid exists anywhere in the workspace (P19)", async ({ page }) => {
  env = await WorkspaceEnv.start("w5-flat");
  const e = env;

  await gotoWorkspace(page, e.baseURL, wsRoutes.product(e.sid));
  const rows = sidebarGroup(sidebarSection(page, "product"), "Stories").getByTestId(WTID.storyRow);
  await expect(rows).toHaveCount(3);
  const distinctParents = await rows.evaluateAll((els) => new Set(els.map((el) => el.parentElement)).size);
  expect(distinctParents).toBe(1); // flat: all share one parent

  await expect(page.locator('[data-testid^="epic"]')).toHaveCount(0);
});

test("w5.3 the SERVED features.yaml is exactly id+description; served story/scenario carry feature refs (P20)", async ({
  page,
}) => {
  env = await WorkspaceEnv.start("w5-schema");
  const e = env;

  // Parse the content the WORKSPACE SERVES (the rendered editor buffer), so this
  // gates the workspace behavior — not merely the seed fixture on disk.
  await gotoWorkspace(page, e.baseURL, wsRoutes.features(e.sid));
  const features = parse(await readEditor(page.getByTestId(WTID.fileViewEditor))) as {
    features: Record<string, unknown>[];
  };
  expect(features.features.length).toBeGreaterThan(0);
  for (const rec of features.features) {
    expect(Object.keys(rec).sort()).toEqual(["description", "id"]);
  }

  await gotoWorkspace(page, e.baseURL, wsRoutes.story(e.sid, "60.2"));
  const story = parse(await readEditor(page.getByTestId(WTID.fileViewEditor))) as { story: { feature?: string } };
  expect(story.story.feature).toBeTruthy();

  await gotoWorkspace(page, e.baseURL, wsRoutes.scenarios(e.sid));
  const scenarios = parse(await readEditor(page.getByTestId(WTID.fileViewEditor))) as {
    scenarios: Record<string, { feature?: string }>;
  };
  expect(scenarios.scenarios["E2E-601"].feature).toBeTruthy();
});

test("w5.4 feature aggregation shows description + tagged rows; reassign by picker AND by hand re-buckets live (P21)", async ({
  page,
}) => {
  env = await WorkspaceEnv.start("w5-aggregation");
  const e = env;
  const target = e.seedFeatureOnDisk("agg"); // unique per-run reassignment target (defeats a hardcoded picker)

  await gotoWorkspace(page, e.baseURL, wsRoutes.feature(e.sid, "summaries"));
  await expect(page.getByTestId(WTID.featureDescription)).toContainText(/length preferences|summaries/i);

  const stories = page.getByTestId(WTID.featureStoriesList);
  await expect(stories.locator(`[data-testid="${WTID.featureStoryRow}"][data-story-id="60.1"]`)).toBeVisible();
  await expect(stories.locator(`[data-testid="${WTID.featureStoryRow}"][data-story-id="60.2"]`)).toBeVisible();
  await expect(
    page.getByTestId(WTID.featureScenariosList).locator(`[data-testid="${WTID.featureScenarioRow}"][data-scenario-id="E2E-601"]`),
  ).toBeVisible();

  // Reassign 60.1 → the UNIQUE feature via ITS row picker; it leaves this bucket
  // live AND persists (node-scoped parse).
  const row60_1 = stories.locator(`[data-testid="${WTID.featureStoryRow}"][data-story-id="60.1"]`);
  await pickFeatureIn(row60_1, target);
  await expect(stories.locator(`[data-testid="${WTID.featureStoryRow}"][data-story-id="60.1"]`)).toHaveCount(0);
  const s1 = await e.waitForDocOnDisk("docs/product/stories/60.1-summary-length-preference.yaml", (b) =>
    new RegExp(`feature:\\s*${target}`).test(b),
  );
  expect((parse(s1) as { story: { feature: string } }).story.feature).toBe(target);

  // Reassign 60.2 to the SAME unique feature by HAND-editing its feature: line.
  // Navigate CLIENT-SIDE (rail + sidebar Links) so the re-bucket is proven LIVE
  // without a reload — a full page.goto would reload the aggregation from disk and
  // could not distinguish live derivation from a reload.
  await railItem(page, "product").click();
  await sidebarStoryRow(sidebarSection(page, "product"), "60.2").click();
  const editor = page.getByTestId(WTID.fileViewEditor);
  const current = await readEditor(editor);
  await writeEditor(editor, current.replace(/feature:\s*summaries/, `feature: ${target}`));
  const s2 = await e.waitForDocOnDisk("docs/product/stories/60.2-summary-shared-docs.yaml", (b) =>
    new RegExp(`feature:\\s*${target}`).test(b),
  );
  expect((parse(s2) as { story: { feature: string } }).story.feature).toBe(target);

  await railItem(page, "product").click();
  await sidebarFeatureRow(sidebarSection(page, "product"), "summaries").click();
  await expect(page).toHaveURL(new RegExp(`/sessions/${e.sid}/product/features/summaries$`));
  await expect(
    page.getByTestId(WTID.featureStoriesList).locator(`[data-testid="${WTID.featureStoryRow}"][data-story-id="60.2"]`),
  ).toHaveCount(0);
});

test("w5.4a PRD and features file pages are directly editable, live, and persisted (P9)", async ({ page }) => {
  env = await WorkspaceEnv.start("w5-any-doc");
  const e = env;

  for (const [route, docPath] of [
    [wsRoutes.product(e.sid), "docs/product/product.yaml"],
    [wsRoutes.features(e.sid), "docs/product/features.yaml"],
  ] as const) {
    await gotoWorkspace(page, e.baseURL, route);
    const editor = page.getByTestId(WTID.fileViewEditor);
    await expect(editor).toBeVisible();
    const token = runToken("edit");
    const current = await readEditor(editor);
    await writeEditor(editor, `${current}\n# ${token}\n`);
    await expect(editor).toContainText(token);
    const bytes = await e.waitForDocOnDisk(docPath, (b) => b.includes(token));
    expect(parse(bytes)).toBeTruthy(); // still parses (persisted valid)
  }
});

test("w5.5 new story requires a feature; search-pick FILTERS; create-new appends an id+description stub (P22)", async ({
  page,
}) => {
  env = await WorkspaceEnv.start("w5-new-story");
  const e = env;

  await gotoWorkspace(page, e.baseURL, wsRoutes.product(e.sid));
  await page.getByTestId(WTID.newStoryOpen).click();
  const form = page.getByTestId(WTID.newStoryForm);

  // Required: submitting without a feature is BLOCKED. The authoritative oracle
  // is the full-doc byte snapshot (catches an overwrite/create via ANY route);
  // the form stays open and no story row appears.
  await form.getByTestId(WTID.newStoryTitle).fill("Story without a feature");
  const before = e.snapshotDocs();
  await form.getByTestId(WTID.newStorySubmit).click();
  expect(e.changedDocsSince(before)).toEqual([]);
  await expect(form).toBeVisible();
  await expect(sidebarGroup(sidebarSection(page, "product"), "Stories").getByTestId(WTID.storyRow)).toHaveCount(3);

  // (a) search + pick an existing feature — the option list FILTERS (a static
  // <select> would fail: the nonmatching feature must be ABSENT).
  await form.getByTestId(WTID.featurePickerToggle).click();
  await form.getByTestId(WTID.featurePickerInput).fill("error");
  await expect(form.locator(`[data-testid="${WTID.featurePickerOption}"][data-feature="error-handling"]`)).toBeVisible();
  await expect(form.locator(`[data-testid="${WTID.featurePickerOption}"][data-feature="summaries"]`)).toHaveCount(0);
  await form.locator(`[data-testid="${WTID.featurePickerOption}"][data-feature="error-handling"]`).click();
  await form.getByTestId(WTID.newStorySubmit).click();

  const pickRel = await e.waitForStoryFile((bytes) => /Story without a feature/i.test(bytes));
  expect((parse(e.readDocOnDisk(pickRel)) as { story: { feature: string } }).story.feature).toBe("error-handling");

  // (b) "+ Create" a NOVEL feature → id+description stub in features.yaml AND an
  // exclusive-created story file carrying feature: <slug>.
  await gotoWorkspace(page, e.baseURL, wsRoutes.product(e.sid));
  await page.getByTestId(WTID.newStoryOpen).click();
  const form2 = page.getByTestId(WTID.newStoryForm);
  await form2.getByTestId(WTID.newStoryTitle).fill("Story with a fresh feature");
  const novel = runToken("feat");
  await createFeatureIn(form2, novel);
  await form2.getByTestId(WTID.newStorySubmit).click();

  const feat = await e.waitForDocOnDisk("docs/product/features.yaml", (b) => b.includes(novel));
  const rec = (parse(feat) as { features: Array<{ id: string }> }).features.find((f) => f.id === novel);
  expect(rec && Object.keys(rec).sort()).toEqual(["description", "id"]);
  await e.waitForStoryFile((bytes) => (parse(bytes) as { story?: { feature?: string } })?.story?.feature === novel);
});

test("w5.6 changing a story's feature via the searchable picker writes it to the story YAML (P23)", async ({ page }) => {
  env = await WorkspaceEnv.start("w5-change-feature");
  const e = env;

  await gotoWorkspace(page, e.baseURL, wsRoutes.story(e.sid, "60.2"));
  await pickFeatureIn(page, "error-handling");

  const bytes = await e.waitForDocOnDisk("docs/product/stories/60.2-summary-shared-docs.yaml", (b) =>
    /feature:\s*error-handling/.test(b),
  );
  expect((parse(bytes) as { story: { feature: string } }).story.feature).toBe("error-handling");
});

test("w5.7 INT-901 (no feature) is in the unaligned bucket; retro-tag sets its node's feature and removes it (P24)", async ({
  page,
}) => {
  env = await WorkspaceEnv.start("w5-unaligned");
  const e = env;
  const target = e.seedFeatureOnDisk("retro"); // unique per-run retro-tag target

  await gotoWorkspace(page, e.baseURL, wsRoutes.feature(e.sid, "summaries"));
  const bucket = page.getByTestId(WTID.unalignedBucket);
  const row = bucket.locator(`[data-testid="${WTID.unalignedScenarioRow}"][data-scenario-id="INT-901"]`);
  await expect(row).toBeVisible();

  await pickFeatureIn(row, target);

  const bytes = await e.waitForDocOnDisk("docs/scenarios.yaml", (b) => {
    const doc = parse(b) as { scenarios: Record<string, { feature?: string }> };

    return doc.scenarios["INT-901"]?.feature === target;
  });
  // Scoped to the INT-901 node (not a spanning regex).
  expect((parse(bytes) as { scenarios: Record<string, { feature?: string }> }).scenarios["INT-901"].feature).toBe(
    target,
  );
  await expect(bucket.locator(`[data-scenario-id="INT-901"]`)).toHaveCount(0);
});

test("w5.8 a dangling feature ref (E2E-604 → ghost) surfaces in the unaligned/dangling bucket with a marker (P25)", async ({
  page,
}) => {
  env = await WorkspaceEnv.start("w5-dangling");
  const e = env;

  await gotoWorkspace(page, e.baseURL, wsRoutes.feature(e.sid, "summaries"));
  const row = page
    .getByTestId(WTID.unalignedBucket)
    .locator(`[data-testid="${WTID.unalignedScenarioRow}"][data-scenario-id="E2E-604"]`);
  await expect(row).toBeVisible();
  await expect(row).toHaveAttribute("data-dangling", "true");
  await expect(row).toHaveAttribute("data-dangling-ref", "ghost");
});
