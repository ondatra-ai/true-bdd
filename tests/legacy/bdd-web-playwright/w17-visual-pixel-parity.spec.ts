/**
 * w17 — deterministic PIXEL parity of the workspace chrome against the
 * committed golden component crops (tests/bdd-web/goldens/crops/*.png).
 *
 * Why this exists (tmp/design-compare/what-was-not-fixed.md + the
 * best-practices research): the w15 vision judge reads full-page screenshots
 * through an enumerated rubric, so sub-rubric geometry (rail density, band
 * insets, border weights, indents, paddings) sails through; and w16 asserts
 * only the properties someone thought to enumerate. A masked pixel diff of
 * tight chrome crops catches the ENTIRE class without enumerating it —
 * the industry-standard baseline mechanism (capture golden → diff every
 * render), applied per-component so fixture-content differences can be
 * masked away instead of flaking the gate.
 *
 * Mechanics per component:
 *   1. dimension parity (|Δw|/|Δh| within a small tolerance);
 *   2. min-crop both images to the shared region (top-left anchored);
 *   3. zero the manifest's content masks on BOTH sides (golden coordinates —
 *      recorded at capture time; e.g. the sidebar's scenario rows, the file
 *      card's editor text area);
 *   4. pixelmatch (threshold 0.15) over the remaining chrome pixels; fail
 *      when more than MAX_DIFF_RATIO of them differ. Golden, actual, and a
 *      highlighted diff are attached on every failure.
 *
 * Soft assertions (Playwright best practice) — one run reports EVERY
 * component's verdict, not just the first failure.
 *
 * Caveat (industry-standard for visual regression): goldens are rendered by
 * the same Chromium/OS the suite runs on; recapture via `npm run
 * goldens:update` when the rendering environment changes.
 *
 * RED intent: current prod diverges on rail density (53px vs 76px tiles),
 * sidebar band insets/indents/label colons, file-card gutter width + border
 * weight, and the chat tab width — each surfaces as a dimension or
 * diff-ratio failure with artifacts attached.
 */

import fs from "node:fs";

import pixelmatch from "pixelmatch";
import { PNG } from "pngjs";
import { expect, test, type Page } from "@playwright/test";

import {
  DESKTOP_VIEWPORT,
  requireGoldenCrop,
  requireGoldenCropMeta,
  type GoldenCropMask,
} from "./helpers/design-conformance";
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

/** Default max fraction of compared (unmasked) chrome pixels allowed to
 * differ. Text-DENSE components carry a higher, documented allowance: between
 * two independently-authored DOM trees, fractional line-height rounding
 * accumulates differently row by row (verified in the diffs: ±1px ALTERNATING
 * stripes — row N pixel-identical, row N+1 ghosted — which no registration
 * offset, CSS rule, or rendering mode can align; the floor held at ~1.7%
 * across dev-mode goldens, build-mode goldens, and three fix rounds). Every
 * REAL defect this gate caught measured 3.96–8.88%, so 2% keeps >2× headroom
 * between the noise ceiling and the defect floor. */
const MAX_DIFF_RATIO = 0.01;
const TEXT_DENSE_DIFF_RATIO = 0.02;
/** Anti-aliasing tolerance for pixelmatch (0..1). */
const PIXEL_THRESHOLD = 0.15;

interface ComponentCase {
  /** Golden crop name under goldens/crops/. */
  golden: string;
  /** Production locator for the SAME component. */
  prodSelector: string;
  /** Human label for failure messages. */
  label: string;
  /** Dimension tolerance, px. Content-driven axes get more slack. */
  dimTol: { w: number; h: number };
  /** Per-component diff cap (see the ratio constants above). */
  maxDiffRatio: number;
}

