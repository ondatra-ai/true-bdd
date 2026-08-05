/**
 * goldens.update — AUTHORING-TIME capture of the committed design
 * gold-standard screenshots (tests/harness/goldens/<name>.png).
 *
 * The design-judge specs (w9/w11/w15) compare these committed goldens against
 * the live production render. The goldens are (re)captured from the runnable
 * prototype (harness/design/proto-workspace/app, booted via `next dev`) ONLY
 * when a test author deliberately runs `npm run goldens:update` — never
 * during a normal suite run, so a green judge always means "prod matches the
 * baseline the author signed off", not "prod matches whatever the prototype
 * renders today".
 *
 * Guard rails:
 *  - self-skips unless UPDATE_GOLDENS=1 (set only by the npm script), so a
 *    plain `playwright test` can never silently rewrite the gold standard;
 *  - each capture waits for the route's anchor testids + settled web fonts at
 *    the pinned 1440x900 reference viewport (design/SPEC.md §6);
 *  - a manifest.json records what was captured, from where, and when.
 */

import fs from "node:fs";
import path from "node:path";

import { expect, test } from "@playwright/test";

import { DESKTOP_VIEWPORT, GOLDENS_DIR, GOLDEN_NAMES } from "./helpers/design-conformance";
import { PROTO_BASELINE_ROUTES, bootPrototype, stopPrototype } from "./helpers/proto-baseline";

/** Every golden: its stable name, prototype route, and readiness anchors. */
const CAPTURES = [
  {
    name: GOLDEN_NAMES.workspaceOverview,
    route: PROTO_BASELINE_ROUTES.workspaceOverview,
    anchors: ["mockup-breadcrumb", "mockup-canvas"],
  },
  {
    name: GOLDEN_NAMES.productLanding,
    route: PROTO_BASELINE_ROUTES.productLanding,
    anchors: ["mockup-canvas", "yaml-editor"],
  },
  {
    name: GOLDEN_NAMES.storyPage,
    route: PROTO_BASELINE_ROUTES.story,
    anchors: ["mockup-canvas", "feature-strip", "yaml-editor"],
  },
  {
    name: GOLDEN_NAMES.featureDetail,
    route: PROTO_BASELINE_ROUTES.feature,
    anchors: ["mockup-canvas", "feature-stories-list", "feature-scenarios-list", "feature-unaligned-scenarios-list"],
  },
] as const;

