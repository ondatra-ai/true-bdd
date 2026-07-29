/**
 * P4 (plan §4.3) — inventory matrix: PARAMETERIZED independent fixture
 * trees, one per degradation class of plan §3.4's identity model. Each
 * case is its own test() over its own fixture, server, and remote.
 *
 * REWRITTEN for the epic→story hierarchy rework: the NINE hierarchy cases
 * (epic-malformed … story-empty-internal-id) now resolve their epic/story
 * rows THROUGH an `epic-section` wrapper, with story rows SCOPED to their
 * section — collision-safe for the duplicate-epic-number case where the
 * position-derived create id `7.1` exists in BOTH sections. A new
 * noncanonical-filename browser case pins the header+flag/no-toggle/no-rows
 * contract previously covered only by Go honesty tests. The SIX
 * non-hierarchy cases (config-invalid, stories-not-a-dir, checklist-missing,
 * checklist-invalid, registry-present-empty, architecture-path-mismatch)
 * are UNCHANGED and stay green.
 *
 * The data-status / data-reason vocabularies asserted here are the
 * contract — see helpers/README-testids.md.
 */

import { expect, test, type Page } from "@playwright/test";

import { ProtocolEnv } from "./helpers/protocol-env";
import { TID, epicSection, gotoSession, inventoryDoc, storyRow, storyRowIn } from "./helpers/ui";

let env: ProtocolEnv | undefined;

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

const CHIP_TIMEOUT = { timeout: 15_000 };

interface MatrixCase {
  fixture: string;
  title: string;
  assert: (page: Page) => Promise<void>;
}

// ── SIX non-hierarchy cases — UNCHANGED (stay green) ──
const NON_HIERARCHY_CASES: MatrixCase[] = [
  {
    fixture: "p4-config-invalid",
    title: "invalid true-bdd.yaml renders the config chip as invalid",
    assert: async (page) => {
      await expect(inventoryDoc(page, "config")).toHaveAttribute("data-status", "invalid", CHIP_TIMEOUT);
    },
  },
  {
    fixture: "p4-stories-not-a-dir",
    title: "a file at the stories path renders not_a_dir",
    assert: async (page) => {
      await expect(inventoryDoc(page, "stories-dir")).toHaveAttribute("data-status", "not_a_dir", CHIP_TIMEOUT);
    },
  },
  {
    fixture: "p4-checklist-missing",
    title: "a removed checklist renders missing while siblings stay present",
    assert: async (page) => {
      await expect(inventoryDoc(page, "checklist-us-apply")).toHaveAttribute("data-status", "missing", CHIP_TIMEOUT);
      await expect(inventoryDoc(page, "checklist-us-create")).toHaveAttribute("data-status", "present");
    },
  },
  {
    fixture: "p4-checklist-invalid",
    title: "an unparseable checklist renders invalid while siblings stay present",
    assert: async (page) => {
      await expect(inventoryDoc(page, "checklist-us-refine")).toHaveAttribute("data-status", "invalid", CHIP_TIMEOUT);
      await expect(inventoryDoc(page, "checklist-us-create")).toHaveAttribute("data-status", "present");
    },
  },
  {
    fixture: "p4-registry-present-empty",
    title: "an empty-map registry renders present_empty",
    assert: async (page) => {
      await expect(inventoryDoc(page, "registry")).toHaveAttribute("data-status", "present_empty", CHIP_TIMEOUT);
    },
  },
  {
    fixture: "p4-architecture-path-mismatch",
    title: "a non-default architecture path warns while scanning the configured location",
    assert: async (page) => {
      await expect(page.getByTestId(TID.pathMismatchWarning)).toBeVisible(CHIP_TIMEOUT);
      await expect(inventoryDoc(page, "architecture")).toHaveAttribute("data-status", "present");
    },
  },
];

