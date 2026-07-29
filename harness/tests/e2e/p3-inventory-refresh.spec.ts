/**
 * P3 (plan §4.3) — inventory + Refresh: engine base with a 3-story
 * spread (missing / partial 1-of-2 applied / fully 2-of-2 applied).
 * Asserts the exact rendering including partial x/y and refined
 * "not recorded"; then the WORKER mutates the fixture tree (appends a
 * registry entry covering 60.2's second AC), clicks Refresh, and
 * asserts a strictly higher generation plus the changed state.
 */

import fs from "node:fs";
import path from "node:path";

import { expect, test } from "@playwright/test";

import { ProtocolEnv } from "./helpers/protocol-env";
import { TID, gotoSession, inventoryDoc, storyRow } from "./helpers/ui";

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

test("P3: 3-story spread renders exactly; Refresh picks up a worker mutation", async ({ page }) => {
  env = await ProtocolEnv.start("p3-inventory");
  const e = env;

  const fixture = await e.materialize("p3-inventory-spread");
  const remote = await e.startRemote(fixture.target);
  const session = await e.api.waitForSession((candidate) => candidate.pid === remote.pid);
  e.note({ sessionId: session.id });
  await e.api.waitForGeneration(session.id, 0);

  await gotoSession(page, e.server.baseURL, session.id);

  // Document chips: everything the spread ships is present.
  await expect(inventoryDoc(page, "config")).toHaveAttribute("data-status", "present", {
    timeout: 15_000,
  });
  await expect(inventoryDoc(page, "prd")).toHaveAttribute("data-status", "present");
  await expect(inventoryDoc(page, "architecture")).toHaveAttribute("data-status", "present");
  await expect(inventoryDoc(page, "registry")).toHaveAttribute("data-status", "present");
  await expect(inventoryDoc(page, "checklist-us-apply")).toHaveAttribute("data-status", "present");

  // 60.1 — declared in the epic, no story file.
  const row1 = storyRow(page, "60.1");
  await expect(row1.getByTestId(TID.storyCreated)).toHaveAttribute("data-status", "missing");
  await expect(row1.getByTestId(TID.storyApplied)).toHaveAttribute("data-status", "unknown");
  await expect(row1.getByTestId(TID.storyApplied)).toHaveAttribute("data-reason", "missing");
  await expect(row1.getByTestId(TID.storyRefined)).toHaveText("not recorded");

  // 60.2 — story file present, 1 of 2 lineage ids covered (partial x/y).
  const row2 = storyRow(page, "60.2");
  await expect(row2.getByTestId(TID.storyCreated)).toHaveAttribute("data-status", "one");
  await expect(row2.getByTestId(TID.storyApplied)).toHaveText("1/2");
  await expect(row2.getByTestId(TID.storyRefined)).toHaveText("not recorded");

  // 60.3 — fully applied.
  const row3 = storyRow(page, "60.3");
  await expect(row3.getByTestId(TID.storyCreated)).toHaveAttribute("data-status", "one");
  await expect(row3.getByTestId(TID.storyApplied)).toHaveText("2/2");
  await expect(row3.getByTestId(TID.storyRefined)).toHaveText("not recorded");

  // WORKER-side mutation (never server-side): cover 60.2's second AC
  // in the registry, then drive Refresh through the UI.
  const generationBefore = (await e.api.getSession(session.id)).inventory_generation;
  fs.appendFileSync(path.join(fixture.target, "docs", "scenarios.yaml"), REGISTRY_APPEND_602_002);

  await page.getByTestId(TID.refresh).click();

  // A strictly higher generation is promoted...
  await e.api.waitForGeneration(session.id, generationBefore);

  // ...and the changed state is rendered.
  await expect(row2.getByTestId(TID.storyApplied)).toHaveText("2/2", { timeout: 30_000 });
});
