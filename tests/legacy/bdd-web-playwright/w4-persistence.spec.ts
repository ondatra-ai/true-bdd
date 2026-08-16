/**
 * w4 — CLI persistence, autosave, invalid YAML (S1, P10a, P10b). CORE.
 *
 * The S1 oracle is TWO-PART (plan §"Why this architecture"): (1) the on-disk
 * change proves the relay did NOT write it (the e2e container has no host-folder
 * mount), and (2) the CLI write receipt — carried on the browser `doc_write`
 * RESPONSE AND recorded by the test-only relay receipt-audit hook keyed by
 * work_id — proves the CLI, not merely some host process, wrote it. Together:
 * browser → relay → CLI → disk.
 *
 * Waits are on the OBSERVABLE save-state / `waitForDocOnDisk`, never a sleep.
 * w4.1–w4.3 share ONE env (order-independent by construction).
 */

import { parse } from "yaml";
import { expect, test } from "@playwright/test";

import { WTID, gotoWorkspace, readEditor, saveState, sidebarGroup, sidebarSection, writeEditor, wsRoutes } from "./helpers/ui";
import { WorkspaceEnv, installDocWriteSpy, runToken, type WriteReceipt } from "./helpers/workspace-env";

const ARCH = "docs/architecture/architecture.yaml";

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

test("w4.1–w4.3 direct edit autosaves through the CLI (two-part S1 oracle), survives reload + restart (S1/P10a)", async ({
  page,
}) => {
  env = await WorkspaceEnv.start("w4-persist");
  const e = env;
  const spy = installDocWriteSpy(page);

  await gotoWorkspace(page, e.baseURL, wsRoutes.architecture(e.sid));
  const editor = page.getByTestId(WTID.fileViewEditor);
  await expect(editor).toBeVisible();

  const token = runToken("term");
  const current = await readEditor(editor);
  await writeEditor(editor, current.replace(/\nterms:\n/, `\nterms:\n  - name: ${token}\n    description: unique per-run term\n`));

  // Observable save-state → saved (deterministic; never a sleep).
  await expect(saveState(page)).toHaveAttribute("data-save-state", "saved");

  // (1) On-disk change — the relay is ruled out (no container mount). Parse the
  // committed file and assert the term lives under the exact `terms` node
  // (node-scoped, not a substring — anti-placeholder discipline).
  const bytes = await e.waitForDocOnDisk(ARCH, (b) => b.includes(token));
  const doc = parse(bytes) as { terms: Array<{ name: string }> };
  expect(doc.terms.some((t) => t.name === token)).toBe(true);
  const diskHash = e.hashDocOnDisk(ARCH);

  // (2) CLI receipt — browser RESPONSE receipt AND the relay receipt-audit hook,
  // CORRELATED BY THE SAME work_id, both matching the on-disk SHA-256 (proves the
  // CLI, not merely a host process, wrote THIS write).
  const browserReceipt = await spy.waitForReceipt();
  expect(browserReceipt.path).toBe(ARCH);
  expect(browserReceipt.content_hash).toBe(diskHash);
  expect(browserReceipt.work_id).toBeTruthy();

  const audit = await e.readReceiptAudit();
  const cliReceipt = audit.find((r) => r.work_id === browserReceipt.work_id);
  expect(cliReceipt, "the relay receipt-audit hook recorded the CLI receipt for this work_id").toBeTruthy();
  expect(cliReceipt).toMatchObject({
    path: ARCH,
    committed_revision: browserReceipt.committed_revision,
    content_hash: diskHash,
  });

  // w4.2 — survives reload (fresh doc_read).
  await page.reload();
  await expect(page.getByTestId(WTID.fileViewEditor)).toContainText(token);

  // w4.3 — survives harness restart: recreate the container, wait for the agent
  // to RE-REGISTER before reloading (never reload before reconnect).
  await e.server.restart();
  await e.waitForReconnect();
  await page.reload();
  await expect(page.getByTestId(WTID.fileViewEditor)).toContainText(token);
  expect(e.readDocOnDisk(ARCH)).toContain(token);
});