test.describe("design goldens capture (authoring-time)", () => {
  test.skip(
    process.env.UPDATE_GOLDENS !== "1",
    "authoring-time only — run `npm run goldens:update` to (re)capture the design gold standards",
  );

  test.afterAll(async () => {
    await stopPrototype();
  });

  test("capture all design gold standards from the booted prototype", async ({ page }, testInfo) => {
    testInfo.setTimeout(10 * 60_000);

    const proto = await bootPrototype();
    await testInfo.attach("prototype-boot-log", { path: proto.logPath, contentType: "text/plain" });
    fs.mkdirSync(GOLDENS_DIR, { recursive: true });

    await page.setViewportSize({ ...DESKTOP_VIEWPORT });
    for (const capture of CAPTURES) {
      await page.goto(`${proto.baseURL}${capture.route}`, { waitUntil: "load" });
      for (const anchor of capture.anchors) {
        await expect(page.getByTestId(anchor), `${capture.name}: anchor ${anchor} must render`).toBeVisible();
      }
      // `font-display: swap` paints the fallback face first — settle the web
      // fonts so no golden ever freezes the wrong typography.
      await page.evaluate(async () => {
        await document.fonts.ready;
      });
      await page.screenshot({ path: path.join(GOLDENS_DIR, `${capture.name}.png`) });
    }

    // ── Per-component crops (w17 pixel-parity gate) ──
    // Full pages are for the vision judge; the deterministic pixel gate needs
    // tight chrome crops. Captured on the /product page. `maskFromY` /
    // `maskRect` record content-varying regions in GOLDEN crop coordinates —
    // w17 zeroes those rects on BOTH sides before diffing, so fixture-content
    // differences (extra scenario row, different YAML) can never flake the
    // gate while all chrome geometry stays compared.
    const cropsDir = path.join(GOLDENS_DIR, "crops");
    fs.mkdirSync(cropsDir, { recursive: true });
    await page.goto(`${proto.baseURL}${PROTO_BASELINE_ROUTES.productLanding}`, { waitUntil: "load" });
    await expect(page.getByTestId("mockup-canvas")).toBeVisible();
    await page.evaluate(async () => {
      await document.fonts.ready;
    });

    const cardLocator = page.locator('xpath=//*[@data-testid="yaml-editor"]/..');
    const cropTargets = {
      rail: page.getByTestId("workspace-rail"),
      sidebar: page.getByTestId("mockup-sidebar"),
      breadcrumb: page.getByTestId("mockup-breadcrumb"),
      "file-card": cardLocator,
      "chat-toggle": page.getByTestId("chat-dock-toggle"),
    } as const;
    const cropMeta: Record<string, { width: number; height: number; masks: Array<{ x: number; y: number; w: number; h: number }> }> = {};
    for (const [name, locator] of Object.entries(cropTargets)) {
      await expect(locator, `golden crop source for "${name}" must render`).toBeVisible();
      await locator.screenshot({ path: path.join(cropsDir, `${name}.png`) });
      const box = (await locator.boundingBox())!;
      cropMeta[name] = { width: Math.round(box.width), height: Math.round(box.height), masks: [] };
    }

    // Sidebar mask: everything from the Scenarios group's list downward varies
    // with fixture content (row counts differ between the prototype's and the
    // e2e env's registries).
    const sidebarBox = (await cropTargets.sidebar.boundingBox())!;
    const scenariosBox = (await page.getByTestId("scenarios-list").boundingBox())!;
    cropMeta.sidebar.masks.push({
      x: 0,
      y: Math.max(0, Math.round(scenariosBox.y - sidebarBox.y)),
      w: cropMeta.sidebar.width,
      h: cropMeta.sidebar.height,
    });

    // File-card mask: the editor TEXT area (right of the gutter, below the
    // header bar) is fixture content; the header bar, borders, and gutter
    // column stay compared.
    const cardBox = (await cardLocator.boundingBox())!;
    const gutterBox = (await page.locator(".yaml-editor__gutter").boundingBox())!;
    const editorTop = Math.round(gutterBox.y - cardBox.y);
    const gutterRight = Math.round(gutterBox.x + gutterBox.width - cardBox.x);
    cropMeta["file-card"].masks.push({
      x: gutterRight,
      y: editorTop,
      w: cropMeta["file-card"].width,
      h: cropMeta["file-card"].height,
    });
    // The header's "N lines" counter is fixture content too (line counts
    // differ between the prototype's and the e2e env's YAML) — mask its
    // region (with air around it) or it alone can eat the diff budget.
    const linesBox = (await page.locator(".file-view__meta").boundingBox())!;
    cropMeta["file-card"].masks.push({
      x: Math.max(0, Math.round(linesBox.x - cardBox.x) - 12),
      y: Math.max(0, Math.round(linesBox.y - cardBox.y) - 6),
      w: Math.round(linesBox.width) + 24,
      h: Math.round(linesBox.height) + 12,
    });

    const manifest = {
      capturedAt: new Date().toISOString(),
      viewport: DESKTOP_VIEWPORT,
      source: "harness/design/proto-workspace/app (booted via `next dev` by helpers/proto-baseline.ts)",
      routes: Object.fromEntries(CAPTURES.map((capture) => [capture.name, capture.route])),
      crops: cropMeta,
    };
    fs.writeFileSync(path.join(GOLDENS_DIR, "manifest.json"), `${JSON.stringify(manifest, null, 2)}\n`);
  });
});
