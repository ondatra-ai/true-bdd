/**
 * w1 — workspace shell: icon rail, hover flyouts, docked sidebar (P1–P6, S2).
 *
 * RED intent: the greenfield harness has no workspace shell, so every case fails
 * on a missing rail/flyout/sidebar (assertions), not on an unresolved route — the
 * `(workspace)` route group + placeholder pages resolve, and the container is
 * healthy (200 on /). The session itself is coder behavior (agent register +
 * `GET /api/sessions`), so WorkspaceEnv.start surfaces that absence first.
 */

import { expect, test } from "@playwright/test";

import {
  WTID,
  gotoWorkspace,
  railItem,
  sidebarCaret,
  sidebarGroup,
  sidebarSection,
  sidebarStoryRow,
  wsRoutes,
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

test("w1.1 rail docks Architecture + marks it active; routes are App Router modules (P1/S2)", async ({ page }) => {
  env = await WorkspaceEnv.start("w1-shell-dock");
  const e = env;

  await gotoWorkspace(page, e.baseURL, wsRoutes.home(e.sid));
  await railItem(page, "architecture").click();

  await expect(sidebarSection(page, "architecture")).toBeVisible();
  await expect(sidebarSection(page, "product")).toBeHidden();
  await expect(railItem(page, "architecture")).toHaveAttribute("aria-current", "page");
  await expect(page).toHaveURL(new RegExp(`/sessions/${e.sid}/architecture$`));

  // S2 — a light structural check that the workspace route is served by a
  // Next App Router route module (the RSC flight-payload marker).
  const res = await page.request.get(new URL(wsRoutes.architecture(e.sid), e.baseURL).href);
  expect(res.status()).toBe(200);
  expect(await res.text()).toContain("__next_f");
});

test("w1.2 hover flyout previews a non-active tree; click inside navigates + docks (P2)", async ({ page }) => {
  env = await WorkspaceEnv.start("w1-flyout");
  const e = env;

  await gotoWorkspace(page, e.baseURL, wsRoutes.architecture(e.sid));

  await railItem(page, "product").hover();
  const flyout = page.getByTestId(WTID.railFlyout);
  await expect(flyout).toBeVisible();
  await expect(sidebarGroup(flyout, "Stories")).toBeVisible();

  const storyRow = flyout.getByTestId(WTID.storyRow).first();
  await storyRow.hover(); // pointer moves rail→flyout so the open/close delay keeps it up
  await storyRow.click();

  await expect(page).toHaveURL(new RegExp(`/sessions/${e.sid}/product/stories/`));
  await expect(page.getByTestId(WTID.railFlyout)).toBeHidden();
  await expect(sidebarSection(page, "product")).toBeVisible();
  await expect(railItem(page, "product")).toHaveAttribute("aria-current", "page");
});

test("w1.1b Home and Builds dock + activate + render their landings (P1/P3)", async ({ page }) => {
  env = await WorkspaceEnv.start("w1-home-builds");
  const e = env;

  await gotoWorkspace(page, e.baseURL, wsRoutes.architecture(e.sid));

  await railItem(page, "home").click();
  await expect(railItem(page, "home")).toHaveAttribute("aria-current", "page");
  await expect(page).toHaveURL(new RegExp(`/sessions/${e.sid}/home$`));
  await expect(page.getByTestId(WTID.homeLanding)).toBeVisible();

  await railItem(page, "builds").click();
  await expect(railItem(page, "builds")).toHaveAttribute("aria-current", "page");
  await expect(page).toHaveURL(new RegExp(`/sessions/${e.sid}/builds$`));
  await expect(page.getByTestId(WTID.buildsLanding)).toBeVisible();

  // Builds is navigation-only: no file editor, no chat mutation target.
  await expect(page.getByTestId(WTID.fileView)).toHaveCount(0);
});

test("w1.3 scenarios nest under Product; Builds is separate with no scenario rows (P3)", async ({ page }) => {
  env = await WorkspaceEnv.start("w1-nesting");
  const e = env;

  await gotoWorkspace(page, e.baseURL, wsRoutes.product(e.sid));
  const product = sidebarSection(page, "product");
  await expect(sidebarGroup(product, "Scenarios")).toBeVisible();
  await expect(product.getByTestId(WTID.scenarioRow).first()).toBeVisible();

  await expect(railItem(page, "builds")).toBeVisible();
  await railItem(page, "builds").click();
  await expect(sidebarSection(page, "builds")).toBeVisible();
  await expect(sidebarSection(page, "builds").getByTestId(WTID.scenarioRow)).toHaveCount(0);
});

test("w1.4 group row: hover caret toggles collapse (URL unchanged); name navigates (P4)", async ({ page }) => {
  env = await WorkspaceEnv.start("w1-split-click");
  const e = env;

  await gotoWorkspace(page, e.baseURL, wsRoutes.product(e.sid));
  const features = sidebarGroup(sidebarSection(page, "product"), "Features");
  const caret = sidebarCaret(features);

  await expect(caret).toBeHidden(); // no caret at rest
  await features.hover();
  await expect(caret).toBeVisible();
  await expect(caret).toHaveAttribute("data-expanded", "true");

  const urlBefore = page.url();
  await caret.click(); // caret target: collapse
  await expect(caret).toHaveAttribute("data-expanded", "false");
  await expect(features.getByTestId(WTID.sidebarGroupBody)).toBeHidden();
  expect(page.url()).toBe(urlBefore);

  await features.getByTestId(WTID.sidebarGroupName).click(); // name target: navigate
  await expect(page).toHaveURL(new RegExp(`/sessions/${e.sid}/product/features$`));
});

test("w1.5 sidebar expand/collapse state survives navigation (P5)", async ({ page }) => {
  env = await WorkspaceEnv.start("w1-persist-state");
  const e = env;

  await gotoWorkspace(page, e.baseURL, wsRoutes.product(e.sid));
  const stories = sidebarGroup(sidebarSection(page, "product"), "Stories");
  await stories.hover();
  await sidebarCaret(stories).click();
  await expect(sidebarCaret(stories)).toHaveAttribute("data-expanded", "false");

  await railItem(page, "architecture").click();
  await expect(sidebarSection(page, "architecture")).toBeVisible();
  await railItem(page, "product").click();
  await expect(sidebarSection(page, "product")).toBeVisible();

  const storiesAgain = sidebarGroup(sidebarSection(page, "product"), "Stories");
  await expect(sidebarCaret(storiesAgain)).toHaveAttribute("data-expanded", "false");
});

test("w1.6 the open page's sidebar row stays highlighted; it moves with navigation (P6)", async ({ page }) => {
  env = await WorkspaceEnv.start("w1-highlight");
  const e = env;

  await gotoWorkspace(page, e.baseURL, wsRoutes.story(e.sid, "60.2"));
  const product = sidebarSection(page, "product");
  await expect(sidebarStoryRow(product, "60.2")).toHaveAttribute("data-selected", "true");

  await gotoWorkspace(page, e.baseURL, wsRoutes.scenarios(e.sid));
  await expect(sidebarStoryRow(product, "60.2")).toHaveAttribute("data-selected", "false");
  await expect(sidebarGroup(product, "Scenarios").getByTestId(WTID.sidebarGroupName)).toHaveAttribute(
    "data-selected",
    "true",
  );
});
