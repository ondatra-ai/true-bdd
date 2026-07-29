/**
 * P3 (plan §4.3, REWRITTEN for the epic→story hierarchy rework) — the
 * engine base with a 3-story spread (missing / partial 1-of-2 applied /
 * fully 2-of-2 applied) now renders as an expandable `epic-section` rather
 * than a flat table. This spec asserts:
 *   - the epic accordion: `epic-section` for epic 60, its title read from
 *     the epic's `name:`, the parseable epic-row chip, and the `epic-toggle`
 *     that collapses (hides the story rows) and re-expands;
 *   - the three story rows SCOPED to the section, keeping the previous
 *     lifecycle assertions (created / applied x/y / refined "not recorded")
 *     plus the new `story-title` opener cells;
 *   - AI-compat: WITHOUT clicking any operation button, missing 60.1 still
 *     exposes a visible+enabled Create + fix-toggle (A1's contract) and
 *     resolved 60.2 exposes visible+enabled Refine / Apply / fix (A2/A4),
 *     with the exact identity data-* attributes; rows stay uniquely
 *     resolvable and visible after Refresh;
 *   - the Refresh flow is unchanged: the WORKER mutates the registry, clicks
 *     Refresh, and the scoped 60.2 row flips to 2/2.
 *
 * RED until the hierarchy UI exists: `epic-section` / `epic-title` /
 * `epic-toggle` / `story-title` are new selectors.
 */

import fs from "node:fs";
import path from "node:path";

import { expect, test } from "@playwright/test";

import { ProtocolEnv } from "./helpers/protocol-env";
import { TID, epicSection, epicTitle, epicToggle, gotoSession, inventoryDoc, storyRowIn, storyTitle } from "./helpers/ui";
import { readFixtureFile, scalarField, storyBlockById } from "./helpers/story-fixture";

let env: ProtocolEnv | undefined;

const EPIC_FILE = "epic-60-inventory-spread.yaml";
const EPIC_REL = path.join("docs", "prd", "epics", EPIC_FILE);

test.afterEach(async () => {
  // Extend (never overwrite) the shared budget so scoped teardown has
  // room even after a test-body timeout (Playwright's documented idiom).
  const info = test.info();
  info.setTimeout(info.timeout + 60_000);
  const current = env;
  env = undefined;

  if (current !== undefined) {
    await current.teardown(test.info());
  }
});

/**
 * Appended verbatim to docs/scenarios.yaml: a registry entry covering
 * lineage 60.2-002 with the EXACT story path — flips 60.2 to 2/2.
 */
const REGISTRY_APPEND_602_002 = `  E2E-604:
    description: "Not-shared documents surface the sharing message"
    service: "mcp-service"
    last_updated: "2026-07-29"
    user_stories:
      - story: "docs/prd/stories/60.2-summary-shared-docs.yaml"
        scenario_id: "60.2-002"
        merge_date: "2026-07-29"
    merged_steps:
      given:
        - "a Google Doc exists that is not shared with the Claude User's account"
      when:
        - "the Claude User asks Claude to summarize that document"
      then:
        - "Claude displays the message 'This document is not shared with your account'"
`;

