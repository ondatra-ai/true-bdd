/**
 * w20 — workspace visual stability (task `workspace-visual-jank`, P1–P5). Pins the
 * five instrumented-walk defects (report: tmp/visual-walk/report.md) as e2e
 * assertions. RED intent against the current build:
 *   - P1 (w20.1): Home entry shifts the canvas 39px when the folder path lands.
 *   - P2 (w20.2): the chat dock defaults to a fixed 768px (prototype DEFAULT_WIDTH=380).
 *   - P3 (w20.3): the rail flyout (w=260) is 20px narrower than the sidebar (w=280) it covers.
 *   - P4 (w20.4): a non-contract architecture.yaml renders bare empty groups with no hint.
 *   - P5 (w20.5): the Poppins font-swap reflows text ~305ms after first paint.
 *
 * Layout-shift oracle: an injected PerformanceObserver('layout-shift') (the working
 * pattern from tmp/visual-walk/walk.mjs), buffered from load, filtered to
 * hadRecentInput=false (non-input) shifts. Each test owns a fresh WorkspaceEnv
 * container on its own port AND a fresh (cache-isolated) browser context, so the
 * font-swap is cold on every run.
 */

import fs from "node:fs";
import path from "node:path";

import { expect, test, type Page } from "@playwright/test";

import { WTID, gotoWorkspace, railItem, sidebarGroup, sidebarSection, wsRoutes } from "./helpers/ui";
import { WorkspaceEnv } from "./helpers/workspace-env";

// The whole suite reproduces at the report's viewport (the fixed 768/260 numbers
// were probed at 1440×900).
test.use({ viewport: { width: 1440, height: 900 } });

const ARCH = "docs/architecture/architecture.yaml";

/**
 * A non-contract architecture.yaml: everything nested under `architecture:` with
 * `vocabulary:` instead of top-level `services:`/`terms:`/`docker:`. Valid YAML, so
 * the outline derives 0 rows per fixed group with NO invalid indicator — the exact
 * A/B shape from tests/harness/fixtures/p3-inventory-spread/input/docs/architecture.
 */
// A unique marker line so the test can PROVE the browser rendered this wrapped file
// (not the happy fixture / a pre-load blank) before asserting on the derived outline.
const WRAPPED_MARKER = "w20-p4-wrapped-shape";
const WRAPPED_ARCH = `# Non-contract shape (see report finding 4): valid YAML, zero contract keys.
# ${WRAPPED_MARKER}
architecture:
  services:
    - name: mcp-service
      path: services/mcp
      language: go
vocabulary:
  actions:
    - name: "ask Claude"
      means: "write a prompt in Claude chat"
`;

interface ShiftSource {
  testid: string | null;
  cls: string;
  prevY: number;
  currY: number;
}
interface LayoutShift {
  t: number;
  value: number;
  hadRecentInput: boolean;
  sources: ShiftSource[];
}

interface ShiftStore {
  __ls: LayoutShift[];
  __lsReady: boolean;
  __lsFlush?: () => void;
}

/** Installs a buffered layout-shift observer BEFORE navigation (walk.mjs pattern). */
async function observeLayoutShifts(page: Page): Promise<void> {
  await page.addInitScript(() => {
    const store = window as unknown as ShiftStore;
    store.__ls = [];
    store.__lsReady = false;
    const record = (entry: PerformanceEntry): void => {
      const e = entry as unknown as {
        startTime: number;
        value: number;
        hadRecentInput: boolean;
        sources?: Array<{ node: Element | null; previousRect: DOMRectReadOnly; currentRect: DOMRectReadOnly }>;
      };
      store.__ls.push({
        t: Math.round(e.startTime),
        value: e.value,
        hadRecentInput: e.hadRecentInput,
        sources: (e.sources ?? []).map((s) => {
          const node = s.node;
          const testid = node && node.getAttribute ? node.getAttribute("data-testid") : null;
          const cls = node && typeof node.className === "string" ? node.className : "";

          return { testid, cls, prevY: Math.round(s.previousRect.y), currY: Math.round(s.currentRect.y) };
        }),
      });
    };
    // NOTE: no swallowing catch — a silent observer failure would false-GREEN a spec
    // whose whole purpose is to be RED. If the API is missing the init throws and the
    // page surfaces it; the test also asserts `__lsReady` before trusting the buffer.
    const observer = new PerformanceObserver((list) => list.getEntries().forEach(record));
    observer.observe({ type: "layout-shift", buffered: true });
    store.__lsFlush = () => observer.takeRecords().forEach(record); // drain undelivered records before reads
    store.__lsReady = true;
  });
}

