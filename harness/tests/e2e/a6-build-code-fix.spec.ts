/**
 * A6 (plan §4.3) — `build code --fix` over a Go module. architecture.yaml
 * declares service `calculator` (unit layer, framework go-test, path
 * services/calculator). build-code runs `go test -C services/calculator
 * ./...`, discovers the failing TestAdd, and the browser Applies through
 * the fix loop until the engine converges — the applier editing ONLY
 * production source under services/*.
 *
 * Oracle (plan §4.5): the failing test AND architecture.yaml are
 * byte-identical; only services/* production code changed. After
 * convergence this test independently runs `go test -C
 * <fixture>/services/calculator ./...` and asserts it passes.
 */

import { execFileSync } from "node:child_process";
import path from "node:path";

import { expect, test } from "@playwright/test";

import { newRunToken } from "./helpers/api-client";
import { ClaudeCallBudget } from "./helpers/claude-budget";
import { skipUnlessClaudeAvailable } from "./helpers/claude-gate";
import { AI_TERMINAL_TIMEOUT_MS, applyUntilTerminal } from "./helpers/fix-loop";
import { ProtocolEnv } from "./helpers/protocol-env";
import { gotoRun } from "./helpers/ui";
import { expectOnlyPathsChanged } from "./helpers/tree-hash";

const AI_CALL_BUDGET = 12;
const APPLY_CAP = 5; // build-code config.max_apply_attempts

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

test("A6: build code --fix makes the failing Go test pass, touching only services/*", async ({ page }) => {
  skipUnlessClaudeAvailable(test);

  env = await ProtocolEnv.start("a6-build-code-fix");
  const e = env;

  const fixture = await e.materialize("a6-build-code-fix");
  const remote = await e.startRemote(fixture.target);
  const session = await e.api.waitForSession((candidate) => candidate.pid === remote.pid);
  e.note({ sessionId: session.session_id });

  const budget = new ClaudeCallBudget(remote.pid).start();

  const { runId } = await e.api.dispatchRun(session.session_id, {
    command: "build-code",
    fix: true,
    client_token: newRunToken(),
  });
  e.note({ runId });

  await gotoRun(page, e.server.baseURL, session.session_id, runId);

  const { run: terminal, applies } = await applyUntilTerminal(page, e.api, session.session_id, runId, {
    cap: APPLY_CAP,
    label: "A6",
    stepTimeoutMs: AI_TERMINAL_TIMEOUT_MS,
  });
  expect(applies).toBeGreaterThanOrEqual(1);
  expect(terminal.outcome).toBe("converged");

  budget.stop();
  budget.assertWithinBudget(AI_CALL_BUDGET, "A6");

  // Oracle: only production source under services/* changed. The failing
  // test and architecture.yaml are byte-identical (neither may appear in
  // the diff, which is confined to services/**).
  const diff = await expectOnlyPathsChanged(fixture.baseline, fixture.target, ["services/**"]);
  expect(diff.modified).toContain("services/calculator/calc.go");
  expect(diff.modified).not.toContain("services/calculator/calc_test.go");
  expect([...diff.created, ...diff.modified, ...diff.deleted]).not.toContain(
    "docs/architecture/architecture.yaml",
  );

  // Independent verdict: run the module's tests ourselves.
  const moduleDir = path.join(fixture.target, "services", "calculator");
  expect(() =>
    execFileSync("go", ["test", "-C", moduleDir, "-count=1", "./..."], { stdio: "pipe" }),
  ).not.toThrow();
});
