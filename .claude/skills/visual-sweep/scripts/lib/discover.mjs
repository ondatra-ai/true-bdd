// Shared runtime for the visual-sweep probes: env parsing, Playwright
// resolution, browser launch with video, and the browser-side helper bundle
// (generic DOM discovery + instrumentation oracles). No project knowledge —
// everything is discovered from the live DOM.
import { createRequire } from "module";
import fs from "fs";
import path from "path";

export const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

// Env interface shared by every probe:
//   VS_URL          (required) entry URL of the live app
//   VS_OUT          (required) artifact output dir (created if absent)
//   VS_SCOPE        (optional) CSS selector constraining interactive discovery
//   VS_VIEWPORT     (optional) "WxH", default 1440x900
//   VS_REQUIRE_FROM (optional) package.json anchor that resolves "playwright"
export function probeEnv() {
  const url = process.env.VS_URL;
  const out = process.env.VS_OUT;
  if (!url || !out) {
    console.error("VS_URL and VS_OUT are required (optional: VS_SCOPE, VS_VIEWPORT, VS_REQUIRE_FROM)");
    process.exit(2);
  }
  const [w, h] = (process.env.VS_VIEWPORT || "1440x900").split("x").map(Number);
  fs.mkdirSync(out, { recursive: true });
  return { url, out, scope: process.env.VS_SCOPE || "", viewport: { width: w || 1440, height: h || 900 } };
}

export function loadPlaywright() {
  const candidates = [
    process.env.VS_REQUIRE_FROM,
    path.join(process.cwd(), "package.json"),
    path.join(process.cwd(), "tests", "harness", "package.json"),
  ].filter(Boolean);
  for (const anchor of candidates) {
    try {
      return createRequire(path.resolve(anchor))("playwright");
    } catch {
      // try the next anchor
    }
  }
  console.error("Cannot resolve 'playwright'. Set VS_REQUIRE_FROM to a package.json whose tree has playwright installed.");
  process.exit(2);
}

// Launch a headless page with video + console capture and the helper bundle
// pre-installed on every document. finish() closes everything and copies the
// video to <out>/<name>.webm.
export async function launchProbe(name, env) {
  const { chromium } = loadPlaywright();
  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext({
    viewport: env.viewport,
    recordVideo: { dir: path.join(env.out, `video-${name}`), size: env.viewport },
  });
  const page = await context.newPage();
  const consoleLog = [];
  page.on("console", (m) => consoleLog.push({ type: m.type(), text: m.text().slice(0, 300) }));
  page.on("pageerror", (e) => consoleLog.push({ type: "pageerror", text: String(e).slice(0, 300) }));
  await page.addInitScript({ content: VS_HELPERS_SRC });
  const finish = async () => {
    const video = page.video();
    await context.close();
    if (video) {
      try {
        fs.copyFileSync(await video.path(), path.join(env.out, `${name}.webm`));
      } catch {
        // video may be absent when the page never painted; the JSON still lands
      }
    }
    await browser.close();
    return consoleLog;
  };
  return { browser, context, page, consoleLog, finish };
}