// ── NINE hierarchy cases — REWRITTEN to `epic-section` scoping (RED) ──
const HIERARCHY_CASES: MatrixCase[] = [
  {
    fixture: "p4-epic-malformed",
    title: "an unparseable epic renders an epic-section with an invalid row and NO toggle",
    assert: async (page) => {
      const section = epicSection(page, "epic-70-broken.yaml");
      await expect(section).toBeVisible(CHIP_TIMEOUT);
      await expect(section.getByTestId(TID.epicRow)).toHaveAttribute("data-status", "invalid");
      // An invalid epic has no story panel → no control that controls nothing.
      await expect(section.getByTestId(TID.epicToggle)).toHaveCount(0);
    },
  },
  {
    fixture: "p4-epic-duplicate-numbers",
    title: "duplicate epic filename numbers flag BOTH sections; scoped 7.1 rows stay resolvable",
    assert: async (page) => {
      const alpha = epicSection(page, "epic-07-alpha.yaml");
      const beta = epicSection(page, "epic-07-beta.yaml");
      await expect(alpha).toBeVisible(CHIP_TIMEOUT);
      await expect(beta).toBeVisible();
      await expect(alpha.getByTestId(TID.epicFlagDuplicateNumber)).toBeVisible();
      await expect(beta.getByTestId(TID.epicFlagDuplicateNumber)).toBeVisible();
      // The colliding create id 7.1 resolves uniquely WITHIN each section.
      await expect(storyRowIn(alpha, "7.1")).toBeVisible();
      await expect(storyRowIn(beta, "7.1")).toBeVisible();
    },
  },
  {
    fixture: "p4-epic-id-mismatch",
    title: "epic filename↔document id mismatch is flagged; the scoped row exposes the identity tuple",
    assert: async (page) => {
      const section = epicSection(page, "epic-42-identity-mismatch.yaml");
      await expect(section).toBeVisible(CHIP_TIMEOUT);
      await expect(section.getByTestId(TID.epicFlagIdMismatch)).toBeVisible();

      // Create id is POSITION-derived from the FILENAME number (42.1);
      // the tuple exposes the declared (77.5) and file-internal (88.9) ids.
      const story = storyRowIn(section, "42.1");
      await expect(story).toBeVisible();
      await expect(story).toHaveAttribute("data-declared-id", "77.5");
      await expect(story).toHaveAttribute("data-file-id", "88.9");
      await expect(story.getByTestId(TID.storyCreated)).toHaveAttribute("data-status", "one");
    },
  },
  {
    fixture: "p4-story-duplicate-ids",
    title: "duplicate declared story ids flag BOTH position-derived rows in the section",
    assert: async (page) => {
      const section = epicSection(page, "epic-70-duplicate-story-ids.yaml");
      await expect(section).toBeVisible(CHIP_TIMEOUT);
      await expect(storyRowIn(section, "70.1").getByTestId(TID.storyFlagDuplicateDeclaredId)).toBeVisible();
      await expect(storyRowIn(section, "70.2").getByTestId(TID.storyFlagDuplicateDeclaredId)).toBeVisible();
    },
  },
  {
    fixture: "p4-story-ambiguous-files",
    title: "two files matching the story glob render created ambiguous",
    assert: async (page) => {
      const section = epicSection(page, "epic-70-inventory.yaml");
      const row = storyRowIn(section, "70.1");
      await expect(row).toBeVisible(CHIP_TIMEOUT);
      await expect(row.getByTestId(TID.storyCreated)).toHaveAttribute("data-status", "ambiguous");
      await expect(row.getByTestId(TID.storyApplied)).toHaveAttribute("data-status", "unknown");
      await expect(row.getByTestId(TID.storyApplied)).toHaveAttribute("data-reason", "ambiguous");
    },
  },
  {
    fixture: "p4-story-malformed",
    title: "an unparseable story renders created invalid, applied unknown",
    assert: async (page) => {
      const section = epicSection(page, "epic-70-inventory.yaml");
      const row = storyRowIn(section, "70.1");
      await expect(row).toBeVisible(CHIP_TIMEOUT);
      await expect(row.getByTestId(TID.storyCreated)).toHaveAttribute("data-status", "invalid");
      await expect(row.getByTestId(TID.storyApplied)).toHaveAttribute("data-status", "unknown");
      await expect(row.getByTestId(TID.storyApplied)).toHaveAttribute("data-reason", "invalid");
    },
  },
  {
    fixture: "p4-story-deprecated-format",
    title: "a legacy test_scenarios story is apply-ineligible (deprecated_format)",
    assert: async (page) => {
      const section = epicSection(page, "epic-70-inventory.yaml");
      const row = storyRowIn(section, "70.1");
      await expect(row).toBeVisible(CHIP_TIMEOUT);
      await expect(row.getByTestId(TID.storyCreated)).toHaveAttribute("data-status", "one");
      await expect(row.getByTestId(TID.storyFlagDeprecatedFormat)).toBeVisible();
      await expect(row.getByTestId(TID.storyApplied)).toHaveAttribute("data-status", "unknown");
      await expect(row.getByTestId(TID.storyApplied)).toHaveAttribute("data-reason", "deprecated_format");
    },
  },
  {
    fixture: "p4-story-no-acs",
    title: "a zero-AC story is apply-ineligible (no_acceptance_criteria)",
    assert: async (page) => {
      const section = epicSection(page, "epic-70-inventory.yaml");
      const row = storyRowIn(section, "70.1");
      await expect(row).toBeVisible(CHIP_TIMEOUT);
      await expect(row.getByTestId(TID.storyCreated)).toHaveAttribute("data-status", "one");
      await expect(row.getByTestId(TID.storyFlagNoAcs)).toBeVisible();
      await expect(row.getByTestId(TID.storyApplied)).toHaveAttribute("data-status", "unknown");
      await expect(row.getByTestId(TID.storyApplied)).toHaveAttribute("data-reason", "no_acceptance_criteria");
    },
  },
  {
    fixture: "p4-story-empty-internal-id",
    title: "an empty internal story id is flagged (empty_internal_id)",
    assert: async (page) => {
      const section = epicSection(page, "epic-70-inventory.yaml");
      const row = storyRowIn(section, "70.1");
      await expect(row).toBeVisible(CHIP_TIMEOUT);
      await expect(row.getByTestId(TID.storyCreated)).toHaveAttribute("data-status", "one");
      await expect(row.getByTestId(TID.storyFlagEmptyInternalId)).toBeVisible();
      await expect(row.getByTestId(TID.storyApplied)).toHaveAttribute("data-status", "unknown");
      await expect(row.getByTestId(TID.storyApplied)).toHaveAttribute("data-reason", "empty_internal_id");
    },
  },
];

