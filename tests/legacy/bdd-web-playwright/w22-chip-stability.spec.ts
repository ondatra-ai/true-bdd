/**
 * w22 — workspace inventory-chip stability (visual-sweep run 20260806-163556,
 * round 1). Pins ONE finding against the current build:
 *
 *   - w22.1 (RED): `[overview-inventory-chip]/shift` — on the workspace Home
 *     page (`/sessions/<sid>/home`), the inventory-health status chips
 *     (`[data-testid="overview-inventory-chip"]`, one per inventory row,
 *     right-aligned) first render as a narrow "…" pending placeholder (~31px
 *     wide) and then, when the live inventory scan lands (~300ms later), swap
 *     to their real status text (e.g. "Present") and widen to ~87px. Because
 *     the chips are right-aligned, every chip's LEFT edge jumps ~56px leftward
 *     in one frame — a visible horizontal pop across all rows on every Home
 *     load. The pinned INTENT (from
 *     tmp/visual-sweep/20260806-163556/round-1/findings.md): once the
 *     inventory chips are visible on Home, the arrival of live inventory data
 *     must not move any chip's bounding box (horizontal OR vertical, tolerance
 *     ≤2px).
 *
 *     This escaped the existing w20.1 oracle because that layout-shift filter
 *     only counts VERTICAL source moves (`prevY !== currY`); this shift is
 *     horizontal-only (same y, prevX → currX). The reliable oracle here — per
 *     the findings file's own warning — is a direct before/after `boundingBox`
 *     comparison of every chip across the pending→live text swap.
 *
 *   - w22.2 (GREEN guard, the finding's healthy counterpart): the pending→live
 *     TEXT swap itself is intended and must keep working — chips start as
 *     pending "…" and genuinely end up showing the live status text. The fix
 *     must not freeze the placeholder or hide statuses.
 *
 * Oracle lineage: the w20 visual-stability spec's `observeLayoutShifts`
 * PerformanceObserver MISSES this defect (its source filter compares prevY vs
 * currY only — w20.1:166-172). The prompt-required reliable shape is a direct
 * `boundingBox` before/after comparison, settled with `expect.poll`
 * settle-waits and a ≤2px tolerance (the w20 tolerance convention, w20.2:190).
 * Each test owns a fresh WorkspaceEnv container (cold font-swap on every run)
 * at the probe viewport 1440×900.
 */

import { expect, test, type Page } from "@playwright/test";

import { INVENTORY_STATUSES, WTID, gotoWorkspace, wsRoutes } from "./helpers/ui";
import { WorkspaceEnv } from "./helpers/workspace-env";

test.use({ viewport: { width: 1440, height: 900 } });

interface Box {
  x: number;
  y: number;
  width: number;
  height: number;
}

interface ChipSnapshot {
  key: string;
  text: string;
  status: string;
  box: Box;
}

/**
 * Reads every inventory row's chip in ONE frame snapshot (no read-to-read skew)
 * — the chip's text + data-status + bounding box, keyed by the row's stable
 * data-key. The box is read via getBoundingClientRect inside the browser so the
 * whole snapshot is atomic w.r.t. the live-data arrival frame.
 */
