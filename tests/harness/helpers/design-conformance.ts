/**
 * Design-conformance helpers for the workspace design gate (task
 * `design-conformance-tests`, R1 + R2). The design source of truth lives under
 * `harness/design/` (paths.yaml → design_system): `system/tokens.css` (the S&F
 * token palette) and the runnable prototype (`proto-workspace`, the per-screen
 * layout baseline — booted via helpers/proto-baseline.ts). These
 * helpers:
 *
 *   - parse the token palette (R1 deterministic checks) — the allowed colour and
 *     font-family set a production workspace page must stay inside;
 *   - sweep a live production page for computed colours / fonts OUTSIDE that set,
 *     naming the offending element + value (R1);
 *   - drive the local `codex` CLI as a vision judge (R2) — two screenshots
 *     (designed mockup vs production page) + a schema-forced JSON verdict of
 *     concrete named layout checks derived from `harness/design/SPEC.md`.
 *
 * Nothing here launches Claude through the harness — R2's judge is a LOCAL codex
 * subprocess, so both specs stay in the `w*` (workspace) Playwright project.
 */

import { spawn } from "node:child_process";
import fs from "node:fs";
import path from "node:path";

import type { Page } from "@playwright/test";

import { findRepoRoot } from "./suite-root";

// ── Design-source locations (all under paths.yaml → design_system) ──

export const REPO_ROOT = findRepoRoot();
export const DESIGN_ROOT = path.join(REPO_ROOT, "harness", "design");
export const TOKENS_CSS = path.join(DESIGN_ROOT, "system", "tokens.css");
/** The repo-owned workspace density layer (`--wk-*` scale) promoted from the
 * prototype skin — the design truth for workspace CHROME sizing (w16). */
export const WORKSPACE_CSS = path.join(DESIGN_ROOT, "system", "workspace.css");
/** The design baseline's pinned desktop reference viewport (design/SPEC.md §6). */
export const DESKTOP_VIEWPORT = { width: 1440, height: 900 } as const;

// ── Design gold standards ──
//
// The judge's baseline images are COMMITTED gold-standard screenshots under
// tests/harness/goldens/, captured from the booted prototype at test-AUTHORING
// time via `npm run goldens:update` (goldens.update.spec.ts). Judge specs
// compare golden vs the live production render — they never boot the
// prototype themselves, so a run compares against exactly what the test
// author signed off, not whatever the prototype happens to render today.
export const GOLDENS_DIR = path.join(REPO_ROOT, "tests", "harness", "goldens");

/** The stable golden names (file: `<name>.png`) the judge specs consume. */
export const GOLDEN_NAMES = {
  workspaceOverview: "workspace-overview",
  productLanding: "product-landing",
  storyPage: "story-page",
  featureDetail: "feature-detail",
} as const;

/**
 * Resolves a committed golden's path, failing LOUDLY when it is absent — a
 * missing golden is an authoring defect (the test changed without recapturing
 * its gold standard), never a skip.
 */
export function requireGolden(name: string): string {
  const golden = path.join(GOLDENS_DIR, `${name}.png`);
  if (!fs.existsSync(golden)) {
    throw new Error(
      `design golden missing: ${golden}\n` +
        "Gold standards are captured from the design prototype at test-authoring time — " +
        "run `npm run goldens:update` in tests/harness after writing or changing a visual spec.",
    );
  }

  return golden;
}

/**
 * Parses the workspace density layer into a `--wk-*` → px map. Reading the
 * DESIGN FILE (not the live page) keeps the expectation anchored to design
 * truth even when the production app fails to consume the layer at all —
 * the spec then reports "renders 20px, design says 13px" instead of
 * comparing two empty strings.
 */
export function readWorkspaceScale(): Record<string, number> {
  const css = fs.readFileSync(WORKSPACE_CSS, "utf8");
  const out: Record<string, number> = {};
  const re = /(--wk-[a-z-]+)\s*:\s*(\d+(?:\.\d+)?)px/g;
  let match: RegExpExecArray | null;
  while ((match = re.exec(css)) !== null) {
    out[match[1]] = Number.parseFloat(match[2]);
  }

  return out;
}