// ── NEW hierarchy case: noncanonical epic filename (header + flag, no
//    toggle, no rows) — previously untested in the browser (plan §4.3) ──
const NONCANONICAL_CASE: MatrixCase = {
  fixture: "p4-epic-noncanonical-filename",
  title: "a noncanonical epic filename renders a header + flag with NO toggle and NO rows",
  assert: async (page) => {
    const section = epicSection(page, "epic-7-noncanon.yaml");
    await expect(section).toBeVisible(CHIP_TIMEOUT);
    await expect(section.getByTestId(TID.epicFlagNoncanonicalFilename)).toBeVisible();
    // No Create-addressable stories ⇒ no toggle and no story rows.
    await expect(section.getByTestId(TID.epicToggle)).toHaveCount(0);
    await expect(storyRow(page, "7.1")).toHaveCount(0);
  },
};

const CASES: MatrixCase[] = [...NON_HIERARCHY_CASES, ...HIERARCHY_CASES, NONCANONICAL_CASE];

for (const matrixCase of CASES) {
  test(`P4 [${matrixCase.fixture}]: ${matrixCase.title}`, async ({ page }) => {
    env = await ProtocolEnv.start(matrixCase.fixture);
    const e = env;

    const fixture = await e.materialize(matrixCase.fixture);
    const remote = await e.startRemote(fixture.target);
    const session = await e.api.waitForSession((candidate) => candidate.pid === remote.pid);
    e.note({ sessionId: session.id });
    await e.api.waitForGeneration(session.id, 0);

    await gotoSession(page, e.server.baseURL, session.id);
    await matrixCase.assert(page);
  });
}