async function readChipSnapshots(page: Page): Promise<ChipSnapshot[]> {
  return page
    .locator(`[data-testid="${WTID.overviewInventoryRow}"]`)
    .evaluateAll((rows, testid) => {
      const out: Array<{ key: string; text: string; status: string; box: Box }> = [];
      for (const row of rows) {
        const chip = row.querySelector(`[data-testid="${testid}"]`);
        if (!chip) continue;
        const rect = chip.getBoundingClientRect();
        out.push({
          key: row.getAttribute("data-key") ?? "",
          text: (chip.textContent ?? "").trim(),
          status: chip.getAttribute("data-status") ?? "",
          box: { x: rect.x, y: rect.y, width: rect.width, height: rect.height },
        });
      }

      return out;
    }, WTID.overviewInventoryChip);
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

test("w22.1 Home: live inventory-data arrival moves no status chip's bounding box (horizontal OR vertical)", async ({
  page,
}) => {
  env = await WorkspaceEnv.start("w22-chip-stability");
  const e = env;

  await gotoWorkspace(page, e.baseURL, wsRoutes.home(e.sid));

  // The inventory list + at least one chip render FIRST (chips in their narrow
  // pending "…" state), THEN the live scan lands ~300ms later.
  await expect(page.getByTestId(WTID.overviewInventory)).toBeVisible();
  await expect(page.getByTestId(WTID.overviewInventoryChip).first()).toBeVisible();

  // Capture the baseline (pending) snapshot BEFORE the live scan lands. A
  // one-frame atomic read so the baseline boxes are not skewed across the swap.
  const baseline = await readChipSnapshots(page);
  expect(baseline.length, "inventory rows rendered with chips").toBeGreaterThan(0);

  const pendingCount = baseline.filter((c) => c.text === "…" || c.text.length === 0).length;
  // Honest-guard: if NO chip was pending at baseline, the live scan landed
  // before we read and the box-stability assertion below could false-green —
  // fail loudly instead of silently passing on a missed window.
  expect(
    pendingCount,
    `at least one chip pending "…" at baseline (got ${pendingCount}/${baseline.length}; the live scan must not have landed before the baseline read)`,
  ).toBeGreaterThan(0);

  // Settle-wait: poll until EVERY chip has swapped off the pending placeholder.
  // This is the live-data arrival event whose geometry impact is under test.
  await expect
    .poll(
      async () => {
        const current = await readChipSnapshots(page);

        return current.length > 0 && current.every((c) => c.text.length > 0 && c.text !== "…");
      },
      { timeout: 20_000, message: "all inventory chips settled (no pending '…') before the box comparison" },
    )
    .toBe(true);

  const settled = await readChipSnapshots(page);

  // INTENT: once the chips are visible, the arrival of live inventory data must
  // not move any chip's bounding box — horizontal OR vertical, ≤2px tolerance
  // (the w20 visual-stability tolerance convention). The defect pops every
  // right-aligned chip's left edge ~56px leftward (width 31→87) when the status
  // text lands; the w20.1 vertical-only layout-shift filter lets it through, so
  // the box comparison below is the reliable oracle.
  for (const base of baseline) {
    const fin = settled.find((s) => s.key === base.key);
    if (fin === undefined) continue; // a row vanishing across the swap is not the pinned defect

    const dx = Math.abs(fin.box.x - base.box.x);
    const dy = Math.abs(fin.box.y - base.box.y);
    const dw = Math.abs(fin.box.width - base.box.width);
    const dh = Math.abs(fin.box.height - base.box.height);

    expect(
      dx,
      `chip "${base.key}" left edge moved ${dx}px horizontally on live-data arrival (x ${base.box.x} → ${fin.box.x}; text "${base.text}" → "${fin.text}")`,
    ).toBeLessThanOrEqual(2);
    expect(
      dw,
      `chip "${base.key}" width changed ${dw}px on live-data arrival (width ${base.box.width} → ${fin.box.width}; text "${base.text}" → "${fin.text}")`,
    ).toBeLessThanOrEqual(2);
    expect(
      dy,
      `chip "${base.key}" top edge moved ${dy}px vertically on live-data arrival`,
    ).toBeLessThanOrEqual(2);
    expect(
      dh,
      `chip "${base.key}" height changed ${dh}px on live-data arrival`,
    ).toBeLessThanOrEqual(2);
  }
});

test("w22.2 Home: the pending→live status-text swap lands for every chip (green guard)", async ({
  page,
}) => {
  env = await WorkspaceEnv.start("w22-chip-text-swap");
  const e = env;

  await gotoWorkspace(page, e.baseURL, wsRoutes.home(e.sid));

  await expect(page.getByTestId(WTID.overviewInventory)).toBeVisible();
  await expect(page.getByTestId(WTID.overviewInventoryChip).first()).toBeVisible();

  // Baseline read. The green guard's INTENT is that the live status swap
  // genuinely LANDS for every chip (asserted below) — NOT that we win a race
  // to catch the transient pending "…" window. The live inventory scan can
  // land before this first sample, so requiring a pending chip here would
  // flake against the app's own timing (the pin rules forbid that coupling).
  // Both starts are acceptable: observed-pending OR already-live.
  const baseline = await readChipSnapshots(page);
  expect(baseline.length, "inventory rows rendered with chips").toBeGreaterThan(0);

  // INTENT (green guard): every chip must genuinely end up showing the live
  // status text — not frozen at "…", not blank. The fix must keep the text
  // swap working even once the box-stability defect (w22.1) is resolved.
  await expect
    .poll(
      async () => {
        const current = await readChipSnapshots(page);

        return current.length > 0 && current.every((c) => c.text.length > 0 && c.text !== "…");
      },
      { timeout: 20_000, message: "all inventory chips show live status text (not pending '…')" },
    )
    .toBe(true);

  // INTENT (green guard): the live status is one of the chip vocabulary — the
  // fix must not hide statuses (e.g. render every chip as a blank to avoid the
  // pop). data-status is the contract attribute (README-testids → overview).
  const settled = await readChipSnapshots(page);
  const known = new Set<string>(INVENTORY_STATUSES);
  for (const chip of settled) {
    expect(
      known.has(chip.status),
      `chip "${chip.key}" carries a known data-status (got "${chip.status}", text "${chip.text}")`,
    ).toBe(true);
  }
});
