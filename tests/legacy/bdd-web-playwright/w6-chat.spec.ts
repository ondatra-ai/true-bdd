/**
 * w6 — docked chat panel (P10, P11, P12). Uses the DETERMINISTIC chat driver
 * (a `@probe append <text>` directive short-circuits the Claude turn with a
 * scripted structured result) so protocol tests exercise the identical
 * buffer→doc_write persistence path with no model call. The real Claude turn is
 * proven separately in a10.
 */

import { parse } from "yaml";
import { expect, test } from "@playwright/test";

import {
  WTID,
  gotoWorkspace,
  railItem,
  readEditor,
  sendChat,
  sidebarFeatureRow,
  sidebarGroup,
  sidebarScenarioRow,
  sidebarSection,
  sidebarStoryRow,
  writeEditor,
  wsRoutes,
} from "./helpers/ui";
import { WorkspaceEnv, installDocWriteSpy, runToken } from "./helpers/workspace-env";

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

test("w6.1 chat docks in-layout (narrows content, no overlay), collapses to a thin edge tab (P11)", async ({ page }) => {
  env = await WorkspaceEnv.start("w6-dock");
  const e = env;

  await gotoWorkspace(page, e.baseURL, wsRoutes.architecture(e.sid));
  const content = page.getByTestId(WTID.contentPane);
  const widthBefore = (await content.boundingBox())!.width;

  await page.getByTestId(WTID.chatDockToggle).click();
  const panel = page.getByTestId(WTID.chatDockPanel);
  await expect(panel).toBeVisible();
  const widthAfter = (await content.boundingBox())!.width;
  expect(widthAfter).toBeLessThan(widthBefore); // content narrows

  // Flow sibling, not an overlay: content right edge ≤ panel left edge; not fixed.
  const cb = (await content.boundingBox())!;
  const pb = (await panel.boundingBox())!;
  expect(cb.x + cb.width).toBeLessThanOrEqual(pb.x + 1);
  expect(await panel.evaluate((el) => getComputedStyle(el).position)).not.toBe("fixed");

  // Collapse → panel gone, a thin edge tab remains.
  await page.getByTestId(WTID.chatDockToggle).click();
  await expect(page.getByTestId(WTID.chatDockPanel)).toBeHidden();
  await expect(page.getByTestId(WTID.chatDockToggle)).toBeVisible();
});

test("w6.2 one workspace-wide conversation follows navigation (P10)", async ({ page }) => {
  env = await WorkspaceEnv.start("w6-conversation");
  const e = env;

  await gotoWorkspace(page, e.baseURL, wsRoutes.architecture(e.sid));
  await page.getByTestId(WTID.chatDockToggle).click();
  const marker = runToken("msg");
  await sendChat(page, marker);
  await expect(page.getByTestId(WTID.chatDockHistory)).toContainText(marker);

  // CLIENT-SIDE navigation (rail + sidebar Links) — a full page.goto would blow
  // away the persistent layout and make this test meaningless. The persistent
  // layout must keep the conversation across the route change.
  await railItem(page, "product").click();
  await sidebarStoryRow(sidebarSection(page, "product"), "60.2").click();
  await expect(page).toHaveURL(new RegExp(`/sessions/${e.sid}/product/stories/`));
  await expect(page.getByTestId(WTID.chatDockHistory)).toContainText(marker);
});