// ── R1: token palette parsing ──

/**
 * Every hex colour literal declared in tokens.css — the design-system palette a
 * rendered workspace page must draw exclusively from (semantic aliases such as
 * `--text-body` resolve BACK to one of these, so the raw hex set is sufficient).
 */
export function readTokenHexColors(): string[] {
  const css = fs.readFileSync(TOKENS_CSS, "utf8");
  const set = new Set<string>();
  const re = /#[0-9a-fA-F]{3,8}\b/g;
  let match: RegExpExecArray | null;
  while ((match = re.exec(css)) !== null) {
    set.add(match[0].toLowerCase());
  }

  return [...set];
}

/**
 * Resolves every palette hex to its computed `rgb(...)` / `rgba(...)` form (as
 * `getComputedStyle` reports colours) using a probe element in the live page,
 * plus the universally-allowed fully-transparent value (an unset background is
 * not a design violation).
 */
export async function resolveAllowedColors(page: Page): Promise<Set<string>> {
  const hexes = readTokenHexColors();
  const resolved = await page.evaluate((list: string[]) => {
    const probe = document.createElement("span");
    document.body.appendChild(probe);
    const out = list.map((hex) => {
      probe.style.color = "";
      probe.style.color = hex;

      return getComputedStyle(probe).color;
    });
    probe.remove();

    return out;
  }, hexes);

  const set = new Set<string>(resolved);
  set.add("rgba(0, 0, 0, 0)"); // transparent — no paint, never a violation

  return set;
}

export interface StyleViolation {
  selector: string;
  property: string;
  value: string;
}

/**
 * Sweeps every VISIBLE element of the current page and returns each computed
 * `color` (text-bearing elements only), `background-color`, and visible
 * border-side colour that is NOT in the allowed token set. A non-empty result is
 * an R1 failure; each entry names the offending element + property + value.
 */
export async function collectColorViolations(page: Page): Promise<StyleViolation[]> {
  const allowed = [...(await resolveAllowedColors(page))];

  return page.evaluate((allowedList: string[]) => {
    const allow = new Set(allowedList);

    const describe = (el: Element): string => {
      const tid = el.getAttribute("data-testid");
      const tidPart = tid === null ? "" : `[data-testid="${tid}"]`;
      const clsPart = el.classList.length > 0 ? `.${[...el.classList].join(".")}` : "";

      return `${el.tagName.toLowerCase()}${tidPart}${clsPart}`;
    };

    // Visibility that also rules out a hidden/opacity:0 ANCESTOR (Codex r1 #2):
    // `checkVisibility` walks the ancestor chain; the rect guard drops 0x0 boxes.
    const isVisible = (el: Element): boolean => {
      const rect = el.getBoundingClientRect();
      if (rect.width === 0 || rect.height === 0) {
        return false;
      }
      const anyEl = el as Element & {
        checkVisibility?: (opts: { opacityProperty: boolean; visibilityProperty: boolean }) => boolean;
      };
      if (typeof anyEl.checkVisibility === "function") {
        return anyEl.checkVisibility({ opacityProperty: true, visibilityProperty: true });
      }
      const s = getComputedStyle(el);

      return s.visibility !== "hidden" && s.display !== "none" && s.opacity !== "0";
    };

    // Form controls render their value/placeholder text without a text node, so
    // treat them as text-bearing too — otherwise the file-view `<textarea>` and
    // any input escape the colour/type check (Codex r1 #3).
    const isFormControl = (el: Element): boolean =>
      ["input", "textarea", "select"].includes(el.tagName.toLowerCase());
    const bearsText = (el: Element): boolean =>
      isFormControl(el) ||
      Array.from(el.childNodes).some((n) => n.nodeType === Node.TEXT_NODE && (n.textContent ?? "").trim() !== "");

    const violations: Array<{ selector: string; property: string; value: string }> = [];
    const elements = [document.body, ...Array.from(document.body.querySelectorAll("*"))];

    for (const el of elements) {
      if (!isVisible(el)) {
        continue;
      }
      const s = getComputedStyle(el);

      if (bearsText(el) && !allow.has(s.color)) {
        violations.push({ selector: describe(el), property: "color", value: s.color });
      }
      if (!allow.has(s.backgroundColor)) {
        violations.push({ selector: describe(el), property: "background-color", value: s.backgroundColor });
      }
      for (const side of ["top", "right", "bottom", "left"]) {
        const width = Number.parseFloat(s.getPropertyValue(`border-${side}-width`));
        const style = s.getPropertyValue(`border-${side}-style`);
        if (width > 0 && style !== "none") {
          const color = s.getPropertyValue(`border-${side}-color`);
          if (!allow.has(color)) {
            violations.push({ selector: describe(el), property: `border-${side}-color`, value: color });
          }
        }
      }
    }

    return violations;
  }, allowed);
}

