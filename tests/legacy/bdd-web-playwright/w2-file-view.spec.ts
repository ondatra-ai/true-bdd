/**
 * w2 — GitHub-style file page + edit-in-place (P8, P17).
 *
 * RED intent: no FileView exists yet, so w2.1 fails on the missing path
 * header / gutter / verbatim content (and the per-run token defeats a hardcoded
 * placeholder), and w2.2 fails on the missing edit-in-place chrome discipline.
 */

import { expect, test } from "@playwright/test";

import { WTID, gotoWorkspace, wsRoutes } from "./helpers/ui";
import { WorkspaceEnv } from "./helpers/workspace-env";

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

test("w2.1 architecture is ONE GitHub-style file page: path + gutter + verbatim content + unique token (P8)", async ({
  page,
}) => {
  env = await WorkspaceEnv.start("w2-file-view");
  const e = env;
  const token = e.seedTokenInDoc("docs/architecture/architecture.yaml");

  await gotoWorkspace(page, e.baseURL, wsRoutes.architecture(e.sid));

  await expect(page.getByTestId(WTID.fileViewPath)).toHaveText("docs/architecture/architecture.yaml");

  const editor = page.getByTestId(WTID.fileViewEditor);
  await expect(editor).toContainText("services:");
  await expect(editor).toContainText("mcp-service:");
  await expect(editor).toContainText(token); // the per-run token — a placeholder cannot pass

  // Gutter: one line-number entry per buffer line (count == buffer line count).
  const bytes = e.readDocOnDisk("docs/architecture/architecture.yaml");
  await expect(page.getByTestId(WTID.fileViewGutterLine)).toHaveCount(bytes.split("\n").length);

  // No per-entity form pages linked from the single file view.
  await expect(page.getByRole("link", { name: /form|per-entity/i })).toHaveCount(0);
});

test("w2.2 edit-in-place: no background/border/outline/shadow change on focus, no text reflow (P17)", async ({
  page,
}) => {
  env = await WorkspaceEnv.start("w2-edit-in-place");
  const e = env;

  await gotoWorkspace(page, e.baseURL, wsRoutes.architecture(e.sid));
  const editor = page.getByTestId(WTID.fileViewEditor);
  await expect(editor).toBeVisible();

  const styleOf = () =>
    editor.evaluate((el) => {
      const s = getComputedStyle(el);

      return {
        bg: s.backgroundColor,
        border: s.borderStyle,
        outline: s.outlineStyle,
        shadow: s.boxShadow,
        // Editor top RELATIVE TO the file-view container, so this isolates a real
        // REFLOW from a native scroll-into-view. On section pages with the shared
        // header (mockup-parity, 2026-08-07) the editor can extend below the
        // scrollable content-pane, so clicking it scrolls the PANE (not window)
        // to reveal the caret — a scroll, not a reflow. The editor's offset
        // within the file-view is scroll-invariant and still moves on a real reflow.
        top:
          el.getBoundingClientRect().top -
          (el.closest('[data-testid="file-view"]')?.getBoundingClientRect().top ?? 0),
      };
    });

  const before = await styleOf();
  await editor.click(); // enter edit mode (focus)
  const after = await styleOf();

  // Background transparent in BOTH states (ClickUp edit-in-place fact).
  expect(before.bg).toBe("rgba(0, 0, 0, 0)");
  expect(after.bg).toBe("rgba(0, 0, 0, 0)");
  // Border / outline / shadow unchanged.
  expect(after.border).toBe(before.border);
  expect(after.outline).toBe(before.outline);
  expect(after.shadow).toBe(before.shadow);
  // No text reflow: the editor's content box does not shift on focus.
  expect(after.top).toBe(before.top);
});