test("P3: epic→story hierarchy renders (title, toggle, scoped rows, AI-compat controls)", async ({ page }) => {
  env = await ProtocolEnv.start("p3-hierarchy");
  const e = env;

  const fixture = await e.materialize("p3-inventory-spread");
  const remote = await e.startRemote(fixture.target);
  const session = await e.api.waitForSession((candidate) => candidate.pid === remote.pid);
  e.note({ sessionId: session.session_id });

  await gotoSession(page, e.server.baseURL, session.session_id);

  // Document chips still render (unchanged contract).
  await expect(inventoryDoc(page, "config")).toHaveAttribute("data-status", "present", { timeout: 15_000 });
  await expect(inventoryDoc(page, "registry")).toHaveAttribute("data-status", "present");

  // ── Epic accordion ──
  const section = epicSection(page, EPIC_FILE);
  await expect(section).toBeVisible();
  await expect(section).toHaveAttribute("data-epic-number", "60");

  // Title from the epic document's `name:` (read from the materialized file).
  const epicRaw = readFixtureFile(fixture.target, EPIC_REL);
  await expect(epicTitle(section)).toHaveText(scalarField(epicRaw, "name"));

  // The epic-row chip inside the section is parseable.
  await expect(section.getByTestId(TID.epicRow)).toHaveAttribute("data-status", "parseable");

  // ── Scoped rows: default expanded, previous lifecycle assertions ──
  const row1 = storyRowIn(section, "60.1");
  const row2 = storyRowIn(section, "60.2");
  const row3 = storyRowIn(section, "60.3");

  await expect(row1).toBeVisible();
  await expect(row1.getByTestId(TID.storyCreated)).toHaveAttribute("data-status", "missing");
  await expect(row1.getByTestId(TID.storyApplied)).toHaveAttribute("data-status", "unknown");
  await expect(row1.getByTestId(TID.storyApplied)).toHaveAttribute("data-reason", "missing");
  await expect(row1.getByTestId(TID.storyRefined)).toHaveText("not recorded");

  await expect(row2.getByTestId(TID.storyCreated)).toHaveAttribute("data-status", "one");
  await expect(row2.getByTestId(TID.storyApplied)).toHaveText("1/2");
  await expect(row2.getByTestId(TID.storyRefined)).toHaveText("not recorded");

  await expect(row3.getByTestId(TID.storyCreated)).toHaveAttribute("data-status", "one");
  await expect(row3.getByTestId(TID.storyApplied)).toHaveText("2/2");

  // Titles: the new story-title opener carries the declared title.
  const title602 = scalarField(storyBlockById(epicRaw, "60.2"), "title");
  await expect(storyTitle(section, "60.2")).toContainText(title602);

  // Identity data-* attributes exact (position-derived + declared/file ids).
  await expect(row1).toHaveAttribute("data-declared-id", "60.1");
  await expect(row2).toHaveAttribute("data-declared-id", "60.2");
  await expect(row2).toHaveAttribute("data-file-id", "60.2");
  await expect(row2).toHaveAttribute("data-position", "2");
  await expect(row2).toHaveAttribute("data-epic-number", "60");

  // ── AI-compat: controls reachable WITHOUT clicking any operation ──
  // Missing 60.1 exposes an enabled Create + fix-toggle (A1's contract).
  await expect(row1.getByTestId(TID.actionCreate)).toBeVisible();
  await expect(row1.getByTestId(TID.actionCreate)).toBeEnabled();
  await expect(row1.getByTestId(TID.fixToggle)).toBeVisible();
  // Resolved 60.2 exposes enabled Refine + Apply + fix (A2 / A4).
  await expect(row2.getByTestId(TID.actionRefine)).toBeEnabled();
  await expect(row2.getByTestId(TID.actionApply)).toBeEnabled();
  await expect(row2.getByTestId(TID.fixToggle)).toBeVisible();

  // ── Toggle collapses (rows hidden) and re-expands ──
  const toggle = epicToggle(section);
  await expect(toggle).toHaveAttribute("aria-expanded", "true");
  await toggle.click();
  await expect(toggle).toHaveAttribute("aria-expanded", "false");
  await expect(row2).toBeHidden();
  await toggle.click();
  await expect(toggle).toHaveAttribute("aria-expanded", "true");
  await expect(row2).toBeVisible();
});

test("P3: Refresh picks up a worker registry mutation (scoped row)", async ({ page }) => {
  env = await ProtocolEnv.start("p3-refresh");
  const e = env;

  const fixture = await e.materialize("p3-inventory-spread");
  const remote = await e.startRemote(fixture.target);
  const session = await e.api.waitForSession((candidate) => candidate.pid === remote.pid);
  e.note({ sessionId: session.session_id });

  await gotoSession(page, e.server.baseURL, session.session_id);

  const section = epicSection(page, EPIC_FILE);
  const row2 = storyRowIn(section, "60.2");
  await expect(row2.getByTestId(TID.storyApplied)).toHaveText("1/2", { timeout: 15_000 });

  // WORKER-side mutation (never server-side): cover 60.2's second AC in the
  // registry, then drive Refresh through the UI — now an immediate live
  // session_detail READ (plan §1.5), not a /refresh mutation.
  fs.appendFileSync(path.join(fixture.target, "docs", "scenarios.yaml"), REGISTRY_APPEND_602_002);

  await page.getByTestId(TID.refresh).click();

  // The changed state renders in the still-uniquely-resolvable scoped row.
  await expect(row2.getByTestId(TID.storyApplied)).toHaveText("2/2", { timeout: 30_000 });
});