/**
 * Sweeps every VISIBLE text-bearing element and returns each whose primary
 * font-family is neither the design face (`Poppins`) NOR — inside the SCOPED
 * monospace exception — a monospace family. Per README-testids the monospace
 * surface is precisely the file-view EDITOR + GUTTER (not the whole file-view:
 * its path label and save-state are Poppins), so the exception is scoped to
 * `file-view-editor` / `file-view-gutter` only. A non-empty result is an R1
 * typography failure.
 */
export async function collectFontViolations(page: Page): Promise<StyleViolation[]> {
  return page.evaluate(() => {
    const monoScopes = Array.from(
      document.querySelectorAll('[data-testid="file-view-editor"], [data-testid="file-view-gutter"]'),
    );
    const inMonoScope = (el: Element): boolean => monoScopes.some((scope) => scope === el || scope.contains(el));
    const primary = (fontFamily: string): string =>
      fontFamily
        .split(",")[0]
        .trim()
        .replace(/^["']|["']$/g, "")
        .toLowerCase();

    // The scoped monospace exception accepts ANY legitimate monospace stack, not
    // a hard-coded family list (Codex r1 #4): a `mono`-named primary OR the
    // generic `monospace` fallback anywhere in the resolved family.
    const isMonospace = (fontFamily: string): boolean =>
      /mono/i.test(primary(fontFamily)) || fontFamily.toLowerCase().includes("monospace");

    const describe = (el: Element): string => {
      const tid = el.getAttribute("data-testid");
      const tidPart = tid === null ? "" : `[data-testid="${tid}"]`;
      const clsPart = el.classList.length > 0 ? `.${[...el.classList].join(".")}` : "";

      return `${el.tagName.toLowerCase()}${tidPart}${clsPart}`;
    };

    const isVisible = (el: Element): boolean => {
      const rect = el.getBoundingClientRect();
      if (rect.width === 0 || rect.height === 0) {
        return false;
      }
      const anyEl = el as Element & {
        checkVisibility?: (opts: { opacityProperty: boolean; visibilityProperty: boolean }) => boolean;
      };
      if (typeof anyEl.checkVisibility === "function") {
        return anyEl.checkVisibility({ opacityProperty: true, visibilityProperty: true });
      }
      const s = getComputedStyle(el);

      return s.visibility !== "hidden" && s.display !== "none" && s.opacity !== "0";
    };

    const isFormControl = (el: Element): boolean =>
      ["input", "textarea", "select"].includes(el.tagName.toLowerCase());
    const bearsText = (el: Element): boolean =>
      isFormControl(el) ||
      Array.from(el.childNodes).some((n) => n.nodeType === Node.TEXT_NODE && (n.textContent ?? "").trim() !== "");

    const violations: Array<{ selector: string; property: string; value: string }> = [];
    const elements = [document.body, ...Array.from(document.body.querySelectorAll("*"))];

    for (const el of elements) {
      if (!isVisible(el) || !bearsText(el)) {
        continue;
      }
      const fontFamily = getComputedStyle(el).fontFamily;
      const ok = inMonoScope(el) ? isMonospace(fontFamily) : primary(fontFamily) === "poppins";
      if (!ok) {
        violations.push({ selector: describe(el), property: "font-family", value: primary(fontFamily) });
      }
    }

    return violations;
  });
}

/** True iff the design face is ACTUALLY loaded + rendering (not just declared). */
export async function poppinsActuallyRenders(page: Page): Promise<boolean> {
  return page.evaluate(async () => {
    await document.fonts.ready;

    return document.fonts.check('700 32px "Poppins"') && document.fonts.check('500 20px "Poppins"');
  });
}

// ── R2: codex vision judge ──

/**
 * Whether the local `codex` CLI is resolvable on PATH. R2 skips cleanly with a
 * named reason when it is absent (the judge needs the vision model); R1 never
 * consults this — it is deterministic.
 */
export function codexOnPath(): boolean {
  const exts = process.platform === "win32" ? [".exe", ".cmd", ".bat", ""] : [""];
  for (const dir of (process.env.PATH ?? "").split(path.delimiter)) {
    if (dir === "") {
      continue;
    }
    for (const ext of exts) {
      const candidate = path.join(dir, `codex${ext}`);
      try {
        fs.accessSync(candidate, fs.constants.X_OK);

        return true;
      } catch {
        /* keep scanning */
      }
    }
  }

  return false;
}

export interface JudgeCheck {
  name: string;
  status: "pass" | "fail";
  note: string;
}

export interface JudgeVerdict {
  verdict: "pass" | "fail";
  checks: JudgeCheck[];
}

/** The concrete named layout checks (design/SPEC.md §1) the FRAME judge reports. */
export const JUDGE_CHECK_NAMES = [
  "persistent_frame",
  "sidebar_fixed_width",
  "breadcrumb_hairline",
  "canvas_padding",
] as const;

/**
 * Builds a schema forcing codex's final message to a machine-readable verdict
 * over EXACTLY the given named checks. Parameterised so a second design-judge
 * pair (the workspace-overview canvas parity) reuses this helper with a
 * DIFFERENT check set instead of forking a parallel judge.
 */
export function buildJudgeSchema(checkNames: readonly string[]) {
  return {
    type: "object",
    additionalProperties: false,
    required: ["verdict", "checks"],
    properties: {
      verdict: { type: "string", enum: ["pass", "fail"] },
      checks: {
        type: "array",
        minItems: checkNames.length,
        maxItems: checkNames.length,
        items: {
          type: "object",
          additionalProperties: false,
          required: ["name", "status", "note"],
          properties: {
            name: { type: "string", enum: [...checkNames] },
            status: { type: "string", enum: ["pass", "fail"] },
            note: { type: "string" },
          },
        },
      },
    },
  } as const;
}

/** Schema for the FRAME profile (design/SPEC.md §1); the default judge target. */
export const JUDGE_SCHEMA = buildJudgeSchema(JUDGE_CHECK_NAMES);

/** The named canvas-parity checks the workspace-overview judge pair reports. */
export const CANVAS_PARITY_CHECK_NAMES = [
  "title_metadata",
  "actions_row",
  "inventory_health",
  "breadcrumb_trail",
] as const;

/** A judge configuration: the named checks + the rubric that scores them. */
export interface JudgeProfile {
  checkNames: readonly string[];
  rubric: string;
}

/**
 * Runtime-validates a judge verdict beyond the type cast (Codex r1 #6/#7): a
 * schema-valid but incomplete/contradictory reply (e.g. `{"verdict":"pass",
 * "checks":[]}`) must NOT let the missing production breadcrumb be born green.
 * Returns the failed checks and any STRUCTURAL problems; a non-empty `problems`
 * list is itself an assertion failure in the spec.
 */
export function auditVerdict(
  verdict: JudgeVerdict,
  checkNames: readonly string[] = JUDGE_CHECK_NAMES,
): { failedChecks: JudgeCheck[]; problems: string[] } {
  const problems: string[] = [];
  const checks = Array.isArray(verdict?.checks) ? verdict.checks : [];
  if (!Array.isArray(verdict?.checks)) {
    problems.push("verdict.checks is not an array");
  }
  if (verdict?.verdict !== "pass" && verdict?.verdict !== "fail") {
    problems.push(`verdict.verdict is not "pass"|"fail" (got ${JSON.stringify(verdict?.verdict)})`);
  }

  const seen = new Map<string, number>();
  for (const check of checks) {
    seen.set(check.name, (seen.get(check.name) ?? 0) + 1);
  }
  for (const name of checkNames) {
    const count = seen.get(name) ?? 0;
    if (count === 0) {
      problems.push(`missing required check "${name}"`);
    } else if (count > 1) {
      problems.push(`duplicate check "${name}" (x${count})`);
    }
  }

  const failedChecks = checks.filter((check) => check.status === "fail");
  if (verdict?.verdict === "pass" && failedChecks.length > 0) {
    problems.push("verdict is \"pass\" but one or more checks failed");
  }
  if (verdict?.verdict === "fail" && failedChecks.length === 0 && problems.length === 0) {
    problems.push("verdict is \"fail\" but no check reported a failure");
  }

  return { failedChecks, problems };
}

function judgeRubric(): string {
  return [
    "You are a strict design-conformance judge for a web UI.",
    "",
    "IMAGE 1 is the DESIGN MOCKUP (the source of truth for LAYOUT and STRUCTURE).",
    "IMAGE 2 is the PRODUCTION application screenshot, taken at the same 1440x900 desktop viewport.",
    "",
    "Judge ONLY structure, layout, spacing and typography INTENT of the production",
    "screenshot against the mockup. IGNORE all content/data differences (different",
    "file names, list rows, labels, placeholder vs fixture text) — those must NOT",
    "fail any check.",
    "",
    "Evaluate exactly these named checks and return the schema-forced JSON verdict:",
    "- persistent_frame: production shows the SAME three-region frame as the mockup —",
    "  a persistent left navigation sidebar, a horizontal breadcrumb bar above the",
    "  content, AND a main content canvas. FAIL if any of the three regions is absent.",
    "- sidebar_fixed_width: the left navigation is a fixed-width vertical sidebar column",
    "  comparable to the mockup's (not a narrow icon-only strip, not full-bleed content).",
    "- breadcrumb_hairline: a breadcrumb/trail bar sits at the top of the content area,",
    "  visually separated from the canvas below it by a thin hairline bottom border.",
    "- canvas_padding: the main content canvas has generous inner padding around its",
    "  content (content is not flush against the frame edges).",
    "",
    "status is 'pass' or 'fail' per check with a one-line note citing the visual evidence.",
    "verdict is 'pass' ONLY if every check is 'pass', otherwise 'fail'.",
  ].join("\n");
}

/**
 * The workspace-overview CANVAS-PARITY rubric (task R6). Judges the main content
 * canvas + breadcrumb of the production `/home` page against the mockup, while
 * TOLERATING the icon rail + docked sidebar navigation chrome (production uses a
 * narrow icon rail + secondary sidebar where the mockup uses one labelled
 * sidebar — that column difference must NEVER fail a check, exactly as the w9
 * frame judge tolerates the rail). All content/data VALUES are ignored.
 */
function canvasParityRubric(): string {
  return [
    "You are a strict design-conformance judge for a web UI.",
    "",
    "IMAGE 1 is the DESIGN MOCKUP (the source of truth for the CONTENT CANVAS layout).",
    "IMAGE 2 is the PRODUCTION application screenshot, taken at the same 1440x900 desktop viewport.",
    "",
    "Judge ONLY the MAIN CONTENT CANVAS (the large region right of the navigation)",
    "and the breadcrumb bar above it. Both images show a narrow icon rail plus a",
    "docked sidebar on the left — IGNORE the left-navigation chrome entirely; any",
    "difference there must NOT fail any check.",
    "IGNORE all content/data differences too (folder path text,",
    "session id, inventory row names/paths, chip status values and counts, button",
    "wording) — judge STRUCTURE and LAYOUT INTENT only.",
    "",
    "Evaluate exactly these named checks and return the schema-forced JSON verdict:",
    "- title_metadata: the canvas LEADS with a page-title heading and, directly",
    "  beneath it, a secondary metadata line (a folder path + session id), like the",
    "  mockup's header block. FAIL if the canvas has no title-with-metadata header.",
    "- actions_row: a horizontal ROW of build-action buttons (three: build tests,",
    "  build code, refresh) sits in the canvas header area, like the mockup's button",
    "  row. FAIL if there is no such row of action buttons in the canvas.",
    "- inventory_health: the canvas shows an INVENTORY-HEALTH LIST — stacked rows,",
    "  each with a label on the left and a right-aligned status CHIP/PILL — like the",
    "  mockup's inventory list. FAIL if there is no such list of rows with status chips.",
    "- breadcrumb_trail: a MULTI-LEVEL breadcrumb trail (a parent 'Sessions' link,",
    "  a separator, then the current page crumb) sits above the canvas, like the",
    "  mockup's 'Sessions / Workspace overview'. Tolerate the exact labels; FAIL only",
    "  if the breadcrumb is single-level or absent.",
    "",
    "status is 'pass' or 'fail' per check with a one-line note citing the visual evidence.",
    "verdict is 'pass' ONLY if every check is 'pass', otherwise 'fail'.",
  ].join("\n");
}

/** The default (frame) judge profile — design/SPEC.md §1 three-region frame. */
export const FRAME_JUDGE_PROFILE: JudgeProfile = {
  checkNames: JUDGE_CHECK_NAMES,
  rubric: judgeRubric(),
};

/** The workspace-overview canvas-parity judge profile (task R6). */
export const CANVAS_PARITY_PROFILE: JudgeProfile = {
  checkNames: CANVAS_PARITY_CHECK_NAMES,
  rubric: canvasParityRubric(),
};

// ── Product-section prototype-parity judge profiles (task
// `product-section-prototype-parity`, R8). Unlike the frame/canvas judges, the
// BASELINE image is the runnable PROTOTYPE app (booted live from the repo, see
// helpers/proto-baseline.ts) — NOT a static mockup — and BOTH images use the
// rail + docked sidebar, so the rail/sidebar chrome is compared DIRECTLY (the
// w9/w11 "tolerate the left-nav difference" clause is deliberately DROPPED).

/** Named checks shared by every product-parity page (shell chrome).
 * `type_scale`, `chat_toggle`, and `breadcrumb_trail` were added 2026-08-04
 * after a passed-green-but-visibly-wrong episode: the original presence-only
 * check set could not fail an oversized type scale, a restyled chat
 * affordance, or a flattened breadcrumb (tmp/design-compare/why-tests-passed.md). */
const PRODUCT_SHELL_CHECKS = [
  "sidebar_structure",
  "rail_labels",
  "kicker_title_block",
  "type_scale",
  "chat_toggle",
  "breadcrumb_trail",
] as const;

export const PRODUCT_LANDING_CHECK_NAMES = [...PRODUCT_SHELL_CHECKS, "file_card"] as const;
export const STORY_PAGE_CHECK_NAMES = [...PRODUCT_SHELL_CHECKS, "file_card", "feature_pill"] as const;
export const FEATURE_DETAIL_CHECK_NAMES = [...PRODUCT_SHELL_CHECKS, "section_lists"] as const;

/** The check-name → rubric-clause text every product-parity rubric can draw from. */
const PRODUCT_CHECK_CLAUSES: Record<string, string> = {
  sidebar_structure:
    "- sidebar_structure: the docked left sidebar has the SAME structure as the baseline — a brand header " +
    "(a product/app name over a smaller muted context line) at the top, then an UNDERLINED section header, " +
    "a highlighted file row, and labelled subsection GROUPS (features / stories / scenarios) listing rows " +
    "with a vertical guide line. FAIL if the sidebar lacks the brand header, the underlined section header, " +
    "the labelled groups, OR the vertical guide line on nested rows; ALSO FAIL if list rows are individually " +
    "ruled with underlines/borders (the baseline rules only section headers, its rows are plain), or if any " +
    "sidebar entry is DUPLICATED (the same row/label rendered twice, e.g. the file row appearing as both a " +
    "chip and a row).",
  rail_labels:
    "- rail_labels: the narrow dark far-left rail shows, per entry, an ICON glyph ABOVE/BEFORE a small-caps " +
    "text label (the full section words), and the ACTIVE entry is an INVERTED (light-on-dark → dark-on-light) " +
    "tile; a session/utility entry sits pinned at the BOTTOM. FAIL if the rail entries are icon-only, or have " +
    "no text labels, or the active entry is not an inverted tile.",
  kicker_title_block:
    "- kicker_title_block: the content canvas LEADS with a small uppercase kicker line, then a large display " +
    "title beneath it, then a muted subtitle line beneath that (three stacked, decreasing-emphasis lines). " +
    "FAIL if the canvas has no kicker-over-title-over-subtitle header block.",
  file_card:
    "- file_card: the file is shown as a GitHub-style CARD — a header BAR with the document path on the left " +
    "and an 'N lines' counter on the right, then a line-number gutter beside a monospace file body; the card " +
    "is OUTLINED by a strong DARK border and ENDS at its content (no large empty region trailing below the " +
    "last line). FAIL if there is no header bar with a path + line-count, no gutter beside the body, no dark " +
    "card outline, or the card sprawls with a large empty tail below the content.",
  type_scale:
    "- type_scale: the chrome typography DENSITY matches the baseline — sidebar rows, group labels, " +
    "breadcrumb and header-bar text render at visually the SAME small scale as the baseline (a dense " +
    "13-16px tool-UI feel), NOT a larger editorial scale. Compare row heights and how much fits vertically: " +
    "FAIL if production chrome text is noticeably larger, or its rows noticeably taller, than the baseline's.",
  chat_toggle:
    "- chat_toggle: the right-edge chat affordance is a DARK INVERTED vertical strip bearing an UPPERCASE " +
    "vertical text label (like the baseline's 'CHAT' tab). FAIL if it is a light/gray strip, a bare " +
    "chevron/arrow or icon, or carries no text label.",
  breadcrumb_trail:
    "- breadcrumb_trail: a MULTI-LEVEL breadcrumb trail sits above the canvas — at least one parent LINK, a " +
    "SPACED separator, then a visually distinct (bold/dark) current crumb, like the baseline's " +
    "'Sessions / Workspace overview / <doc>'. Tolerate the exact labels; FAIL if the trail is single-level " +
    "or absent, or the separator is jammed against the crumb text with no spacing around it.",
  feature_pill:
    "- feature_pill: ABOVE the file card sits a 'Feature: <name>' PILL control bearing a change affordance " +
    "(a label + a value + a change control in one pill). FAIL if there is no such feature pill above the card.",
  section_lists:
    "- section_lists: the canvas shows THREE named sections (user stories / requirements / unaligned " +
    "requirements), each a stacked list of card ROWS where every row has a LINKED title on the LEFT and a " +
    "'Feature:' pill control on the RIGHT. FAIL if the three sections or the left-title/right-pill row anatomy " +
    "are absent.",
};

function productParityRubric(checkNames: readonly string[]): string {
  return [
    "You are a strict design-conformance judge for a web UI.",
    "",
    "IMAGE 1 is the BASELINE — the committed gold-standard screenshot of the design-truth",
    "prototype for this exact page (captured at test-authoring time).",
    "IMAGE 2 is the PRODUCTION application screenshot, taken at the same 1440x900 desktop viewport.",
    "",
    "Judge ONLY structure, layout, spacing and typography INTENT of the production",
    "screenshot against the baseline. BOTH images use the SAME navigation chrome (a narrow",
    "icon rail PLUS a docked sidebar), so compare that chrome DIRECTLY — it is NOT exempt.",
    "IGNORE all content/data differences (different file names, list rows, ids, labels,",
    "descriptions, counts, placeholder vs fixture text) — those must NOT fail any check.",
    "",
    "Evaluate exactly these named checks and return the schema-forced JSON verdict:",
    ...checkNames.map((name) => PRODUCT_CHECK_CLAUSES[name]),
    "",
    "status is 'pass' or 'fail' per check with a one-line note citing the visual evidence.",
    "verdict is 'pass' ONLY if every check is 'pass', otherwise 'fail'.",
  ].join("\n");
}

/** w15.1 — product landing (`/product`) prototype-parity judge profile. */
export const PRODUCT_LANDING_PROFILE: JudgeProfile = {
  checkNames: PRODUCT_LANDING_CHECK_NAMES,
  rubric: productParityRubric(PRODUCT_LANDING_CHECK_NAMES),
};

/** w15.2 — story page prototype-parity judge profile. */
export const STORY_PAGE_PROFILE: JudgeProfile = {
  checkNames: STORY_PAGE_CHECK_NAMES,
  rubric: productParityRubric(STORY_PAGE_CHECK_NAMES),
};

/** w15.3 — feature-detail prototype-parity judge profile. */
export const FEATURE_DETAIL_PROFILE: JudgeProfile = {
  checkNames: FEATURE_DETAIL_CHECK_NAMES,
  rubric: productParityRubric(FEATURE_DETAIL_CHECK_NAMES),
};

export interface JudgeRun {
  verdict: JudgeVerdict;
  verdictPath: string;
  tracePath: string;
  promptPath: string;
  exitCode: number;
}

/**
 * Runs the codex vision judge over the two screenshots and returns the parsed
 * verdict. Read-only sandbox (codex never edits); `--output-schema` + `-o` force
 * and capture a JSON verdict; stdout/stderr are teed to a trace artifact so a
 * failure/timeout still leaves a diagnosable log. Throws (never silently passes)
 * if codex produced no parseable verdict — a broken judge is a test defect, not
 * a green.
 */
export async function runDesignJudge(opts: {
  mockupPng: string;
  prodPng: string;
  artifactDir: string;
  label: string;
  /** The judge configuration (named checks + rubric). Defaults to the FRAME
   * profile so existing callers (w9) are unchanged. */
  profile?: JudgeProfile;
  timeoutMs?: number;
}): Promise<JudgeRun> {
  const profile = opts.profile ?? FRAME_JUDGE_PROFILE;
  fs.mkdirSync(opts.artifactDir, { recursive: true });
  const schemaPath = path.join(opts.artifactDir, `${opts.label}.schema.json`);
  const promptPath = path.join(opts.artifactDir, `${opts.label}.prompt.txt`);
  const verdictPath = path.join(opts.artifactDir, `${opts.label}.verdict.json`);
  const tracePath = path.join(opts.artifactDir, `${opts.label}.trace.log`);

  fs.writeFileSync(schemaPath, JSON.stringify(buildJudgeSchema(profile.checkNames), null, 2));
  fs.writeFileSync(promptPath, profile.rubric);
  if (fs.existsSync(verdictPath)) {
    fs.rmSync(verdictPath);
  }

  const args = [
    "exec",
    "-s",
    "read-only",
    "--ephemeral",
    "--skip-git-repo-check",
    "-C",
    REPO_ROOT,
    "--color",
    "never",
    "-c",
    "model_reasoning_effort=low",
    "-i",
    opts.mockupPng,
    "-i",
    opts.prodPng,
    "--output-schema",
    schemaPath,
    "-o",
    verdictPath,
    "-",
  ];

  const traceFd = fs.openSync(tracePath, "w");
  const exitCode = await new Promise<number>((resolve, reject) => {
    const child = spawn("codex", args, { cwd: REPO_ROOT, stdio: ["pipe", traceFd, traceFd] });
    const timer = setTimeout(() => child.kill("SIGKILL"), opts.timeoutMs ?? 150_000);
    child.once("error", (err) => {
      clearTimeout(timer);
      reject(err);
    });
    child.once("exit", (code) => {
      clearTimeout(timer);
      resolve(code ?? 1);
    });
    child.stdin.write(profile.rubric);
    child.stdin.end();
  });
  fs.closeSync(traceFd);

  if (!fs.existsSync(verdictPath)) {
    const tail = fs.readFileSync(tracePath, "utf8").split("\n").slice(-25).join("\n");
    throw new Error(`codex produced no verdict (exit ${exitCode}); trace ${tracePath}\n${tail}`);
  }

  const verdict = JSON.parse(fs.readFileSync(verdictPath, "utf8")) as JudgeVerdict;

  return { verdict, verdictPath, tracePath, promptPath, exitCode };
}