/** Flushes pending records, then returns the readiness flag + the collected shifts. */
async function collectShifts(page: Page): Promise<{ ready: boolean; shifts: LayoutShift[] }> {
  return page.evaluate(() => {
    const store = window as unknown as ShiftStore;
    store.__lsFlush?.();

    return { ready: store.__lsReady === true, shifts: store.__ls ?? [] };
  });
}

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

test("w20.1 Home entry: live data arrival causes no non-input layout shift on the overview canvas (P1)", async ({
  page,
}) => {
  env = await WorkspaceEnv.start("w20-home-stability");
  const e = env;

  await observeLayoutShifts(page);
  await gotoWorkspace(page, e.baseURL, wsRoutes.home(e.sid));

  // The overview canvas is visible FIRST (placeholder meta), THEN live data lands.
  await expect(page.getByTestId(WTID.overviewTitle)).toBeVisible();
  await expect(page.getByTestId(WTID.overviewInventory)).toBeVisible();

  // Wait for the LIVE folder path (a realpath contains "/") to replace the
  // placeholder "… — session <id>" meta — the arrival that must NOT move the canvas.
  await expect(page.getByTestId(WTID.overviewMeta)).toContainText("/", { timeout: 20_000 });
  // Also wait for the inventory list to SETTLE (every chip past its pending "…"): a
  // regression that re-sorted rows on the live read (reordering the skeleton) emits its
  // shift only when the scan lands, so without this the fixed-750ms window could close
  // BEFORE a slow scan and false-green the reorder guard (reviewer Codex r1 #1).
  const chips = page.getByTestId(WTID.overviewInventoryChip);
  await expect
    .poll(
      async () => {
        const texts = await chips.evaluateAll((els) => els.map((el) => (el.textContent ?? "").trim()));

        return texts.length > 0 && texts.every((t) => t.length > 0 && t !== "…");
      },
      { timeout: 20_000, message: "overview inventory settled (no pending … chip) before shift collection" },
    )
    .toBe(true);
  await page.waitForTimeout(750); // flush any post-arrival shift into the observer buffer

  const { ready, shifts } = await collectShifts(page);
  expect(ready, "layout-shift observer initialized (else the empty buffer would false-green)").toBe(true);
  const overviewJumps = shifts.filter(
    (s) =>
      !s.hadRecentInput &&
      s.sources.some(
        (src) => (src.testid?.startsWith("overview") === true || /overview/.test(src.cls)) && src.prevY !== src.currY,
      ),
  );
  expect(overviewJumps, `non-input overview layout shifts: ${JSON.stringify(overviewJumps)}`).toEqual([]);
});

test("w20.2 chat dock opens at the prototype default width (380px), content stays usable, resizer present (P2)", async ({
  page,
}) => {
  env = await WorkspaceEnv.start("w20-chat-width");
  const e = env;

  await gotoWorkspace(page, e.baseURL, wsRoutes.architecture(e.sid));
  await page.getByTestId(WTID.chatDockToggle).click();

  const panel = page.getByTestId(WTID.chatDockPanel);
  await expect(panel).toBeVisible();
  await expect(page.getByTestId(WTID.chatDockResizer)).toBeVisible(); // the drag-resizer remains

  const panelWidth = (await panel.boundingBox())!.width;
  expect(Math.abs(panelWidth - 380), `chat-dock-panel default width (prototype DEFAULT_WIDTH=380), got ${panelWidth}`).toBeLessThanOrEqual(
    2,
  );

  // Content pane stays usable at 1440 (not squeezed to the ~310px the 768 default left).
  const contentPane = page.getByTestId(WTID.contentPane);
  await expect(contentPane).toBeVisible();
  const paneWidth = (await contentPane.boundingBox())!.width;
  expect(paneWidth, `content pane width ${paneWidth} must exceed the chat panel ${panelWidth}`).toBeGreaterThan(panelWidth);
});

test("w20.3 rail flyout covers the docked sidebar exactly — no underlying sliver (P3)", async ({ page }) => {
  env = await WorkspaceEnv.start("w20-flyout-cover");
  const e = env;

  await gotoWorkspace(page, e.baseURL, wsRoutes.architecture(e.sid));
  const sidebar = page.getByTestId(WTID.sidebar);
  await expect(sidebar).toBeVisible(); // guard: boundingBox() is null until it renders (crash ≠ valid RED)
  const sidebarBox = (await sidebar.boundingBox())!;

  // Hover a NON-active section (Product) → its preview flyout floats over the sidebar.
  await railItem(page, "product").hover();
  const flyout = page.getByTestId(WTID.railFlyout);
  await expect(flyout).toBeVisible();
  const flyoutBox = (await flyout.boundingBox())!;

  expect(Math.abs(flyoutBox.x - sidebarBox.x), "flyout left edge == sidebar left edge").toBeLessThanOrEqual(1);
  expect(
    Math.abs(flyoutBox.width - sidebarBox.width),
    `flyout width ${flyoutBox.width} must match sidebar width ${sidebarBox.width} (no right-edge sliver)`,
  ).toBeLessThanOrEqual(1);
  expect(flyoutBox.height, "flyout covers the sidebar's full height").toBeGreaterThanOrEqual(sidebarBox.height - 1);
});