test("w4.4 invalid YAML is not committed; derived views hold last-valid, then recover (P10a/P10b)", async ({ page }) => {
  env = await WorkspaceEnv.start("w4-invalid");
  const e = env;
  // Observe doc_write traffic so the negative oracle isolates the CLIENT-side
  // gate (P10a "not committed until it parses") from the CLI's own gate: a
  // regression that dropped the client gate would POST the invalid buffer even
  // though the CLI would still 422 it and leave disk unchanged.
  const spy = installDocWriteSpy(page);

  await gotoWorkspace(page, e.baseURL, wsRoutes.architecture(e.sid));
  const editor = page.getByTestId(WTID.fileViewEditor);
  await expect(editor).toBeVisible();

  const services = sidebarGroup(sidebarSection(page, "architecture"), "Services");
  await expect(services.getByTestId(WTID.archServiceRow)).toHaveCount(2);

  const lastValid = e.readDocOnDisk(ARCH);
  const before = e.snapshotDocs();
  const writesBeforeBreak = spy.writeCount;

  // Break YAML with an unterminated flow sequence.
  const buffer = await readEditor(editor);
  await writeEditor(editor, `${buffer}\nbroken: [`);
  await expect(saveState(page)).toHaveAttribute("data-save-state", /invalid|rejected/);
  await expect(page.getByTestId(WTID.yamlInvalidIndicator)).toBeVisible();

  // The client issued NO doc_write for the invalid buffer (the invalid state is
  // reached WITHOUT any autosave POST — client-side suppression, not merely a
  // CLI rejection), disk byte-equals last valid, and the outline holds last-valid.
  expect(spy.writeCount).toBe(writesBeforeBreak);
  expect(e.changedDocsSince(before)).toEqual([]);
  expect(e.readDocOnDisk(ARCH)).toBe(lastValid);
  await expect(services.getByTestId(WTID.archServiceRow)).toHaveCount(2);

  // Correct it → on-disk updates, indicator clears, outline re-derives (+1 service).
  const token = runToken("svc");
  await writeEditor(editor, lastValid.replace(/\nterms:\n/, `\n  ${token}:\n    type: custom\n    path: services/${token}\nterms:\n`));
  await expect(page.getByTestId(WTID.yamlInvalidIndicator)).toBeHidden();
  await e.waitForDocOnDisk(ARCH, (b) => b.includes(token));
  await expect(services.getByTestId(WTID.archServiceRow)).toHaveCount(3);
});

test("w4.5a with no connected agent, doc_write returns 404 session_gone (or 504) and never claims saved (S1)", async ({
  page,
}) => {
  env = await WorkspaceEnv.start("w4-no-cli");
  const e = env;
  const spy = installDocWriteSpy(page);

  await gotoWorkspace(page, e.baseURL, wsRoutes.architecture(e.sid));
  const editor = page.getByTestId(WTID.fileViewEditor);
  await expect(editor).toBeVisible();

  const before = e.snapshotDocs();
  // Drop the agent; wait for the session to disappear (sessions are gone on disconnect).
  await e.killAgent();
  await e.waitForSessionGone();

  const failed = page.waitForResponse((r) => /\/docs\/write(\?|$)/.test(r.url()));
  const current = await readEditor(editor);
  await writeEditor(editor, `${current}\n# ${runToken("term")}\n`);
  const res = await failed;

  // The EXACT mapped error (not merely any 404/504): 404→session_gone,
  // 504→cli_timeout. Never a `saved` state.
  const status = res.status();
  expect([404, 504]).toContain(status);
  const body = (await res.json().catch(() => ({}))) as { error?: string };
  expect(body.error).toBe(status === 404 ? "session_gone" : "cli_timeout");
  await expect(saveState(page)).not.toHaveAttribute("data-save-state", "saved");
  expect(e.changedDocsSince(before)).toEqual([]); // authoritative negative oracle
  expect(spy.writeCount).toBeGreaterThan(0); // a write WAS attempted (diagnostic)
});

test("w4.5b agent dies after accepting a write: browser sees the failure, never claims false success (S1)", async ({
  page,
}) => {
  env = await WorkspaceEnv.start("w4-die-after-accept");
  const e = env;

  await gotoWorkspace(page, e.baseURL, wsRoutes.architecture(e.sid));
  const editor = page.getByTestId(WTID.fileViewEditor);
  await expect(editor).toBeVisible();

  const token = runToken("term");
  const current = await readEditor(editor);
  const writeReq = page.waitForRequest((r) => /\/docs\/write(\?|$)/.test(r.url()));
  const writeResp = page.waitForResponse((r) => /\/docs\/write(\?|$)/.test(r.url()));
  await writeEditor(editor, `${current}\n# ${token}\n`);
  await writeReq; // the debounced write was actually SENT (accepted/in-flight)…
  await e.killAgent(); // …THEN the agent dies before/while replying
  const res = await writeResp;

  // An already-delivered mutation MAY complete CLI-side, so either a lost-reply
  // failure (404/504) OR a genuine success (200) is admissible — but never a
  // FALSE success.
  expect([200, 404, 504]).toContain(res.status());
  if (res.status() === 200) {
    // A TRUE success must carry a COMPLETE receipt consistent with disk AND
    // corroborated by the audit hook (correlated by work_id) — `200 {}` or a
    // malformed receipt is the exact false success this case rejects.
    const receipt = ((await res.json()) as { receipt?: WriteReceipt }).receipt;
    expect(receipt, "a 200 doc_write must carry a complete write receipt").toBeTruthy();
    const diskHash = e.hashDocOnDisk(ARCH);
    expect(receipt).toMatchObject({ path: ARCH, content_hash: diskHash });
    expect(receipt?.work_id).toBeTruthy();
    const cliReceipt = (await e.readReceiptAudit()).find((r) => r.work_id === receipt?.work_id);
    expect(cliReceipt, "audit hook recorded the CLI receipt for this work_id").toBeTruthy();
    expect(cliReceipt).toMatchObject({
      path: ARCH,
      committed_revision: receipt?.committed_revision,
      content_hash: diskHash,
    });
  } else {
    // Lost-reply failure → the UI must NOT claim success.
    await expect(saveState(page)).not.toHaveAttribute("data-save-state", "saved");
  }
});
