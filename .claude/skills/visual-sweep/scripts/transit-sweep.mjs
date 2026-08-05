// Generic transit probe: sweep the cursor across every nav-like region the
// way a user mouses past it (fast vertical/horizontal passes + a diagonal
// entry), sampling overlay state every 25ms. Any overlay that appears during
// pure transit is counted; an appearance with dwell < 150ms is a FLASH.
// Regions and overlays are discovered by geometry — no project selectors.
//
//   VS_URL=http://... VS_OUT=tmp/out node transit-sweep.mjs
import fs from "fs";
import path from "path";
import { probeEnv, launchProbe, sleep } from "./lib/discover.mjs";

const env = probeEnv();
const { page, consoleLog, finish } = await launchProbe("transit-sweep", env);

await page.goto(env.url, { waitUntil: "domcontentloaded" });
await page.waitForLoadState("networkidle").catch(() => {});
await sleep(1500);

const rest = { x: env.viewport.width / 2, y: env.viewport.height / 2 };
const regions = await page.evaluate(() => window.__vsHelpers.discoverRegions());
console.log(`regions=${regions.length} ${regions.map((r) => `${r.sig}(${r.axis})`).join(" ")}`);

// Turn 25ms samples into per-overlay episodes: open span -> {sig, ms}.
function analyze(samples) {
  const episodes = [];
  const openSince = new Map();
  let swaps = 0;
  let prevOpen = [];
  let lastT = samples.length ? samples[samples.length - 1].t : 0;
  for (const s of samples) {
    for (const sig of s.open) if (!openSince.has(sig)) openSince.set(sig, s.t);
    for (const [sig, since] of [...openSince]) {
      if (!s.open.includes(sig)) {
        episodes.push({ sig, ms: s.t - since });
        openSince.delete(sig);
      }
    }
    if (s.open.length && prevOpen.length && s.open.join("|") !== prevOpen.join("|")) swaps++;
    if (s.open.length) prevOpen = s.open;
  }
  for (const [sig, since] of openSince) episodes.push({ sig, ms: lastT - since, unclosed: true });
  const flashes = episodes.filter((e) => e.ms < 150);
  return {
    appearances: episodes.length,
    flashes: flashes.length,
    swaps,
    minDwellMs: episodes.length ? Math.min(...episodes.map((e) => e.ms)) : null,
    totalOpenMs: episodes.reduce((a, e) => a + e.ms, 0),
    episodes,
  };
}

const results = [];
for (const region of regions) {
  await page.mouse.move(rest.x, rest.y);
  await sleep(400);
  await page.evaluate(() => window.__vsHelpers.startOverlaySampler(25));

  if (region.axis === "vertical") {
    const cx = region.x + region.w / 2;
    const y0 = region.y + 10;
    const y1 = region.y + Math.min(region.h - 10, 800);
    for (let pass = 0; pass < 3; pass++) {
      for (let y = y0; y <= y1; y += 14) { await page.mouse.move(cx, y); await sleep(25); }
      await sleep(300);
      for (let y = y1; y >= y0; y -= 14) { await page.mouse.move(cx, y); await sleep(25); }
      await sleep(300);
    }
    // Diagonal transit from content across the region's edge.
    for (let i = 0; i <= 20; i++) {
      await page.mouse.move(rest.x + (cx - rest.x) * (i / 20), rest.y + (y1 - rest.y) * (i / 20));
      await sleep(20);
    }
  } else {
    const cy = region.y + region.h / 2;
    const x0 = region.x + 10;
    const x1 = region.x + Math.min(region.w - 10, 1200);
    for (let pass = 0; pass < 3; pass++) {
      for (let x = x0; x <= x1; x += 20) { await page.mouse.move(x, cy); await sleep(25); }
      await sleep(300);
      for (let x = x1; x >= x0; x -= 20) { await page.mouse.move(x, cy); await sleep(25); }
      await sleep(300);
    }
    for (let i = 0; i <= 20; i++) {
      await page.mouse.move(rest.x + (x1 - rest.x) * (i / 20), rest.y + (cy - rest.y) * (i / 20));
      await sleep(20);
    }
  }
  await sleep(500);
  await page.mouse.move(rest.x, rest.y);
  await sleep(500);

  const { baseline, samples } = await page.evaluate(() => window.__vsHelpers.stopOverlaySampler());
  const a = analyze(samples);
  results.push({ region, baseline, samples: samples.length, ...a });
  console.log(`region=${region.sig} appearances=${a.appearances} flashes=${a.flashes} swaps=${a.swaps} minDwellMs=${a.minDwellMs ?? "-"} openMs=${a.totalOpenMs}`);
}

fs.writeFileSync(path.join(env.out, "transit.json"), JSON.stringify({ url: env.url, regions, results, console: consoleLog }, null, 1));
const totalFlashes = results.reduce((a, r) => a + r.flashes, 0);
const totalAppearances = results.reduce((a, r) => a + r.appearances, 0);
const errors = consoleLog.filter((c) => c.type === "error" || c.type === "pageerror");
console.log(`SUMMARY regions=${results.length} appearances=${totalAppearances} flashes=${totalFlashes} consoleErrors=${errors.length}`);
await finish();
console.log("DONE");