test("w20.4 non-contract architecture.yaml: each fixed outline group shows an explicit empty indicator (P4)", async ({
  page,
}) => {
  env = await WorkspaceEnv.start("w20-empty-groups");
  const e = env;

  // Seed the non-contract shape on disk BEFORE navigation: the CLI serves a fresh
  // scan (no relay write), same disk-seed pattern as WorkspaceEnv.seedTokenInDoc.
  fs.writeFileSync(path.join(e.fixtureDir, ARCH), WRAPPED_ARCH);

  await gotoWorkspace(page, e.baseURL, wsRoutes.architecture(e.sid));
  const sidebar = sidebarSection(page, "architecture");
  await expect(sidebar).toBeVisible();

  // PROVE the browser rendered the seeded wrapped file (a fresh CLI scan of disk) —
  // otherwise the zero-count checks below pass trivially pre-load and the missing
  // indicator gets blamed on absent UI rather than the wrapped shape.
  await expect(page.getByTestId(WTID.fileViewEditor)).toContainText(WRAPPED_MARKER, { timeout: 20_000 });

  // Red-for-the-right-reason precondition: valid YAML, zero contract rows.
  await expect(page.getByTestId(WTID.yamlInvalidIndicator)).toHaveCount(0);
  await expect(sidebar.getByTestId(WTID.archServiceRow)).toHaveCount(0);
  await expect(sidebar.getByTestId(WTID.archTermRow)).toHaveCount(0);
  await expect(sidebar.getByTestId(WTID.archDockerRow)).toHaveCount(0);

  // The behavior under test: an explicit, non-empty empty indicator in EACH fixed group.
  const rowTid: Record<string, string> = {
    Services: WTID.archServiceRow,
    Terms: WTID.archTermRow,
    Docker: WTID.archDockerRow,
  };
  for (const group of ["Services", "Terms", "Docker"] as const) {
    const g = sidebarGroup(sidebar, group);
    await expect(g, `${group} group header renders (never silently bare)`).toBeVisible();
    await expect(g.getByTestId(rowTid[group])).toHaveCount(0);
    const empty = g.getByTestId(WTID.sidebarGroupEmpty);
    await expect(empty, `${group} empty indicator visible`).toBeVisible();
    await expect(empty, `${group} empty indicator non-empty`).not.toBeEmpty();
  }
});

test("w20.4b happy architecture.yaml: populated fixed groups render NO empty indicator (P4 negative guard)", async ({
  page,
}) => {
  env = await WorkspaceEnv.start("w20-nonempty-groups");
  const e = env;

  await gotoWorkspace(page, e.baseURL, wsRoutes.architecture(e.sid));
  const sidebar = sidebarSection(page, "architecture");
  await expect(sidebarGroup(sidebar, "Services").getByTestId(WTID.archServiceRow).first()).toBeVisible();

  // A populated group must NOT carry the empty indicator (guard against over-eager rendering).
  await expect(sidebar.getByTestId(WTID.sidebarGroupEmpty)).toHaveCount(0);
});

test("w20.5 idle architecture first load produces zero non-input layout shifts in the first 3s (P5, font swap included)", async ({
  page,
}) => {
  env = await WorkspaceEnv.start("w20-arch-stability");
  const e = env;

  await observeLayoutShifts(page);
  await gotoWorkspace(page, e.baseURL, wsRoutes.architecture(e.sid));

  await expect(page.getByTestId(WTID.contentBreadcrumb)).toBeVisible();
  await expect(page.getByTestId(WTID.fileView)).toBeVisible();
  await page.waitForTimeout(3200); // observe the first ~3s of the idle load (font swap fires ~305ms)

  const { ready, shifts } = await collectShifts(page);
  expect(ready, "layout-shift observer initialized (else the empty buffer would false-green)").toBe(true);
  const nonInput = shifts.filter((s) => !s.hadRecentInput && s.t <= 3000);
  expect(nonInput, `non-input layout shifts in first 3s: ${JSON.stringify(nonInput)}`).toEqual([]);
});
