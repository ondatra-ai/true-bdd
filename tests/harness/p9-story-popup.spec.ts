/**
 * P9 (plan §4.3, NEW) — the story review popup. Every declared story row's
 * `story-title` is a button that opens a native `<dialog>` (showModal())
 * review modal: a structured Review tab (statement + ACs + Gherkin steps +
 * lifecycle chips) and a Raw tab (verbatim file). The suite pins the a11y
 * contract (accessible name, native focus containment, Escape / close /
 * backdrop close + focus restore, panel-click-does-not-close), the
 * every-story-openable fallback chain (file content → epic-declared content
 * → identity-only notice), the invalid-story Raw+error contract with a
 * declared-content Review fallback, and the mandatory degraded-snapshot
 * state (tiny per-test MAX_INVENTORY_REQUEST_BYTES server).
 *
 * Expected Review values are READ FROM THE MATERIALIZED FIXTURE FILES (no
 * duplicated literals); the Raw panel is a byte-exact textContent oracle.
 *
 * RED until the modal exists: `story-title` / `story-modal-*` are new.
 */

import fs from "node:fs";
import path from "node:path";

import { expect, test, type Locator, type Page } from "@playwright/test";

import { ProtocolEnv } from "./helpers/protocol-env";
import type { MaterializedFixture } from "./helpers/materializer";
import {
  TID,
  epicSection,
  gotoSession,
  storyModal,
  storyModalAc,
  storyModalStep,
  storyTitle,
} from "./helpers/ui";
import {
  acDescriptions,
  firstStep,
  readFixtureFile,
  scalarField,
  statementOf,
  storyBlockById,
} from "./helpers/story-fixture";

let env: ProtocolEnv | undefined;

const STORY_602_REL = path.join("docs", "prd", "stories", "60.2-summary-shared-docs.yaml");
const EPIC_60_REL = path.join("docs", "prd", "epics", "epic-60-inventory-spread.yaml");
const STORY_BROKEN_REL = path.join("docs", "prd", "stories", "70.1-broken.yaml");
const EPIC_70_REL = path.join("docs", "prd", "epics", "epic-70-inventory.yaml");

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
 * Materializes a fixture, starts a remote, waits for its first promoted
 * inventory, and navigates to the session page. Returns the materialized
 * fixture so the test can read its files as the assertion oracle.
 */
async function openSession(
  page: Page,
  slug: string,
  fixtureName: string,
): Promise<{ fixture: MaterializedFixture; sessionId: string; env: ProtocolEnv }> {
  env = await ProtocolEnv.start(slug);
  const e = env;
  const fixture = await e.materialize(fixtureName);
  const remote = await e.startRemote(fixture.target);
  const session = await e.api.waitForSession((candidate) => candidate.pid === remote.pid);
  e.note({ sessionId: session.session_id });
  await gotoSession(page, e.server.baseURL, session.session_id);

  return { fixture, sessionId: session.session_id, env: e };
}

/** Opens the modal for a scoped row after asserting the opener exists. */
async function openModal(opener: Locator): Promise<void> {
  await expect(opener).toBeVisible({ timeout: 15_000 });
  await opener.click();
  await expect(storyModal(opener.page())).toBeVisible();
}

test("P9: story-title opens an accessible modal with native focus containment", async ({ page }) => {
  const { fixture } = await openSession(page, "p9-a11y", "p3-inventory-spread");

  const section = epicSection(page, "epic-60-inventory-spread.yaml");
  const opener = storyTitle(section, "60.2");
  await openModal(opener);

  const modal = storyModal(page);
  await expect(modal).toHaveAttribute("data-story-id", "60.2");

  // Accessible name = story id + title (from the fixture).
  const title = scalarField(readFixtureFile(fixture.target, STORY_602_REL), "title");
  await expect(page.getByTestId(TID.storyModalTitle)).toContainText("60.2");
  await expect(page.getByTestId(TID.storyModalTitle)).toContainText(title);

  // Focus moved INSIDE the dialog on open.
  const focusInsideOnOpen = await page.evaluate(() => {
    const dialog = document.querySelector('[data-testid="story-modal"]');
    return !!(dialog && document.activeElement && dialog.contains(document.activeElement));
  });
  expect(focusInsideOnOpen).toBe(true);

  // Tab / Shift+Tab cannot escape to background controls (native modality).
  for (let index = 0; index < 8; index += 1) {
    await page.keyboard.press("Tab");
  }
  const stillContained = await page.evaluate(() => {
    const dialog = document.querySelector('[data-testid="story-modal"]');
    return !!(dialog && document.activeElement && dialog.contains(document.activeElement));
  });
  expect(stillContained).toBe(true);
  await expect(page.getByTestId(TID.refresh)).not.toBeFocused();
});

