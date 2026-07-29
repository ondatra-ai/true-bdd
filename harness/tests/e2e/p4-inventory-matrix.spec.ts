/**
 * P4 (plan §4.3) — inventory matrix: PARAMETERIZED independent
 * fixture trees, one per degradation class of plan §3.4's identity
 * model. Each case is its own test() over its own fixture, server,
 * and remote, so every class fails (and later passes) independently.
 *
 * The data-status / data-reason vocabularies asserted here are the
 * contract — see helpers/README-testids.md.
 */

import { expect, test, type Page } from "@playwright/test";

import { ProtocolEnv } from "./helpers/protocol-env";
import { TID, epicRow, gotoSession, inventoryDoc, storyRow } from "./helpers/ui";

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

const CASES: MatrixCase[] = [
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
    fixture: "p4-epic-malformed",
    title: "an unparseable epic renders its epic row as invalid",
    assert: async (page) => {
      const row = epicRow(page, "epic-70-broken.yaml");
      await expect(row).toBeVisible(CHIP_TIMEOUT);
      await expect(row).toHaveAttribute("data-status", "invalid");
    },
  },
  {
    fixture: "p4-epic-duplicate-numbers",
    title: "duplicate epic filename numbers flag BOTH epic rows",
    assert: async (page) => {
      const alpha = epicRow(page, "epic-07-alpha.yaml");
      const beta = epicRow(page, "epic-07-beta.yaml");
      await expect(alpha).toBeVisible(CHIP_TIMEOUT);
      await expect(alpha.getByTestId(TID.epicFlagDuplicateNumber)).toBeVisible();
      await expect(beta.getByTestId(TID.epicFlagDuplicateNumber)).toBeVisible();
    },
  },
  {
    fixture: "p4-epic-id-mismatch",
    title: "epic filename↔document id mismatch is flagged; the identity tuple is exposed",
    assert: async (page) => {
      const epic = epicRow(page, "epic-42-identity-mismatch.yaml");
      await expect(epic).toBeVisible(CHIP_TIMEOUT);
      await expect(epic.getByTestId(TID.epicFlagIdMismatch)).toBeVisible();

      // Create id is POSITION-derived from the FILENAME number (42.1);
      // the tuple exposes the declared (77.5) and file-internal (88.9) ids.
      const story = storyRow(page, "42.1");
      await expect(story).toBeVisible();
      await expect(story).toHaveAttribute("data-declared-id", "77.5");
      await expect(story).toHaveAttribute("data-file-id", "88.9");
      await expect(story.getByTestId(TID.storyCreated)).toHaveAttribute("data-status", "one");
    },
  },
  {
    fixture: "p4-story-duplicate-ids",
    title: "duplicate declared story ids flag BOTH position-derived rows",
    assert: async (page) => {
      const first = storyRow(page, "70.1");
      const second = storyRow(page, "70.2");
      await expect(first).toBeVisible(CHIP_TIMEOUT);
      await expect(first.getByTestId(TID.storyFlagDuplicateDeclaredId)).toBeVisible();
      await expect(second.getByTestId(TID.storyFlagDuplicateDeclaredId)).toBeVisible();
    },
  },
  {
    fixture: "p4-story-ambiguous-files",
    title: "two files matching the story glob render created ambiguous",
    assert: async (page) => {
      const row = storyRow(page, "70.1");
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
      const row = storyRow(page, "70.1");
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
      const row = storyRow(page, "70.1");
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
      const row = storyRow(page, "70.1");
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
      const row = storyRow(page, "70.1");
      await expect(row).toBeVisible(CHIP_TIMEOUT);
      await expect(row.getByTestId(TID.storyCreated)).toHaveAttribute("data-status", "one");
      await expect(row.getByTestId(TID.storyFlagEmptyInternalId)).toBeVisible();
      await expect(row.getByTestId(TID.storyApplied)).toHaveAttribute("data-status", "unknown");
      await expect(row.getByTestId(TID.storyApplied)).toHaveAttribute("data-reason", "empty_internal_id");
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
