/**
 * The `ai-gate` project's only "test": run the Claude availability
 * probe ONCE per invocation and persist the result for the A-tests
 * and the AI banner reporter (plan §4.4).
 *
 * Deliberately NEVER fails on Claude unavailability — unavailability
 * turns the AI project into loud skips (each A-test calls
 * skipUnlessClaudeAvailable), and the reporter prints the reason.
 * Infrastructure errors (suite context missing, gate file unwritable)
 * DO fail — those are failures, not skips.
 */

import { test } from "@playwright/test";

import { gateFilePath, probeClaudeGate, writeGateResult } from "./helpers/claude-gate";

test("claude gate probe", async () => {
  const result = await probeClaudeGate();
  writeGateResult(result);

  test.info().annotations.push({
    type: "claude-gate",
    description: result.ok
      ? `available (model ${result.model})`
      : `UNAVAILABLE — ${result.reason}`,
  });

  console.log(
    `[ai-gate] ${result.ok ? "claude available" : `claude UNAVAILABLE: ${result.reason}`} ` +
      `(recorded at ${gateFilePath()})`,
  );
});
