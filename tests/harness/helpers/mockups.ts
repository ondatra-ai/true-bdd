/**
 * Mockup-only test helper for the ClickUp-style S&F workspace UI design task
 * (`harness-workspace-ui-design`).
 *
 * These mockups are STATIC local HTML pages under `harness/design/mockups/`,
 * opened over `file://` (no server, no build) so the specs directly verify the
 * brief's "viewable without a server or build step" requirement. This helper is
 * the single source of truth for:
 *   - the canonical page inventory (PAGES, 17 stems) reused by every data-driven
 *     spec so no page can be silently dropped;
 *   - the breadcrumb-bearing / workspace-page subset (WORKSPACE_PAGES);
 *   - a `file://` resolver (mockupUrl) and fs roots (systemRoot / designRoot /
 *     mockupsRoot / specPath) derived from THIS file's location (__dirname),
 *     never process.cwd() — the run command executes from `tests/harness/`;
 *   - the shared page-opening helper (openMockup) that attaches request /
 *     pageerror / requestfailed collectors BEFORE goto (requests emitted before
 *     attachment are otherwise lost) so the offline/self-contained proof is sound;
 *   - the mockup-only data-testid vocabulary (NOT part of the frozen
 *     helpers/README-testids.md + helpers/ui.ts contract).
 *
 * IMPORTANT: this helper does NOT import or extend `helpers/ui.ts`; the v2 UI
 * contract is frozen and these mockups deliberately depart from it.
 */

import type { Page } from "@playwright/test";
import fs from "node:fs";
import path from "node:path";
import { pathToFileURL } from "node:url";

// ── Filesystem roots (derived from __dirname — round 2, finding B3) ──
//
// This file lives at `tests/harness/helpers/mockups.ts`, so:
//   ../../harness/design            → designRoot
//   ../../harness/design/system     → systemRoot (the orchestrator's S&F mirror)
//   ../../harness/design/mockups    → mockupsRoot (the coder's HTML deliverables)
//   ../../harness/design/SPEC.md    → specPath (the coder's design spec)
export const designRoot = path.resolve(__dirname, "../../../harness/design");
export const systemRoot = path.resolve(designRoot, "system");
export const mockupsRoot = path.resolve(designRoot, "mockups");
export const specPath = path.resolve(designRoot, "SPEC.md");

// ── Canonical page inventory (all 17 stems — round 2, finding B6) ──
//
// Ordering mirrors the plan's m1.1 enumeration. Every existence (m1.1), sidebar
// (m1.2), token (m1.6) and breadcrumb (m2.4) check reuses this list, so a page
// cannot drift out of coverage.
export const PAGES = [
  "sessions",
  "workspace-overview",
  "prd-overview",
  "epic",
  "story-detail",
  "story-ambiguous",
  "story-invalid",
  "scenarios",
  "scenario-detail",
  "service",
  "vocabulary",
  "runs",
  "run-detail",
  "prompt-choice",
  "prompt-clarify",
  "prompt-freetext",
  "unavailable",
] as const;

export type PageName = (typeof PAGES)[number];

// Every workspace page carries the shared sidebar + breadcrumb frame; only the
// root `sessions` page (the pre-workspace list) does not. m1.2 (sidebar) and
// m2.4 (breadcrumb) both iterate this subset.
export const WORKSPACE_PAGES = PAGES.filter((name) => name !== "sessions");

// Alias kept for the plan's m2.4 vocabulary — identical set, exported so the
// breadcrumb spec reads intently and cannot drift from the inventory.
export const BREADCRUMB_PAGES = WORKSPACE_PAGES;

// The offline font-decode oracle (m1.6b) is heavier than the cheap applied-type
// check, so it runs on a small, representative subset of pages.
export const FONT_CHECK_PAGES: PageName[] = ["sessions", "workspace-overview", "story-detail"];

// ── file:// resolver ──

/** Absolute `file://` URL for a mockup page stem (with or without `.html`). */
export function mockupUrl(name: string): string {
  const file = name.endsWith(".html") ? name : `${name}.html`;

  return pathToFileURL(path.join(mockupsRoot, file)).href;
}

/** The exact resolved `file://` href of the mirror entrypoint the pages MUST link. */
export const tokensEntrypointHref = pathToFileURL(path.join(systemRoot, "tokens.css")).href;

// ── Mockup-only testid vocabulary (documented HERE, never in the frozen contract) ──
//
// These are the hooks the coder wires into the static HTML. They intentionally
// overlap in spelling with a few v2 UI names where the concept is identical
// (session-row, run-row), but they are declared independently here; the tests do
// NOT import helpers/ui.ts.
export const MOCKUP_TID = {
  // Layout frame (m1.2 / m1.8 / m2.4).
  sidebar: "mockup-sidebar", // the left navigation landmark
  breadcrumb: "mockup-breadcrumb", // the breadcrumb trail above the canvas
  canvas: "mockup-canvas", // the main content region
  // Sessions list (m1.1 / m3.3).
  sessionRow: "session-row",
  // Epic grouped table + scenario list rows (m1.1 / m1.5 / m3.5).
  storyRow: "story-row", // one row per story in the epic grouped table
  scenarioRow: "scenario-row", // one row per scenario in the flat Requirements list
  epicHeader: "mockup-epic-header", // the epic header region (carries the epic identity flag)
  // Runs (m1.1 / m2.3 / m3.8).
  runRow: "run-row", // + data-session-id (session-scoping surfaced in presentation)
  runState: "run-state", // the run state label on run-detail
  runOutcome: "run-outcome", // outcome badge on run-detail (non-success — m3.8)
  // Inventory + lifecycle chips (m3.1 / m3.9).
  inventoryChip: "inventory-chip", // + data-status ∈ the frozen 7-status vocabulary
  lifecycleChip: "lifecycle-chip", // + data-kind ∈ created | applied | refined
  statusPill: "status-pill", // story/epic status pill (m1.7)
  storyRaw: "story-raw", // the revealed raw-file view on story-detail (m2.6), hidden until activated
  // Degraded / flagged banners (m3.2 / m3.4 / m3.5).
  truncationBanner: "mockup-truncation-banner",
  folderConflict: "mockup-folder-conflict",
  pathMismatch: "mockup-path-mismatch",
  epicFlag: "mockup-epic-flag", // epic-scoped identity flag on the epic header
  storyFlag: "mockup-story-flag", // story-scoped identity flag on a story row
  // Prompt dialogs (m4). Native <dialog open> so structure is testable with JS off.
  promptDialog: "prompt-dialog", // + data-kind ∈ choice | clarify | freetext
} as const;

