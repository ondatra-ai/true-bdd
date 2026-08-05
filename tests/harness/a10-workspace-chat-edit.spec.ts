/**
 * a10 (ai project) — a REAL Claude chat turn edits the architecture file and the
 * CLI persists it, proving the production chat → doc_write → CLI path end-to-end
 * (the deterministic w6.3 covers the same path cheaply; this proves the real
 * integration). Counts toward the AI-call budget.
 */

import { expect, test } from "@playwright/test";

import { parse } from "yaml";

import { ClaudeCallBudget } from "./helpers/claude-budget";
import { skipUnlessClaudeAvailable } from "./helpers/claude-gate";
import { AI_TERMINAL_TIMEOUT_MS } from "./helpers/fix-loop";
import { WTID, gotoWorkspace, sendChat, wsRoutes } from "./helpers/ui";
import { WorkspaceEnv, installDocWriteSpy, runToken } from "./helpers/workspace-env";

const ARCH = "docs/architecture/architecture.yaml";
const AI_CALL_BUDGET = 3;

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

test("a10: a real Claude chat turn edits architecture.yaml and the CLI persists it (P10/S1)", async ({ page }) => {
  skipUnlessClaudeAvailable(test);

  // deterministicChat: false → the CLI runs a REAL Claude turn for the chat item.
  env = await WorkspaceEnv.start("a10-workspace-chat", { deterministicChat: false });
  const e = env;
  const spy = installDocWriteSpy(page);
  const budget = new ClaudeCallBudget(e.remotePid).start();

  await gotoWorkspace(page, e.baseURL, wsRoutes.architecture(e.sid));
  await page.getByTestId(WTID.chatDockToggle).click();

  const token = runToken("term");
  await sendChat(
    page,
    `Add a new term named "${token}" with a one-line description to the terms list in this architecture file. ` +
      `Keep the rest of the file unchanged and valid YAML.`,
  );

  // The change appears in the editor AND the CLI persisted it — parse the exact
  // term node on disk (node-scoped, not a substring).
  const editor = page.getByTestId(WTID.fileViewEditor);
  await expect(editor).toContainText(token, { timeout: AI_TERMINAL_TIMEOUT_MS });
  const bytes = await e.waitForDocOnDisk(ARCH, (b) => b.includes(token), { timeoutMs: AI_TERMINAL_TIMEOUT_MS });
  expect((parse(bytes) as { terms: Array<{ name: string }> }).terms.some((t) => t.name === token)).toBe(true);
  const diskHash = e.hashDocOnDisk(ARCH);

  // The persistence went through the SAME doc_write path — the FULL two-part S1
  // receipt oracle (browser response AND relay audit hook, correlated by work_id).
  const receipt = await spy.waitForReceipt(AI_TERMINAL_TIMEOUT_MS);
  expect(receipt.path).toBe(ARCH);
  expect(receipt.content_hash).toBe(diskHash);
  const cliReceipt = (await e.readReceiptAudit()).find((r) => r.work_id === receipt.work_id);
  expect(cliReceipt, "audit hook recorded the CLI receipt for this real-chat write's work_id").toBeTruthy();
  expect(cliReceipt).toMatchObject({
    path: ARCH,
    committed_revision: receipt.committed_revision,
    content_hash: diskHash,
  });

  budget.stop();
  budget.assertWithinBudget(AI_CALL_BUDGET, "a10");
});
