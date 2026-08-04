/**
 * Design-conformance helpers for the workspace design gate (task
 * `design-conformance-tests`, R1 + R2). The design source of truth lives under
 * `harness/design/` (paths.yaml → design_system): `system/tokens.css` (the S&F
 * token palette) and `mockups/*.html` (the per-screen layout baseline). These
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
export const MOCKUPS_DIR = path.join(DESIGN_ROOT, "mockups");

/** The mockups' pinned desktop reference viewport (design/SPEC.md §6). */
export const DESKTOP_VIEWPORT = { width: 1440, height: 900 } as const;

/** `file://` URL for a self-contained offline mockup page. */
export function mockupFileUrl(name: string): string {
  return `file://${path.join(MOCKUPS_DIR, name)}`;
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

/** The concrete named layout checks (design/SPEC.md §1) the judge must report. */
export const JUDGE_CHECK_NAMES = [
  "persistent_frame",
  "sidebar_fixed_width",
  "breadcrumb_hairline",
  "canvas_padding",
] as const;

/** Schema forcing codex's final message to a machine-readable verdict. */
export const JUDGE_SCHEMA = {
  type: "object",
  additionalProperties: false,
  required: ["verdict", "checks"],
  properties: {
    verdict: { type: "string", enum: ["pass", "fail"] },
    checks: {
      type: "array",
      minItems: JUDGE_CHECK_NAMES.length,
      maxItems: JUDGE_CHECK_NAMES.length,
      items: {
        type: "object",
        additionalProperties: false,
        required: ["name", "status", "note"],
        properties: {
          name: { type: "string", enum: [...JUDGE_CHECK_NAMES] },
          status: { type: "string", enum: ["pass", "fail"] },
          note: { type: "string" },
        },
      },
    },
  },
} as const;

/**
 * Runtime-validates a judge verdict beyond the type cast (Codex r1 #6/#7): a
 * schema-valid but incomplete/contradictory reply (e.g. `{"verdict":"pass",
 * "checks":[]}`) must NOT let the missing production breadcrumb be born green.
 * Returns the failed checks and any STRUCTURAL problems; a non-empty `problems`
 * list is itself an assertion failure in the spec.
 */
export function auditVerdict(verdict: JudgeVerdict): { failedChecks: JudgeCheck[]; problems: string[] } {
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
  for (const name of JUDGE_CHECK_NAMES) {
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
  timeoutMs?: number;
}): Promise<JudgeRun> {
  fs.mkdirSync(opts.artifactDir, { recursive: true });
  const schemaPath = path.join(opts.artifactDir, `${opts.label}.schema.json`);
  const promptPath = path.join(opts.artifactDir, `${opts.label}.prompt.txt`);
  const verdictPath = path.join(opts.artifactDir, `${opts.label}.verdict.json`);
  const tracePath = path.join(opts.artifactDir, `${opts.label}.trace.log`);

  fs.writeFileSync(schemaPath, JSON.stringify(JUDGE_SCHEMA, null, 2));
  fs.writeFileSync(promptPath, judgeRubric());
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
    child.stdin.write(judgeRubric());
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