// The full frozen inventory-chip status vocabulary (helpers/README-testids.md):
// a DESIGN deliverable must define the WHOLE set, not just the happy states.
export const INVENTORY_STATUSES = [
  "present",
  "missing",
  "invalid",
  "not_a_dir",
  "present_empty",
  "ambiguous",
  "unknown",
] as const;

/** Sidebar section landmark testid, e.g. `sidebar-section-architecture`. */
export function sidebarSectionId(name: string): string {
  return `sidebar-section-${name}`;
}

// ── The token-application probe hook (m1.6c) ──
//
// The asserted element carries `data-token-probe="<semantic-custom-property>"`,
// naming the token it consumes for `color`. The check is self-referential (read
// the token, normalize it via a throwaway probe element, compare the element's
// computed color) so it never hardcodes a DesignSync-owned value. The mirror
// defines `--text-primary` (→ #040406) as the documented default; the coder
// records the final property + element in SPEC.md.
export const TOKEN_PROBE_ATTR = "data-token-probe";

// ── Shared page opener (attaches collectors BEFORE goto — round 2, finding B2) ──

export interface OpenedMockup {
  /** URLs of EVERY request the page issued (main doc, CSS, fonts, images, JS). */
  requests: string[];
  /** Uncaught page errors. */
  pageErrors: string[];
  /** URLs of requests that FAILED (missing file, refused connection, …). */
  requestFailures: string[];
}

/**
 * Navigate to a mockup page over file://, collecting the request / pageerror /
 * requestfailed streams that the offline & self-contained proofs (m1.6d) key on.
 * Listeners are attached BEFORE goto so no early request is lost, then the helper
 * awaits load + font readiness so late resource fetches are captured too.
 *
 * If the page file does not exist (the RED state before the coder builds the
 * mockups), `page.goto` rejects with net::ERR_FILE_NOT_FOUND — a navigation
 * failure, which is the intended red for m1–m4.
 */
export async function openMockup(page: Page, name: string): Promise<OpenedMockup> {
  const requests: string[] = [];
  const pageErrors: string[] = [];
  const requestFailures: string[] = [];

  page.on("request", (req) => requests.push(req.url()));
  page.on("pageerror", (err) => pageErrors.push(err.message));
  page.on("requestfailed", (req) => requestFailures.push(req.url()));

  await page.goto(mockupUrl(name), { waitUntil: "load" });
  // Force any @font-face fetches to resolve so their requests are captured.
  await page.evaluate(async () => {
    try {
      await document.fonts.ready;
    } catch {
      /* fonts API unavailable — the caller's assertions still run */
    }
  });

  return { requests, pageErrors, requestFailures };
}

/** Force-expand every `<details>` in the sidebar so nav links become clickable. */
export async function expandSidebarTree(page: Page): Promise<void> {
  await page.evaluate((sidebarTid) => {
    document
      .querySelectorAll(`[data-testid="${sidebarTid}"] details`)
      .forEach((d) => {
        (d as HTMLDetailsElement).open = true;
      });
  }, MOCKUP_TID.sidebar);
}

// ── Recursive fs scan (m5.3 / m5.4 / m5.6) ──

/** All files under `root` with one of the given extensions (lowercased), recursive. */
export function filesByExt(root: string, exts: string[]): string[] {
  const out: string[] = [];
  const walk = (dir: string): void => {
    for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
      const full = path.join(dir, entry.name);
      if (entry.isDirectory()) {
        walk(full);
      } else if (exts.includes(path.extname(entry.name).toLowerCase())) {
        out.push(full);
      }
    }
  };
  walk(root);

  return out;
}

/** Extract the raw argument of every CSS `url(...)` in `css`. */
export function cssUrls(css: string): string[] {
  const out: string[] = [];
  const re = /url\(\s*(['"]?)([^)'"]+)\1\s*\)/gi;
  let m: RegExpExecArray | null;
  while ((m = re.exec(css)) !== null) {
    out.push(m[2].trim());
  }

  return out;
}

/**
 * The set of S&F custom-property names (`--foo`) DECLARED anywhere in the mirror's
 * CSS (`system/**.css`). The m1.6c token check requires the probed property to be
 * a real mirror token — so a page defining an arbitrary inline `--fake-token`
 * cannot self-certify (round 2, finding 4).
 */
export function mirrorTokenNames(): Set<string> {
  const names = new Set<string>();
  const decl = /(--[A-Za-z0-9-]+)\s*:/g;
  for (const file of filesByExt(systemRoot, [".css"])) {
    const css = fs.readFileSync(file, "utf8");
    let m: RegExpExecArray | null;
    while ((m = decl.exec(css)) !== null) {
      names.add(m[1]);
    }
  }

  return names;
}
