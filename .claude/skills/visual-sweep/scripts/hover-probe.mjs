// Generic hover-jank probe: discover every interactive element on the page
// (semantic interactives + pointer-cursor roots — no project selectors),
// hover each one, and measure its own box + parent box + font props at rest
// vs hover vs after. Flags JIGGLE at >=0.5px movement or any font change.
// Finishes with slow real-user sweeps along each nav-like region.
//
//   VS_URL=http://... VS_OUT=tmp/out node hover-probe.mjs
import fs from "fs";
import path from "path";
import { probeEnv, launchProbe, sleep } from "./lib/discover.mjs";

const env = probeEnv();
const { page, consoleLog, finish } = await launchProbe("hover-probe", env);

await page.goto(env.url, { waitUntil: "domcontentloaded" });
await page.waitForLoadState("networkidle").catch(() => {});
await sleep(1500);

await page.evaluate(() => window.__vsHelpers.installShiftObserver());
const discovered = await page.evaluate((scope) => window.__vsHelpers.discoverInteractive(scope), env.scope);
if (discovered.scopeMissing) {
  console.error(`VS_SCOPE '${env.scope}' matched nothing`);
  process.exit(2);
}
await page.evaluate((scope) => window.__vsHelpers.installMutationObserver(scope), env.scope);
console.log(`targets=${discovered.targets.length}${discovered.capped ? ` (capped from ${discovered.total})` : ""}`);

const rest = { x: env.viewport.width / 2, y: env.viewport.height / 2 };
const delta = (a, b) => (a && b ? a.map((v, k) => Math.round((b[k] - v) * 10) / 10) : null);
const moved = (d) => !!d && d.some((v) => Math.abs(v) >= 0.5);

const findings = [];
for (const t of discovered.targets) {
  await page.mouse.move(rest.x, rest.y);
  await sleep(200);
  const restM = await page.evaluate((i) => window.__vsHelpers.measureTarget(i), t.i);
  if (!restM) {
    findings.push({ sig: t.sig, text: t.text, gone: true });
    continue;
  }
  // Aim from the fresh rest box, not discovery-time coords — an earlier hover
  // may have shifted layout.
  const [rx, ry, rw, rh] = restM.self;
  await page.mouse.move(rx + Math.min(rw / 2, 80), ry + rh / 2);
  await sleep(300);
  const hoverM = await page.evaluate((i) => window.__vsHelpers.measureTarget(i), t.i);
  await page.mouse.move(rest.x, rest.y);
  await sleep(200);
  const afterM = await page.evaluate((i) => window.__vsHelpers.measureTarget(i), t.i);
  const rec = {
    sig: t.sig,
    text: t.text,
    dSelfHover: delta(restM.self, hoverM?.self),
    dSelfAfter: delta(restM.self, afterM?.self),
    dParentHover: delta(restM.parent, hoverM?.parent),
    fontRest: restM.font,
    fontHover: hoverM?.font,
  };
  rec.JIGGLE = moved(rec.dSelfHover) || moved(rec.dSelfAfter) || moved(rec.dParentHover) || rec.fontRest !== rec.fontHover;
  findings.push(rec);
  if (rec.JIGGLE) console.log("JIGGLE " + JSON.stringify(rec));
}

// Slow real-user sweeps along each discovered region (for the video + shift
// telemetry; per-element geometry was covered above).
const regions = await page.evaluate(() => window.__vsHelpers.discoverRegions());
for (const r of regions) {
  if (r.axis === "vertical") {
    const cx = r.x + r.w / 2;
    for (let y = r.y + 10; y < r.y + Math.min(r.h - 10, 600); y += 8) {
      await page.mouse.move(cx, y);
      await sleep(40);
    }
  } else {
    const cy = r.y + r.h / 2;
    for (let x = r.x + 10; x < r.x + Math.min(r.w - 10, 900); x += 12) {
      await page.mouse.move(x, cy);
      await sleep(40);
    }
  }
  await page.mouse.move(rest.x, rest.y);
  await sleep(300);
}
await sleep(400);

const ls = await page.evaluate(() => window.__vsLsFlush());
const mut = await page.evaluate(() => window.__vsMut || []);
fs.writeFileSync(
  path.join(env.out, "hover-findings.json"),
  JSON.stringify({ url: env.url, scope: env.scope || null, discovered: { count: findings.length, capped: discovered.capped, total: discovered.total }, findings, regions, ls, mut, console: consoleLog }, null, 1)
);

const jiggles = findings.filter((f) => f.JIGGLE);
const nonInputShifts = ls.filter((e) => !e.hadRecentInput);
const errors = consoleLog.filter((c) => c.type === "error" || c.type === "pageerror");
console.log(`SUMMARY targets=${findings.length} jiggles=${jiggles.length} shifts=${ls.length} nonInputShifts=${nonInputShifts.length} mutations=${mut.length} consoleErrors=${errors.length}`);
await finish();
console.log("DONE");
