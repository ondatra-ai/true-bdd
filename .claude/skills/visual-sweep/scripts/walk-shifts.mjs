// Generic cold-load + navigation walk: buffered layout-shift observer from
// navigation start (catches font-swap reflow), a 250ms box sampler over
// auto-picked chrome landmarks, a same-origin route cycle harvested from the
// DOM (x2 + rapid no-dwell flips), random cursor walks, and a final dwell.
// Telemetry is drained node-side at every step, so SPA and hard navigations
// both keep their data.
//
//   VS_URL=http://... VS_OUT=tmp/out node walk-shifts.mjs
import fs from "fs";
import path from "path";
import { probeEnv, launchProbe, sleep } from "./lib/discover.mjs";

const env = probeEnv();
const { page, consoleLog, finish } = await launchProbe("walk-shifts", env);
// Second init script (runs after the helper bundle): observe from nav start.
await page.addInitScript({ content: "window.__vsHelpers.installShiftObserver();" });
fs.mkdirSync(path.join(env.out, "shots"), { recursive: true });

const allShifts = [];
const allBoxes = [];
const steps = [];
let shotN = 0;

async function collect(label) {
  const url = page.url();
  const shifts = await page.evaluate(() => window.__vsLsFlush());
  const boxes = await page.evaluate(() => window.__vsHelpers.drainBoxes());
  for (const s of shifts) allShifts.push({ step: label, url, ...s });
  for (const b of boxes) allBoxes.push({ step: label, url, ...b });
  // A hard navigation kills the sampler; reinstall on the new document.
  const alive = await page.evaluate(() => !!window.__vsBoxTimer);
  if (!alive) await page.evaluate(() => window.__vsHelpers.installBoxSampler(250));
}

async function mark(label) {
  steps.push({ label, t: Date.now(), url: page.url() });
  shotN++;
  await page.screenshot({ path: path.join(env.out, "shots", `${String(shotN).padStart(2, "0")}-${label}.png`) }).catch(() => {});
  console.log(`STEP ${label}`);
}

// 1. Cold load + settle across the font-swap window (~first 3s).
await page.goto(env.url, { waitUntil: "domcontentloaded" });
await page.waitForLoadState("networkidle").catch(() => {});
await mark("loaded");
await sleep(3500);
await page.evaluate(() => window.__vsHelpers.installBoxSampler(250));
await mark("settled");
await collect("cold-load");

// 2. Harvest same-origin routes from the DOM.
const routes = await page.evaluate(() => {
  const here = location.pathname + location.search;
  const seen = new Set([here]);
  const out = [];
  for (const a of document.querySelectorAll('a[href],[role="link"][href]')) {
    const u = new URL(a.getAttribute("href"), location.href);
    if (u.origin !== location.origin) continue;
    const key = u.pathname + u.search;
    if (seen.has(key)) continue;
    seen.add(key);
    out.push({ href: u.href, key, sig: window.__vsHelpers.sigOf(a), text: (a.textContent || "").trim().slice(0, 30) });
  }
  return out.slice(0, 8);
});
console.log(`routes=${routes.length} ${routes.map((r) => r.key).join(" ")}`);

async function visit(route, dwellMs) {
  // Prefer clicking the real link (exercises SPA routing); fall back to goto.
  const link = page.locator(`a[href="${route.key}"], a[href="${route.href}"]`).first();
  if (await link.isVisible().catch(() => false)) await link.click();
  else await page.goto(route.href, { waitUntil: "domcontentloaded" });
  await page.waitForLoadState("domcontentloaded").catch(() => {});
  await sleep(dwellMs);
}

// 3. Route cycle x2 with dwell, then rapid no-dwell flips.
const entry = { href: env.url, key: new URL(env.url).pathname + new URL(env.url).search };
for (let cycle = 0; cycle < 2 && routes.length; cycle++) {
  for (const route of routes) {
    await visit(route, 1200);
    await mark(`cycle${cycle + 1}-${route.key.replaceAll("/", "_").slice(0, 40)}`);
    await collect(`cycle${cycle + 1}:${route.key}`);
  }
  await visit(entry, 1200);
  await collect(`cycle${cycle + 1}:return`);
}
for (const route of routes) await visit(route, 150);
if (routes.length) {
  await visit(entry, 1200);
  await mark("rapid-flips-done");
  await collect("rapid-flips");
}

// 4. Random cursor walks (junk input a real user produces).
for (let walk = 0; walk < 2; walk++) {
  for (let i = 0; i < 25; i++) {
    await page.mouse.move(Math.random() * env.viewport.width, Math.random() * env.viewport.height);
    await sleep(30 + Math.random() * 30);
  }
}
await collect("random-walks");

// 5. Final dwell — idle churn (polling, animations) shows up here.
await sleep(5000);
await mark("final-dwell");
await collect("final-dwell");

// Box-move analysis: distinct landmark positions across consecutive samples
// within the same step+url (cross-navigation jumps are expected, not jank).
const moves = [];
const bySeq = new Map();
for (const rec of allBoxes) {
  const scope = `${rec.step}|${rec.url}`;
  const prev = bySeq.get(scope) || {};
  for (const [name, b] of Object.entries(rec.boxes)) {
    const p = prev[name];
    if (p && b.some((v, i) => Math.abs(v - p[i]) >= 0.5)) {
      moves.push({ step: rec.step, name, from: p, to: b, t: rec.t });
    }
  }
  bySeq.set(scope, { ...prev, ...rec.boxes });
}

fs.writeFileSync(path.join(env.out, "layout-shifts.json"), JSON.stringify(allShifts, null, 1));
fs.writeFileSync(path.join(env.out, "boxes.json"), JSON.stringify({ samples: allBoxes.length, moves }, null, 1));
fs.writeFileSync(path.join(env.out, "steps.json"), JSON.stringify(steps, null, 1));
fs.writeFileSync(path.join(env.out, "console.json"), JSON.stringify(consoleLog, null, 1));

const nonInput = allShifts.filter((s) => !s.hadRecentInput);
const cls = Math.round(nonInput.reduce((a, s) => a + s.value, 0) * 1000) / 1000;
const errors = consoleLog.filter((c) => c.type === "error" || c.type === "pageerror");
console.log(`SUMMARY shifts=${allShifts.length} nonInputShifts=${nonInput.length} totalCLS=${cls} boxMoves=${moves.length} routes=${routes.length} consoleErrors=${errors.length}`);
for (const s of [...nonInput].sort((a, b) => b.value - a.value).slice(0, 5)) {
  console.log(`SHIFT step=${s.step} t=${s.t} value=${s.value} sources=${s.sources.map((x) => `${x.node} y${x.prev?.y}->${x.curr?.y}`).join(" ")}`);
}
await finish();
console.log("DONE");