const COMPONENTS: ComponentCase[] = [
  {
    golden: "rail",
    prodSelector: `[data-testid="${WTID.rail}"]`,
    label: "icon rail",
    dimTol: { w: 2, h: 2 },
    maxDiffRatio: MAX_DIFF_RATIO,
  },
  {
    golden: "sidebar",
    prodSelector: `[data-testid="${WTID.sidebar}"]`,
    label: "workspace sidebar",
    dimTol: { w: 2, h: 2 },
    maxDiffRatio: TEXT_DENSE_DIFF_RATIO,
  },
  {
    golden: "breadcrumb",
    prodSelector: `[data-testid="${WTID.contentBreadcrumb}"]`,
    label: "breadcrumb bar",
    // Width varies with the chat tab's width (its own case pins that).
    dimTol: { w: 8, h: 2 },
    maxDiffRatio: MAX_DIFF_RATIO,
  },
  {
    golden: "file-card",
    prodSelector: `xpath=//*[@data-testid="${WTID.fileViewHeader}"]/..`,
    label: "file card",
    // Height follows fixture line count (one 20px line between the fixtures)
    // plus header rounding — content must not fail the DIMENSION check; the
    // masked pixel diff still gates the card's geometry. Width stays tight.
    dimTol: { w: 8, h: 48 },
    maxDiffRatio: TEXT_DENSE_DIFF_RATIO,
  },
  {
    golden: "chat-toggle",
    prodSelector: `[data-testid="${WTID.chatDockToggle}"]`,
    label: "chat dock toggle",
    dimTol: { w: 2, h: 2 },
    maxDiffRatio: MAX_DIFF_RATIO,
  },
];

/** Crop of a PNG at the given origin/dimensions (clipped by the caller). */
function cropPng(src: PNG, originX: number, originY: number, width: number, height: number): PNG {
  const out = new PNG({ width, height });
  PNG.bitblt(src, out, originX, originY, width, height, 0, 0);

  return out;
}

/** Zeroes the mask rects (clipped to the image) in-place and returns the
 * UNION of masked pixels — overlapping rects are counted once (round-2 fixer
 * escalation: the earlier `Math.max` under-counted multi-mask areas, quietly
 * inflating the compared denominator). */
function applyMasks(img: PNG, masks: GoldenCropMask[]): number {
  const flags = new Uint8Array(img.width * img.height);
  for (const mask of masks) {
    const x0 = Math.max(0, Math.floor(mask.x));
    const y0 = Math.max(0, Math.floor(mask.y));
    const x1 = Math.min(img.width, Math.ceil(mask.x + mask.w));
    const y1 = Math.min(img.height, Math.ceil(mask.y + mask.h));
    for (let y = y0; y < y1; y += 1) {
      for (let x = x0; x < x1; x += 1) {
        flags[img.width * y + x] = 1;
        const idx = (img.width * y + x) * 4;
        img.data[idx] = 0;
        img.data[idx + 1] = 0;
        img.data[idx + 2] = 0;
        img.data[idx + 3] = 255;
      }
    }
  }
  let masked = 0;
  for (const flag of flags) {
    masked += flag;
  }

  return masked;
}

/** The registration search radius, px. Two INDEPENDENT apps nest their
 * borders/boxes differently, so element-crop origins can differ by a border's
 * worth without any visual difference (round-2 Codex STOP evidence: the
 * file-card residual was a whole-crop shift, not a CSS gap). A GLOBAL shift
 * within this radius is an artifact and forgiven by scoring the best-aligned
 * offset; INTERNAL misalignment still diffs at every offset and fails. */
const REGISTRATION_RADIUS = 2;

async function openProduct(page: Page, slug: string): Promise<void> {
  env = await WorkspaceEnv.start(slug);
  const e = env;
  await page.setViewportSize({ ...DESKTOP_VIEWPORT });
  await gotoWorkspace(page, e.baseURL, wsRoutes.product(e.sid));
  await expect(page.getByTestId(WTID.rail)).toBeVisible();
  await expect(page.getByTestId(WTID.sidebar)).toBeVisible();
  await expect(page.getByTestId(WTID.fileView)).toBeVisible();
  await page.evaluate(async () => {
    await document.fonts.ready;
  });
}

