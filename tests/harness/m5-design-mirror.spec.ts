/**
 * m5 — the S&F design-system mirror + design spec + mockups are real, offline
 * deliverables. Pure Node `fs` (no browser).
 *
 * Path resolution (round 2, finding B3): all roots derive from the helper's own
 * `__dirname` (systemRoot / designRoot / mockupsRoot / specPath), never
 * process.cwd() — the run command executes from `tests/harness/`.
 *
 * Expected split at RED time: m5.1–m5.4 exercise the orchestrator-populated
 * `system/` mirror (already present + contract-validated) and PASS; m5.5 (SPEC.md)
 * and m5.6 (mockups scan) FAIL because those coder deliverables do not exist yet.
 */

import { expect, test } from "@playwright/test";
import fs from "node:fs";
import path from "node:path";

import { PAGES, cssUrls, filesByExt, mockupsRoot, specPath, systemRoot } from "./helpers/mockups";

test.describe("m5 design mirror", () => {
  // ── m5.1 — contract files present + non-empty ──
  test("m5.1 mirror contract files present + non-empty", () => {
    const tokens = path.join(systemRoot, "tokens.css");
    const sync = path.join(systemRoot, "SYNC.md");
    for (const file of [tokens, sync]) {
      expect(fs.existsSync(file), `${file} exists`).toBe(true);
      expect(fs.statSync(file).size, `${file} non-empty`).toBeGreaterThan(0);
    }
    for (const dir of ["fonts", "components", "guidelines"]) {
      const full = path.join(systemRoot, dir);
      expect(fs.existsSync(full), `${dir}/ exists`).toBe(true);
      expect(fs.statSync(full).isDirectory(), `${dir}/ is a directory`).toBe(true);
      expect(fs.readdirSync(full).length, `${dir}/ non-empty`).toBeGreaterThan(0);
    }
  });

  // ── m5.2 — provenance recorded ──
  test("m5.2 SYNC.md records the exact project id + an ISO-8601 sync date", () => {
    const sync = fs.readFileSync(path.join(systemRoot, "SYNC.md"), "utf8");
    expect(sync).toContain("147f5da0-fedd-4aaa-b0c2-ccb9f7d7b41e");
    expect(sync, "an ISO-8601 sync date").toMatch(/sync date:\s*\d{4}-\d{2}-\d{2}/i);
  });

  // ── m5.3 — works offline: no remote deps in any CSS under system/ ──
  test("m5.3 no CSS under system/ references remote url()/@import", () => {
    const cssFiles = filesByExt(systemRoot, [".css"]);
    expect(cssFiles.length, "at least one CSS file under system/").toBeGreaterThan(0);
    for (const file of cssFiles) {
      const css = fs.readFileSync(file, "utf8");
      for (const url of cssUrls(css)) {
        expect(/^https?:/i.test(url), `${file} references remote url(${url})`).toBe(false);
      }
      expect(css, `${file} has a remote @import`).not.toMatch(
        /@import\s+(url\()?\s*['"]?https?:/i,
      );
    }
  });

  // ── m5.4 — referenced local fonts/assets exist (recursive) ──
  test("m5.4 every local url() under system/ resolves to a file on disk", () => {
    const cssFiles = filesByExt(systemRoot, [".css"]);
    for (const file of cssFiles) {
      const css = fs.readFileSync(file, "utf8");
      for (const url of cssUrls(css)) {
        if (/^(https?:|data:)/i.test(url)) {
          continue;
        }
        const clean = url.split(/[?#]/)[0];
        const resolved = path.resolve(path.dirname(file), clean);
        expect(fs.existsSync(resolved), `${file} → missing local asset url(${url})`).toBe(true);
      }
    }
  });

  // ── m5.5 — the design spec exists and covers its required topics ──
  test("m5.5 SPEC.md exists, non-empty, and covers the seven required topics", () => {
    expect(fs.existsSync(specPath), "harness/design/SPEC.md exists").toBe(true);
    const spec = fs.readFileSync(specPath, "utf8");
    expect(spec.trim().length, "SPEC.md non-empty").toBeGreaterThan(0);

    // Topic 1 — the layout frame: sidebar + breadcrumb + canvas.
    expect(/sidebar/i.test(spec), 'SPEC.md covers "layout frame: sidebar"').toBe(true);
    expect(/breadcrumb/i.test(spec), 'SPEC.md covers "layout frame: breadcrumb"').toBe(true);
    expect(/canvas/i.test(spec), 'SPEC.md covers "layout frame: canvas"').toBe(true);
    // Topics 2–7 (keyword presence).
    const topics: [string, RegExp][] = [
      ["sidebar top-level ordering", /order/i],
      ["token→UI mapping", /token/i],
      ["page inventory", /inventory|pages?\b/i],
      ["degraded-state catalog", /degraded|truncat|unavailable|flag/i],
      ["1440px reference", /1440/],
      ["data provenance", /provenance|fixture|source|scenarios\.yaml/i],
    ];
    for (const [name, re] of topics) {
      expect(re.test(spec), `SPEC.md covers "${name}"`).toBe(true);
    }

    // Concrete completeness so a one-sentence placeholder cannot pass (round 2,
    // finding 6) — but still content presence, not exact formatting:
    const lower = spec.toLowerCase();
    // (a) the page inventory enumerates ALL 17 mockup filenames.
    for (const name of PAGES) {
      expect(lower, `SPEC.md page inventory lists ${name}.html`).toContain(`${name}.html`);
    }
    // (b) the six sidebar top-level labels are all named.
    for (const label of ["workspace overview", "architecture", "product", "requirements", "runs", "sessions"]) {
      expect(lower, `SPEC.md names sidebar entry "${label}"`).toContain(label);
    }
    // (c) the degraded-state catalog names the specific states from the plan.
    for (const state of ["truncat", "cli_timeout", "path", "ambiguous", "invalid", "identity"]) {
      expect(lower, `SPEC.md degraded catalog mentions "${state}"`).toContain(state);
    }
    // (d) the token→UI mapping pairs tokens with concrete components.
    const components = ["tree", "pill", "chip", "table", "dialog"].filter((c) => lower.includes(c));
    expect(components.length, "SPEC.md token→UI mapping names ≥2 components").toBeGreaterThanOrEqual(2);
  });

  // ── m5.6 — the mockups reference nothing remote (static self-contained scan) ──
  test("m5.6 mockup HTML/CSS/JS reference nothing remote", () => {
    expect(fs.existsSync(mockupsRoot), "harness/design/mockups/ exists").toBe(true);
    const files = filesByExt(mockupsRoot, [".html", ".css", ".js"]);
    expect(files.length, "mockups directory contains html/css/js files").toBeGreaterThan(0);

    // Catch explicit http(s) anywhere AND protocol-relative (`//host`) refs
    // (rounds 2/3, findings 5 & 3). JS-assembled URLs can't be caught statically —
    // the m1.6(d) render-time proof covers those. Protocol-relative patterns are
    // scoped by file type so a `// comment` in JS is never a false positive; the
    // HTML-attribute form allows the value to be quoted OR unquoted.
    const REMOTE_SCHEME = /https?:\/\/[^\s"'()]+/gi; // any file
    const htmlPatterns: RegExp[] = [
      /(?:href|src|srcset)\s*=\s*(?:["']\s*)?\/\//gi, // protocol-relative attribute (quoted or not)
    ];
    const cssPatterns: RegExp[] = [
      /url\(\s*["']?\s*\/\//gi, // protocol-relative CSS url()
      /@import\s+(?:url\(\s*)?["']?\s*\/\//gi, // protocol-relative CSS @import
    ];
    const offenders: string[] = [];
    for (const file of files) {
      const ext = path.extname(file).toLowerCase();
      const src = fs.readFileSync(file, "utf8");
      const active = [REMOTE_SCHEME, ...(ext === ".html" ? htmlPatterns : []), ...(ext === ".css" ? cssPatterns : [])];
      const hits: string[] = [];
      for (const re of active) {
        for (const m of src.matchAll(re)) {
          hits.push(m[0]);
        }
      }
      if (hits.length > 0) {
        offenders.push(`${path.basename(file)}: ${hits.slice(0, 3).join(", ")}`);
      }
    }
    expect(offenders, `remote references found:\n${offenders.join("\n")}`).toEqual([]);
  });
});