test("w6.3 chat edits the current file (two-part S1 receipt); converses only on feature + Home pages (P10)", async ({
  page,
}) => {
  env = await WorkspaceEnv.start("w6-chat-edit");
  const e = env;
  const spy = installDocWriteSpy(page);

  await gotoWorkspace(page, e.baseURL, wsRoutes.architecture(e.sid));
  await page.getByTestId(WTID.chatDockToggle).click();

  // A deterministic chat edit that adds a VALID named term (not just a comment)
  // so the LIVE outline re-derives — a comment-only writer must not pass.
  const term = runToken("term");
  await sendChat(page, `@probe add-term ${term}`);

  const editor = page.getByTestId(WTID.fileViewEditor);
  await expect(editor).toContainText(term); // buffer updates immediately
  const termsGroup = sidebarGroup(sidebarSection(page, "architecture"), "Terms");
  await expect(termsGroup.locator(`[data-testid="${WTID.archTermRow}"][data-term="${term}"]`)).toBeVisible(); // live outline

  // The CLI persisted it — parse the exact term node + the SAME two-part S1
  // receipt oracle as w4.1 (correlated by work_id).
  const bytes = await e.waitForDocOnDisk(ARCH, (b) => b.includes(term));
  expect((parse(bytes) as { terms: Array<{ name: string }> }).terms.some((t) => t.name === term)).toBe(true);
  const diskHash = e.hashDocOnDisk(ARCH);
  const browserReceipt = await spy.waitForReceipt();
  expect(browserReceipt.path).toBe(ARCH);
  expect(browserReceipt.content_hash).toBe(diskHash);
  const cliReceipt = (await e.readReceiptAudit()).find((r) => r.work_id === browserReceipt.work_id);
  expect(cliReceipt, "audit hook recorded the CLI receipt for this chat write's work_id").toBeTruthy();
  expect(cliReceipt).toMatchObject({
    path: ARCH,
    committed_revision: browserReceipt.committed_revision,
    content_hash: diskHash,
  });

  // Non-file pages (feature aggregation AND Home): chat REPLIES but performs NO
  // edit. Navigate CLIENT-SIDE (a full page.goto would reset the persistent chat)
  // and wait for the assistant reply (turn FINISHED) before snapshotting, so a
  // delayed edit cannot slip past a premature snapshot.
  const toFeature = async () => {
    await railItem(page, "product").click();
    await sidebarFeatureRow(sidebarSection(page, "product"), "summaries").click();
  };
  const toHome = async () => railItem(page, "home").click();

  const assistantMsgs = page.locator(`[data-testid="${WTID.chatDockMessage}"][data-role="assistant"]`);
  for (const navigate of [toFeature, toHome]) {
    await navigate();
    const before = e.snapshotDocs();
    const writesBefore = spy.writeCount;
    const repliesBefore = await assistantMsgs.count();
    await sendChat(page, `@probe add-term nowrite-${runToken()}`);
    await expect(assistantMsgs).toHaveCount(repliesBefore + 1); // turn finished
    expect(e.changedDocsSince(before)).toEqual([]); // authoritative negative oracle
    expect(spy.writeCount).toBe(writesBefore); // diagnostic supplement: zero doc_write
  }
});

test("w6.4 dragging the chat divider resizes the panel by ≈Δ (P12)", async ({ page }) => {
  env = await WorkspaceEnv.start("w6-resize");
  const e = env;

  await gotoWorkspace(page, e.baseURL, wsRoutes.architecture(e.sid));
  await page.getByTestId(WTID.chatDockToggle).click();
  const panel = page.getByTestId(WTID.chatDockPanel);
  const w0 = (await panel.boundingBox())!.width;

  const resizer = page.getByTestId(WTID.chatDockResizer);
  const rb = (await resizer.boundingBox())!;
  const delta = 120;
  await page.mouse.move(rb.x + rb.width / 2, rb.y + rb.height / 2);
  await page.mouse.down();
  await page.mouse.move(rb.x + rb.width / 2 - delta, rb.y + rb.height / 2, { steps: 8 }); // drag left → widen
  await page.mouse.up();

  const w1 = (await panel.boundingBox())!.width;
  expect(Math.abs(w1 - w0 - delta)).toBeLessThan(24); // width changed by ≈Δ
});

test("w6.5 scenarios file: typing a new id updates the outline live; a chat edit also updates + persists (P9)", async ({
  page,
}) => {
  env = await WorkspaceEnv.start("w6-product-live");
  const e = env;
  const spy = installDocWriteSpy(page);

  await gotoWorkspace(page, e.baseURL, wsRoutes.scenarios(e.sid));
  const product = sidebarSection(page, "product");
  const editor = page.getByTestId(WTID.fileViewEditor);

  // DISJOINT id ranges so the by-hand and chat ids can NEVER collide (a collision
  // would let the by-hand edit pre-satisfy the chat assertions).
  const byHand = `E2E-1${Math.floor(Math.random() * 900) + 100}`;
  const chatId = `E2E-8${Math.floor(Math.random() * 900) + 100}`;

  // By hand: type a new scenario id → the Scenarios outline gains a row live.
  const current = await readEditor(editor);
  await writeEditor(
    editor,
    current.replace(/\n {2}INT-901:/, `\n  ${byHand}:\n    description: "hand"\n    service: "mcp-service"\n    user_stories: []\n  INT-901:`),
  );
  await expect(sidebarScenarioRow(product, byHand)).toBeVisible();

  // By chat: a SCHEMA-AWARE deterministic edit adds a DISTINCT scenario → the
  // outline gains its row live AND the CHAT ITSELF persists it via a NEW
  // doc_write whose receipt matches disk (not pre-satisfied by the by-hand edit).
  await page.getByTestId(WTID.chatDockToggle).click();
  const writesBefore = spy.writeCount;
  await sendChat(page, `@probe add-scenario ${chatId}`);
  await expect(sidebarScenarioRow(product, chatId)).toBeVisible();
  const bytes = await e.waitForDocOnDisk("docs/scenarios.yaml", (b) => b.includes(chatId));
  expect((parse(bytes) as { scenarios: Record<string, unknown> }).scenarios[chatId]).toBeTruthy();
  expect(spy.writeCount).toBeGreaterThan(writesBefore); // the chat issued its own doc_write
  await expect
    .poll(() => spy.receipts.some((r) => r.content_hash === e.hashDocOnDisk("docs/scenarios.yaml")))
    .toBe(true);
});