test("w17.1 every chrome component matches its golden crop pixel-for-pixel (masked content excluded)", async ({
  page,
}, testInfo) => {
  // Resolve every golden up-front — a missing crop is an authoring defect and
  // must fail the case BEFORE booting the env (clear, fast, named).
  const goldens = COMPONENTS.map((component) => ({
    component,
    pngPath: requireGoldenCrop(component.golden),
    meta: requireGoldenCropMeta(component.golden),
  }));

  await openProduct(page, "w17-pixel-parity");

  for (const { component, pngPath, meta } of goldens) {
    const golden = PNG.sync.read(fs.readFileSync(pngPath));
    const locator = page.locator(component.prodSelector);
    await expect(locator, `${component.label}: production component must render`).toBeVisible();
    const actual = PNG.sync.read(await locator.screenshot());

    // 1 — dimension parity. A size drift IS a design drift.
    expect
      .soft(
        Math.abs(actual.width - golden.width),
        `${component.label}: renders ${actual.width}px wide — the golden is ${golden.width}px (tolerance ${component.dimTol.w}px)`,
      )
      .toBeLessThanOrEqual(component.dimTol.w);
    expect
      .soft(
        Math.abs(actual.height - golden.height),
        `${component.label}: renders ${actual.height}px tall — the golden is ${golden.height}px (tolerance ${component.dimTol.h}px)`,
      )
      .toBeLessThanOrEqual(component.dimTol.h);

    // 2/3 — shared-region crop + content masks on BOTH sides, scored at the
    // best registration offset within ±REGISTRATION_RADIUS in EITHER
    // direction (global crop-origin shifts between the two independent apps
    // are artifacts, not deviations). Masks are recorded in golden-crop
    // coordinates and translated with the golden's origin so they cover the
    // same visual region in both aligned crops.
    const width = Math.min(actual.width, golden.width) - REGISTRATION_RADIUS;
    const height = Math.min(actual.height, golden.height) - REGISTRATION_RADIUS;
    let best: { ratio: number; diff: PNG; goldenCrop: PNG; actualCrop: PNG; rx: number; ry: number } | null = null;
    for (let rx = -REGISTRATION_RADIUS; rx <= REGISTRATION_RADIUS; rx += 1) {
      for (let ry = -REGISTRATION_RADIUS; ry <= REGISTRATION_RADIUS; ry += 1) {
        const goldenOriginX = Math.max(0, -rx);
        const goldenOriginY = Math.max(0, -ry);
        const goldenCrop = cropPng(golden, goldenOriginX, goldenOriginY, width, height);
        const actualCrop = cropPng(actual, Math.max(0, rx), Math.max(0, ry), width, height);
        const shiftedMasks = meta.masks.map((mask) => ({
          ...mask,
          x: mask.x - goldenOriginX,
          y: mask.y - goldenOriginY,
        }));
        const masked = applyMasks(goldenCrop, shiftedMasks);
        applyMasks(actualCrop, shiftedMasks);
        const diff = new PNG({ width, height });
        const differing = pixelmatch(goldenCrop.data, actualCrop.data, diff.data, width, height, {
          threshold: PIXEL_THRESHOLD,
        });
        const compared = width * height - masked;
        const ratio = compared > 0 ? differing / compared : 1;
        if (best === null || ratio < best.ratio) {
          best = { ratio, diff, goldenCrop, actualCrop, rx, ry };
        }
      }
    }
    const { ratio, diff, goldenCrop, actualCrop } = best!;

    const goldenOut = testInfo.outputPath(`${component.golden}-golden.png`);
    const actualOut = testInfo.outputPath(`${component.golden}-actual.png`);
    const diffOut = testInfo.outputPath(`${component.golden}-diff.png`);
    fs.writeFileSync(goldenOut, PNG.sync.write(goldenCrop));
    fs.writeFileSync(actualOut, PNG.sync.write(actualCrop));
    fs.writeFileSync(diffOut, PNG.sync.write(diff));
    await testInfo.attach(`${component.golden}-golden`, { path: goldenOut, contentType: "image/png" });
    await testInfo.attach(`${component.golden}-actual`, { path: actualOut, contentType: "image/png" });
    await testInfo.attach(`${component.golden}-diff`, { path: diffOut, contentType: "image/png" });

    expect
      .soft(
        ratio,
        `${component.label}: ${(ratio * 100).toFixed(2)}% of compared chrome pixels differ from the golden ` +
          `(cap ${(component.maxDiffRatio * 100).toFixed(1)}%) — inspect ${component.golden}-diff.png in the attachments`,
      )
      .toBeLessThanOrEqual(component.maxDiffRatio);
  }
});