test("P9: the Review panel renders the statement, ACs, and steps from the fixture", async ({ page }) => {
  const { fixture } = await openSession(page, "p9-review", "p3-inventory-spread");

  const section = epicSection(page, "epic-60-inventory-spread.yaml");
  await openModal(storyTitle(section, "60.2"));

  const raw = readFixtureFile(fixture.target, STORY_602_REL);
  const statement = statementOf(raw);
  const descriptions = acDescriptions(raw);

  // Statement (as-a / i-want / so-that).
  const statementBlock = page.getByTestId(TID.storyModalStatement);
  await expect(statementBlock).toContainText(statement.asA);
  await expect(statementBlock).toContainText(statement.iWant);
  await expect(statementBlock).toContainText(statement.soThat);

  // Acceptance criteria keyed by data-ac-id, with their descriptions.
  await expect(storyModalAc(page, "AC-1")).toContainText(descriptions[0]);
  await expect(storyModalAc(page, "AC-2")).toContainText(descriptions[1]);

  // Gherkin steps: every kind incl. the `and` AND `but` modifiers, exact text.
  for (const kind of ["given", "when", "then", "and", "but"] as const) {
    const step = firstStep(raw, kind);
    expect(step, `fixture must contain a ${kind} step`).toBeDefined();
    if (step !== undefined) {
      await expect(storyModalStep(page, kind).first()).toHaveText(step.text);
    }
  }

  // Lifecycle chips mirror the row (60.2 is created=one, applied 1/2).
  await expect(page.getByTestId(TID.storyModalApplied)).toContainText("1/2");
  await expect(page.getByTestId(TID.storyModalRefined)).toContainText("not recorded");
  await expect(page.getByTestId(TID.storyModalStatus)).toBeVisible();
});

test("P9: the tablist switches and the Raw panel matches the file bytes exactly", async ({ page }) => {
  const { fixture } = await openSession(page, "p9-raw", "p3-inventory-spread");

  const section = epicSection(page, "epic-60-inventory-spread.yaml");
  await openModal(storyTitle(section, "60.2"));

  // Review is the default selected tab.
  const reviewTab = page.getByTestId(TID.storyModalTabReview);
  const rawTab = page.getByTestId(TID.storyModalTabRaw);
  await expect(reviewTab).toHaveAttribute("aria-selected", "true");
  await expect(rawTab).toHaveAttribute("aria-selected", "false");

  // Keyboard navigation moves selection (real tablist).
  await reviewTab.focus();
  await page.keyboard.press("ArrowRight");
  await expect(rawTab).toHaveAttribute("aria-selected", "true");
  await expect(reviewTab).toHaveAttribute("aria-selected", "false");

  // The Raw panel is the verbatim file — byte-exact textContent (NOT the
  // whitespace-normalizing toHaveText).
  const fileBytes = readFixtureFile(fixture.target, STORY_602_REL);
  const rendered = await page.getByTestId(TID.storyModalRaw).textContent();
  expect(rendered).toBe(fileBytes);
});

test("P9: Escape / close / backdrop close and restore focus; panel click does not close", async ({ page }) => {
  await openSession(page, "p9-close", "p3-inventory-spread");

  const section = epicSection(page, "epic-60-inventory-spread.yaml");
  const opener = storyTitle(section, "60.2");
  const modal = storyModal(page);

  // A click on the inner panel must NOT close the modal.
  await openModal(opener);
  await page.getByTestId(TID.storyModalPanel).click();
  await expect(modal).toBeVisible();

  // Escape closes and restores focus to the opener.
  await page.keyboard.press("Escape");
  await expect(modal).toBeHidden();
  await expect(opener).toBeFocused();

  // The close button closes and restores focus.
  await openModal(opener);
  await page.getByTestId(TID.storyModalClose).click();
  await expect(modal).toBeHidden();
  await expect(opener).toBeFocused();

  // A backdrop click (outside the panel) closes and restores focus.
  await openModal(opener);
  await modal.click({ position: { x: 3, y: 3 } });
  await expect(modal).toBeHidden();
  await expect(opener).toBeFocused();
});

