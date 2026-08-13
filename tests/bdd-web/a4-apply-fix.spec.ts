/**
 * A4 (plan §4.3) — `us apply --fix`. One-AC story (file-prefix 77.5,
 * internal 88.9), a registry seeded with ONE unrelated scenario. The
 * Apply action on epic row 70.1 sends the file-prefix id "77.5"; the
 * single "present in registry?" check fails, the browser Applies to
 * convergence, and the registry gains lineage 88.9-001 pointing at the
 * exact story path.
 *
 * Oracle (plan §4.5): ONLY docs/scenarios.yaml changes; the unrelated
 * seeded entry survives byte-for-byte; the new entry carries the exact
 * lineage id, story path, and step text.
 */

import fs from "node:fs";
import path from "node:path";

import { pollUntil, type RunSummary } from "./helpers/api-client";
import { ClaudeCallBudget } from "./helpers/claude-budget";
import { skipUnlessClaudeAvailable } from "./helpers/claude-gate";
import { AI_TERMINAL_TIMEOUT_MS, applyUntilTerminal } from "./helpers/fix-loop";
import { ProtocolEnv } from "./helpers/protocol-env";
import { TID, gotoRun, gotoSession, storyRow } from "./helpers/ui";
import { expectOnlyPathsChanged } from "./helpers/tree-hash";

import { expect, test } from "@playwright/test";

const AI_CALL_BUDGET = 10;
const APPLY_CAP = 5; // us-apply config.max_apply_attempts

// The exact unrelated entry (verbatim from the seed) that must survive
// the merge byte-for-byte.
const UNRELATED_ENTRY = `  E2E-500:
    description: "Unrelated seeded scenario that must survive the apply merge byte-for-byte"
    service: "mcp-service"
    last_updated: "2026-07-01"
    user_stories:
      - story: "docs/prd/stories/12.4-unrelated.yaml"
        scenario_id: "12.4-001"
        merge_date: "2026-07-01"`;

let env: ProtocolEnv | undefined;

test.afterEach(async () => {
  const info = test.info();
  info.setTimeout(info.timeout + 60_000);
  const current = env;
  env = undefined;

  if (current !== undefined) {
    await current.teardown(test.info());
  }
});

test("A4: apply --fix merges lineage 88.9-001; unrelated entry survives byte-for-byte", async ({ page }) => {
  skipUnlessClaudeAvailable(test);

  env = await ProtocolEnv.start("a4-apply-fix");
  const e = env;

  const fixture = await e.materialize("a4-apply-fix");
  const remote = await e.startRemote(fixture.target);
  const session = await e.api.waitForSession((candidate) => candidate.pid === remote.pid);
  e.note({ sessionId: session.session_id });

  const budget = new ClaudeCallBudget(remote.pid).start();

  // Dispatch through the row action with --fix.
  await gotoSession(page, e.server.baseURL, session.session_id);
  const row = storyRow(page, "70.1");
  await expect(row.getByTestId(TID.actionApply)).toBeEnabled({ timeout: 15_000 });
  await row.getByTestId(TID.fixToggle).check();
  await row.getByTestId(TID.actionApply).click();

  const dispatched = await pollUntil<RunSummary>(
    async () => (await e.api.getSession(session.session_id)).runs.find((run) => run.command === "us-apply"),
    { timeoutMs: 30_000, what: "the Apply action to dispatch a us-apply run" },
  );
  expect(dispatched.story_id).toBe("77.5");
  expect(dispatched.fix).toBe(true);
  const runId = dispatched.run_id;
  e.note({ runId });

  await gotoRun(page, e.server.baseURL, session.session_id, runId);

  const { run: terminal, applies } = await applyUntilTerminal(page, e.api, session.session_id, runId, {
    cap: APPLY_CAP,
    label: "A4",
    stepTimeoutMs: AI_TERMINAL_TIMEOUT_MS,
  });
  expect(applies).toBeGreaterThanOrEqual(1);
  expect(terminal.outcome).toBe("converged");

  budget.stop();
  budget.assertWithinBudget(AI_CALL_BUDGET, "A4");

  // Oracle: only the registry changed.
  await expectOnlyPathsChanged(fixture.baseline, fixture.target, ["docs/scenarios.yaml"]);

  const registry = fs.readFileSync(path.join(fixture.target, "docs", "scenarios.yaml"), "utf8");

  // Unrelated entry survived byte-for-byte.
  expect(registry).toContain(UNRELATED_ENTRY);

  // The new entry carries the exact lineage id, story path, and a step
  // from the AC.
  expect(registry).toContain("88.9-001");
  expect(registry).toContain("docs/prd/stories/77.5-apply-summary.yaml");
  expect(registry).toContain("a summary of 150 words or fewer");
});