// Browser-side helper bundle installed via addInitScript on every document.
// Deliberately no template literals inside (this whole bundle is one).
export const VS_HELPERS_SRC = `
window.__vsHelpers = (() => {
  const sigOf = (node) => {
    const el = node && node.nodeType === 1 ? node : node && node.parentElement;
    if (!el) return String((node && node.nodeName) || "?");
    const tid = el.getAttribute && el.getAttribute("data-testid");
    if (tid) return "[" + tid + "]";
    const id = el.id ? "#" + el.id : "";
    const cls = typeof el.className === "string" && el.className.trim()
      ? "." + el.className.trim().split(/\\s+/)[0]
      : "";
    let nth = "";
    if (!id && el.parentElement) {
      const same = Array.from(el.parentElement.children).filter((c) => c.tagName === el.tagName);
      if (same.length > 1) nth = ":nth(" + same.indexOf(el) + ")";
    }
    return el.tagName.toLowerCase() + id + cls + nth;
  };

  const visible = (el) => {
    if (!el || el.nodeType !== 1) return false;
    const r = el.getBoundingClientRect();
    if (r.width < 8 || r.height < 8) return false;
    if (r.bottom <= 0 || r.right <= 0 || r.top >= innerHeight || r.left >= innerWidth) return false;
    const s = getComputedStyle(el);
    return s.visibility !== "hidden" && s.display !== "none" && Number(s.opacity || 1) > 0.05;
  };

  const round1 = (v) => Math.round(v * 10) / 10;
  const box = (el) => {
    const r = el.getBoundingClientRect();
    return [r.x, r.y, r.width, r.height].map(round1);
  };

  // Base selectors of every :hover rule in same-origin stylesheets — elements
  // styled to react to hover are hover targets even at cursor:auto (list rows,
  // cards). The base is the selector prefix up to the first :hover.
  const hoverRuleSelectors = () => {
    const out = new Set();
    for (const sheet of document.styleSheets) {
      let rules;
      try { rules = sheet.cssRules; } catch (e) { continue; }
      if (!rules) continue;
      const walk = (list) => {
        for (const r of list) {
          if (r.cssRules) { walk(r.cssRules); continue; }
          const sel = r.selectorText;
          if (!sel || sel.indexOf(":hover") === -1) continue;
          for (const part of sel.split(",")) {
            const at = part.indexOf(":hover");
            if (at === -1) continue;
            const base = part.slice(0, at).trim();
            if (base) out.add(base);
          }
        }
      };
      walk(rules);
    }
    return Array.from(out).slice(0, 300);
  };

  // Everything a user might hover/click: semantic interactives, "pointer
  // roots" (computed cursor:pointer whose parent isn't — cursor inherits, so
  // counting descendants would explode the list), and :hover-rule targets.
  // Tags each kept element with data-vs-i so measureTarget can re-locate it.
  const discoverInteractive = (scopeSelector, cap) => {
    cap = cap || 200;
    const scope = scopeSelector ? document.querySelector(scopeSelector) : document.body;
    if (!scope) return { targets: [], capped: false, scopeMissing: true };
    const sel = 'a,button,[role="button"],[role="tab"],[role="menuitem"],[role="link"],[role="option"],[tabindex],input,select,textarea,summary,label';
    const set = new Set(Array.from(scope.querySelectorAll(sel)));
    for (const el of scope.querySelectorAll("*")) {
      if (getComputedStyle(el).cursor !== "pointer") continue;
      const p = el.parentElement;
      if (p && getComputedStyle(p).cursor === "pointer") continue;
      set.add(el);
    }
    for (const base of hoverRuleSelectors()) {
      let matched;
      try { matched = scope.querySelectorAll(base); } catch (e) { continue; }
      for (const el of matched) set.add(el);
    }
    const seen = new Set();
    const items = [];
    for (const el of set) {
      if (!visible(el)) continue;
      const b = box(el);
      const sig = sigOf(el);
      const key = sig + "@" + Math.round(b[0]) + "," + Math.round(b[1]);
      if (seen.has(key)) continue;
      seen.add(key);
      items.push({ el, sig, b });
    }
    items.sort((a, z) => a.b[1] - z.b[1] || a.b[0] - z.b[0]);
    const kept = items.slice(0, cap);
    kept.forEach((it, i) => it.el.setAttribute("data-vs-i", String(i)));
    return {
      targets: kept.map((it, i) => ({
        i: i, sig: it.sig, x: it.b[0], y: it.b[1], w: it.b[2], h: it.b[3],
        text: (it.el.textContent || "").trim().slice(0, 30),
      })),
      capped: items.length > cap,
      total: items.length,
    };
  };

  // Geometry of a tagged target: own box, parent box, and the font props that
  // betray hover bold/spacing swaps.
  const measureTarget = (i) => {
    const el = document.querySelector('[data-vs-i="' + i + '"]');
    if (!el) return null;
    const s = getComputedStyle(el);
    return {
      self: box(el),
      parent: el.parentElement ? box(el.parentElement) : null,
      font: s.fontWeight + "/" + s.letterSpacing,
    };
  };

  // Nav-like chrome detected purely by geometry: tall/narrow containers hugging
  // a vertical viewport edge (rails, sidebars) or wide/short ones hugging a
  // horizontal edge (toolbars, headers). Keeps the innermost of nested matches.
  const regionCandidates = () => {
    const vw = innerWidth, vh = innerHeight;
    const cands = [];
    for (const el of document.body.querySelectorAll("*")) {
      if (!visible(el)) continue;
      const r = el.getBoundingClientRect();
      if (r.width < 24 || r.height < 24) continue;
      const tallNarrow = r.height >= vh * 0.5 && r.width <= vw * 0.25 && (r.left <= 16 || r.right >= vw - 16);
      const wideShort = r.width >= vw * 0.5 && r.height <= vh * 0.15 && (r.top <= 16 || r.bottom >= vh - 16);
      if (!tallNarrow && !wideShort) continue;
      cands.push({ el, sig: sigOf(el), x: round1(r.x), y: round1(r.y), w: round1(r.width), h: round1(r.height),
        axis: tallNarrow ? "vertical" : "horizontal" });
    }
    const contains = (a, b) =>
      a.x <= b.x + 1 && a.y <= b.y + 1 && a.x + a.w >= b.x + b.w - 1 && a.y + a.h >= b.y + b.h - 1 &&
      a.w * a.h > b.w * b.h * 1.02;
    const kept = cands.filter((a) => !cands.some((b) => b !== a && a.axis === b.axis && contains(a, b)));
    const out = [];
    for (const c of kept) {
      if (out.some((o) => Math.abs(o.x - c.x) < 8 && Math.abs(o.y - c.y) < 8 && Math.abs(o.w - c.w) < 8 && Math.abs(o.h - c.h) < 8)) continue;
      out.push(c);
    }
    return out.slice(0, 6);
  };
  const discoverRegions = () => regionCandidates().map((c) => ({
    sig: c.sig, x: c.x, y: c.y, w: c.w, h: c.h, axis: c.axis,
  }));

  // Overlay roots currently visible: positioned, stacked, big enough to be a
  // panel/flyout/tooltip — with no qualifying positioned ancestor (roots only).
  const overlaysNow = () => {
    const out = [];
    for (const el of document.body.querySelectorAll("*")) {
      const s = getComputedStyle(el);
      if (s.position !== "fixed" && s.position !== "absolute") continue;
      if (!(Number(s.zIndex) > 0)) continue;
      if (!visible(el)) continue;
      const r = el.getBoundingClientRect();
      if (r.width < 40 || r.height < 40) continue;
      let anc = el.parentElement, nested = false;
      while (anc && anc !== document.body) {
        const as = getComputedStyle(anc);
        if ((as.position === "fixed" || as.position === "absolute") && Number(as.zIndex) > 0) { nested = true; break; }
        anc = anc.parentElement;
      }
      if (nested) continue;
      out.push(sigOf(el));
    }
    return out;
  };

  // Buffered layout-shift observer with per-element attribution. No swallowing
  // try/catch — a silently dead observer would false-green everything. Flush
  // DRAINS: callers accumulate node-side, so SPA and hard navigations both work.
  const installShiftObserver = () => {
    window.__vsLs = [];
    const record = (e) => {
      window.__vsLs.push({
        t: Math.round(e.startTime),
        value: e.value,
        hadRecentInput: e.hadRecentInput,
        sources: (e.sources || []).map((src) => {
          let d = "gone";
          try { d = sigOf(src.node); } catch (err) { d = "gone"; }
          const r = (x) => x && { x: Math.round(x.x), y: Math.round(x.y), w: Math.round(x.width), h: Math.round(x.height) };
          return { node: d, prev: r(src.previousRect), curr: r(src.currentRect) };
        }),
      });
    };
    const obs = new PerformanceObserver((list) => {
      for (const e of list.getEntries()) record(e);
    });
    obs.observe({ type: "layout-shift", buffered: true });
    window.__vsLsReady = true;
    window.__vsLsFlush = () => {
      for (const e of obs.takeRecords()) record(e);
      const out = window.__vsLs;
      window.__vsLs = [];
      return out;
    };
  };

  // 250ms box sampler over auto-picked chrome landmarks (regions + semantic
  // landmarks + large layout children). Catches jiggle below CLS's radar.
  const installBoxSampler = (ms) => {
    ms = ms || 250;
    window.__vsBoxes = [];
    const els = new Set();
    for (const c of regionCandidates()) els.add(c.el);
    for (const el of document.querySelectorAll("header,nav,aside,footer,main")) if (visible(el)) els.add(el);
    for (const el of document.body.querySelectorAll(":scope > *, :scope > * > *")) {
      if (!visible(el)) continue;
      const r = el.getBoundingClientRect();
      if (r.width * r.height >= innerWidth * innerHeight * 0.08) els.add(el);
    }
    const list = Array.from(els).slice(0, 14);
    list.forEach((el, i) => el.setAttribute("data-vs-box", String(i)));
    const names = list.map((el) => sigOf(el));
    window.__vsBoxTimer = setInterval(() => {
      const rec = { t: Math.round(performance.now()), boxes: {} };
      names.forEach((name, i) => {
        const el = document.querySelector('[data-vs-box="' + i + '"]');
        if (el) rec.boxes[name] = box(el);
      });
      window.__vsBoxes.push(rec);
    }, ms);
    return names;
  };
  const drainBoxes = () => {
    const out = window.__vsBoxes || [];
    window.__vsBoxes = [];
    return out;
  };

  // Mutation observer scoped to a selector (default body). Ignores the probes'
  // own data-vs-* tagging.
  const installMutationObserver = (scopeSelector) => {
    window.__vsMut = [];
    const scope = scopeSelector ? document.querySelector(scopeSelector) : document.body;
    if (!scope) return false;
    new MutationObserver((muts) => {
      for (const m of muts) {
        if (m.type === "attributes" && m.attributeName && m.attributeName.indexOf("data-vs-") === 0) continue;
        window.__vsMut.push({
          t: Math.round(performance.now()),
          type: m.type,
          target: sigOf(m.target),
          added: Array.from(m.addedNodes).map(sigOf),
          removed: Array.from(m.removedNodes).map(sigOf),
          attr: m.attributeName || undefined,
        });
      }
    }).observe(scope, { subtree: true, childList: true, attributes: true });
    return true;
  };

  // 25ms overlay sampler for transit sweeps: records which overlay roots beyond
  // the at-rest baseline are visible at each tick.
  const startOverlaySampler = (ms) => {
    ms = ms || 25;
    window.__vsFly = [];
    window.__vsFlyBaseline = overlaysNow();
    const base = new Set(window.__vsFlyBaseline);
    window.__vsFlyTimer = setInterval(() => {
      window.__vsFly.push({
        t: Math.round(performance.now()),
        open: overlaysNow().filter((s) => !base.has(s)),
      });
    }, ms);
  };
  const stopOverlaySampler = () => {
    clearInterval(window.__vsFlyTimer);
    return { baseline: window.__vsFlyBaseline, samples: window.__vsFly };
  };

  return {
    sigOf, visible, hoverRuleSelectors,
    discoverInteractive, measureTarget, discoverRegions, overlaysNow,
    installShiftObserver, installBoxSampler, drainBoxes, installMutationObserver,
    startOverlaySampler, stopOverlaySampler,
  };
})();
`;