test("P9: a missing story opens a declared-content modal with a not-created notice", async ({ page }) => {
  const { fixture } = await openSession(page, "p9-missing", "p3-inventory-spread");

  // 60.1 has NO story file — its title is still a button (every story openable).
  const section = epicSection(page, "epic-60-inventory-spread.yaml");
  await openModal(storyTitle(section, "60.1"));

  // The modal reviews the EPIC-DECLARED content (from the epic fixture).
  const block = storyBlockById(readFixtureFile(fixture.target, EPIC_60_REL), "60.1");
  const statement = statementOf(block);
  const descriptions = acDescriptions(block);
  await expect(page.getByTestId(TID.storyModalStatement)).toContainText(statement.iWant);
  await expect(storyModalAc(page, "AC-1")).toContainText(descriptions[0]);

  // An availability notice, and NO Raw tab (there is no file).
  await expect(page.getByTestId(TID.storyModalNotice)).toHaveAttribute("data-reason", "not_created");
  await expect(page.getByTestId(TID.storyModalTabRaw)).toHaveCount(0);
});

test("P9: an ambiguous story opens a declared-content modal with a match-count notice", async ({ page }) => {
  const { fixture } = await openSession(page, "p9-ambiguous", "p4-story-ambiguous-files");

  const section = epicSection(page, "epic-70-inventory.yaml");
  await openModal(storyTitle(section, "70.1"));

  // Declared content is shown; the notice reports the ambiguity (2 matches).
  const block = storyBlockById(readFixtureFile(fixture.target, EPIC_70_REL), "70.1");
  await expect(page.getByTestId(TID.storyModalStatement)).toContainText(statementOf(block).iWant);
  const notice = page.getByTestId(TID.storyModalNotice);
  await expect(notice).toHaveAttribute("data-reason", "ambiguous");
  await expect(notice).toContainText("2");
});

test("P9: a malformed story opens on Raw with an error banner and a declared-content Review fallback", async ({ page }) => {
  const { fixture } = await openSession(page, "p9-malformed", "p4-story-malformed");

  const section = epicSection(page, "epic-70-inventory.yaml");
  await openModal(storyTitle(section, "70.1"));

  // Invalid story opens on Raw with an error banner.
  await expect(page.getByTestId(TID.storyModalTabRaw)).toHaveAttribute("aria-selected", "true");
  await expect(page.getByTestId(TID.storyModalError)).toBeVisible();

  // Raw is byte-exact even for the unparseable file.
  const brokenBytes = readFixtureFile(fixture.target, STORY_BROKEN_REL);
  expect(await page.getByTestId(TID.storyModalRaw).textContent()).toBe(brokenBytes);

  // Review is ENABLED because the epic declares the story — it falls back to
  // the declared content beneath the error.
  const reviewTab = page.getByTestId(TID.storyModalTabReview);
  await expect(reviewTab).not.toHaveAttribute("aria-disabled", "true");
  await reviewTab.click();
  const block = storyBlockById(readFixtureFile(fixture.target, EPIC_70_REL), "70.1");
  await expect(page.getByTestId(TID.storyModalStatement)).toContainText(statementOf(block).iWant);
});

test("P9: arrow-key tab switching moves focus to the destination tab (roving focus)", async ({ page }) => {
  await openSession(page, "p9-roving", "p3-inventory-spread");

  const section = epicSection(page, "epic-60-inventory-spread.yaml");
  await openModal(storyTitle(section, "60.2"));

  const reviewTab = page.getByTestId(TID.storyModalTabReview);
  const rawTab = page.getByTestId(TID.storyModalTabRaw);

  await reviewTab.focus();
  await expect(reviewTab).toBeFocused();

  // Roving focus: ArrowRight moves FOCUS to the destination tab, not just its
  // aria-selected state (finding 5b).
  await page.keyboard.press("ArrowRight");
  await expect(rawTab).toBeFocused();
  await expect(rawTab).toHaveAttribute("aria-selected", "true");

  // ArrowLeft rolls focus back to Review.
  await page.keyboard.press("ArrowLeft");
  await expect(reviewTab).toBeFocused();
  await expect(reviewTab).toHaveAttribute("aria-selected", "true");
});

