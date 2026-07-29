/**
 * A5 (plan §4.3) — `build tests --fix`. A one-scenario registry
 * (INT-901) with no test referencing it: the single build-tests check
 * fails, the browser Applies through the fix loop, and a new test lands
 * under the only allowed root (tests/) referencing the exact scenario
 * id.
 *
 * Oracle (plan §4.5): a new test under tests/** only; registry and
 * config are byte-unchanged. Build commands carry no story identifier,
 * so A5 dispatches through the API and drives the browser fix loop — the
 * "browser Applies" contract that matters here.
 */

import fs from "node:fs";
import path from "node:path";

import { expect, test } from "@playwright/test";

import { newRunToken } from "./helpers/api-client";
import { ClaudeCallBudget } from "./helpers/claude-budget";
import { skipUnlessClaudeAvailable } from "./helpers/claude-gate";
import { AI_TERMINAL_TIMEOUT_MS, applyUntilTerminal } from "./helpers/fix-loop";
import { ProtocolEnv } from "./helpers/protocol-env";
import { gotoRun } from "./helpers/ui";
import { expectOnlyPathsChanged } from "./helpers/tree-hash";

const AI_CALL_BUDGET = 10;
const APPLY_CAP = 5; // build-tests has no config block → engine default

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

test("A5: build tests --fix authors a test under tests/ referencing INT-901", async ({ page }) => {
  skipUnlessClaudeAvailable(test);

  env = await ProtocolEnv.start("a5-build-tests-fix");
  const e = env;

  const fixture = await e.materialize("a5-build-tests-fix");
  const remote = await e.startRemote(fixture.target);
  const session = await e.api.waitForSession((candidate) => candidate.pid === remote.pid);
  e.note({ sessionId: session.id });
  await e.api.waitForGeneration(session.id, 0);

  const budget = new ClaudeCallBudget(remote.pid).start();

  const { runId } = await e.api.dispatchRun(session.id, {
    command: "build-tests",
    fix: true,
    client_token: newRunToken(),
  });
  e.note({ runId });

  await gotoRun(page, e.server.baseURL, runId);

  const { run: terminal, applies } = await applyUntilTerminal(page, e.api, runId, {
    cap: APPLY_CAP,
    label: "A5",
    stepTimeoutMs: AI_TERMINAL_TIMEOUT_MS,
  });
  expect(applies).toBeGreaterThanOrEqual(1);
  expect(terminal.outcome).toBe("converged");

  budget.stop();
  budget.assertWithinBudget(AI_CALL_BUDGET, "A5");

  // Oracle: only new files under tests/**; registry/config untouched.
  const diff = await expectOnlyPathsChanged(fixture.baseline, fixture.target, ["tests/**"]);
  expect(diff.created.length).toBeGreaterThanOrEqual(1);
  expect(diff.modified).toEqual([]);

  // The created test references the exact scenario id and looks executable.
  const created = diff.created.filter((rel) => rel.startsWith("tests/"));
  expect(created.length).toBeGreaterThanOrEqual(1);

  const body = fs.readFileSync(path.join(fixture.target, created[0]), "utf8");
  expect(body).toContain("INT-901");
  expect(body).toMatch(/\b(test\(|describe\(|it\(|func Test)/);
});