test("P9: a story removed on disk keeps the modal OPEN as changed-on-disk", async ({ page }) => {
  const { fixture } = await openSession(page, "p9-changed-on-disk", "p3-inventory-spread");

  // Open the modal for 60.3 — the last declared row (position 3, has a file).
  const section = epicSection(page, "epic-60-inventory-spread.yaml");
  const opener = storyTitle(section, "60.3");
  await openModal(opener);
  const modal = storyModal(page);

  // Remove the 60.3 declaration from the epic on disk so its (epic, position)
  // composite identity vanishes from the NEXT scan.
  const epicPath = path.join(fixture.target, EPIC_60_REL);
  const original = fs.readFileSync(epicPath, "utf8");
  const withoutLastStory = original.split('  - id: "60.3"')[0];
  expect(withoutLastStory.length).toBeLessThan(original.length);
  fs.writeFileSync(epicPath, withoutLastStory);

  // v2 has NO /refresh route: every browser poll of session_detail is a fresh
  // CLI scan (plan §1.5), so the live poll picks the on-disk change up on its
  // own — no manual re-scan trigger needed.

  // The dialog STAYS OPEN (never silently unmounts) and reports the story
  // changed on disk (finding 4).
  const notice = page.locator('[data-testid="story-modal-notice"][data-reason="changed_on_disk"]');
  await expect(notice).toBeVisible({ timeout: 30_000 });
  await expect(modal).toBeVisible();
});

test("P9: a degraded snapshot shows the truncation banner and an omission-state modal", async ({ page }) => {
  // A per-test server with a tiny inventory request budget forces the
  // remote's fit-to-budget degrade path (plan §1a/§r3-3). Per-test server
  // isolation makes this cheap. 5120 sits above the always-fit floor plus
  // one identity row (~700B) but far below full content+raw, so the 60.2
  // row survives in an omitted state instead of collapsing to the floor.
  env = await ProtocolEnv.start("p9-degraded", { serverEnv: { MAX_INVENTORY_REQUEST_BYTES: "5120" } });
  const e = env;
  const fixture = await e.materialize("p3-inventory-spread");
  const remote = await e.startRemote(fixture.target);
  const session = await e.api.waitForSession((candidate) => candidate.pid === remote.pid);
  e.note({ sessionId: session.session_id });

  // Do NOT wait for a generation — a degraded/again-413 flow may never
  // promote until implemented; assert the banner at the DOM level so the
  // failure is assertion-level (banner absent), not a setup timeout.
  await gotoSession(page, e.server.baseURL, session.session_id);

  await expect(page.getByTestId(TID.inventoryTruncatedBanner)).toBeVisible({ timeout: 20_000 });

  // A row in an omitted state still renders an opener whose modal shows the
  // omission state (raw/content omitted) rather than pretending to have it.
  const section = epicSection(page, "epic-60-inventory-spread.yaml");
  await openModal(storyTitle(section, "60.2"));
  await expect(page.getByTestId(TID.storyModalNotice)).toBeVisible();
});

test("P9: a server cap below the minimum viable request shows the limit-too-small banner (out of band)", async ({ page }) => {
  // A cap below even the floor request (envelope + floor snapshot) means NO
  // inventory upload can ever be accepted. The remote reports the terminal
  // cannot-fit state OUT OF BAND on its poll (plan §1a / finding 1), so the
  // session page still renders it honestly despite having no snapshot.
  env = await ProtocolEnv.start("p9-limit-too-small", { serverEnv: { MAX_INVENTORY_REQUEST_BYTES: "200" } });
  const e = env;
  const fixture = await e.materialize("p3-inventory-spread");
  const remote = await e.startRemote(fixture.target);
  const session = await e.api.waitForSession((candidate) => candidate.pid === remote.pid);
  e.note({ sessionId: session.session_id });

  await gotoSession(page, e.server.baseURL, session.session_id);

  const banner = page.getByTestId(TID.inventoryTruncatedBanner);
  await expect(banner).toBeVisible({ timeout: 20_000 });
  await expect(banner).toContainText("limit too small");
});
