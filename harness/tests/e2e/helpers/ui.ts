/**
 * UI navigation + selector contract for the harness views (plan §3.5).
 * The protocol specs assert against exactly these routes and
 * data-testid attributes; the UI phase is implemented to match. The
 * full contract, including data-* attribute vocabularies, is
 * documented in helpers/README-testids.md.
 *
 * Navigation (App Router):
 *   /                sessions list
 *   /sessions/<id>   session detail (inventory + stories + actions)
 *   /runs/<id>       run view (output, prompt panel, outcome badge)
 *
 * Live-update contract: all three views poll their API and re-render
 * without a manual reload — Playwright assertions rely on locator
 * auto-wait against a self-updating page.
 */

import type { Locator, Page } from "@playwright/test";

// ── Static testids ──

export const TID = {
  // Sessions list (one row per session).
  sessionRow: "session-row", // + data-session-id, data-folder
  sessionFolder: "session-folder", // text = canonical folder (realpath)
  sessionReachability: "session-reachability", // text: connected | unreachable
  testConnection: "test-connection", // per-row control; dispatches `version`

  // Session detail.
  inventoryGeneration: "inventory-generation", // text = promoted generation number
  refresh: "refresh", // POSTs /api/sessions/:id/refresh
  folderWarningBanner: "folder-warning-banner", // sibling-session folder activity
  pathMismatchWarning: "path-mismatch-warning", // non-default architecture path
  runHistory: "run-history",
  runRow: "run-row", // + data-run-id, data-command
  actionBuildTests: "action-build-tests", // <button>, disabled per plan §3.5
  actionBuildCode: "action-build-code", // <button>
  actionCreate: "action-create", // per story row
  actionRefine: "action-refine", // per story row
  actionApply: "action-apply", // per story row
  fixToggle: "fix-toggle",

  // Session detail: epics table.
  epicRow: "epic-row", // + data-epic-file, data-epic-number, data-status
  epicFlagDuplicateNumber: "epic-flag-duplicate-number",
  epicFlagIdMismatch: "epic-flag-id-mismatch",

  // Session detail: story-row cells + flags.
  storyCreated: "story-created", // data-status: one|missing|ambiguous|invalid
  storyApplied: "story-applied", // text "x/y" or "unknown (<reason>)"
  storyRefined: "story-refined", // text "not recorded" in v1
  storyFlagDuplicateDeclaredId: "story-flag-duplicate-declared-id",
  storyFlagIdMismatch: "story-flag-id-mismatch",
  storyFlagDeprecatedFormat: "story-flag-deprecated-format",
  storyFlagNoAcs: "story-flag-no-acs",
  storyFlagEmptyInternalId: "story-flag-empty-internal-id",

  // Run view.
  runState: "run-state", // text = run state (queued, running, terminal, ...)
  runOutcome: "run-outcome", // text = outcome badge: ok, abandoned, error(no_result), ...
  runOutput: "run-output", // output tail
  reachabilityOverlay: "reachability-overlay", // "unknown — remote unreachable"
  markAbandoned: "mark-abandoned", // POSTs /api/runs/:id/abandon
  promptPanel: "prompt-panel", // data-kind: choice|clarify|freetext; data-prompt-id
  promptChoiceApply: "prompt-choice-apply",
  promptChoiceRefine: "prompt-choice-refine",
  promptChoiceExit: "prompt-choice-exit",
  promptAnswerInput: "prompt-answer-input", // single-line clarify answer (a number, A3)
  promptAnswerSubmit: "prompt-answer-submit",
  // Clarify numbered options (AI specs): one chip per option, carrying
  // data-index (1-based) with text = the exact option string. A3 reads
  // the option text at the index it answers, then asserts the collector
  // recorded that OPTION'S TEXT.
  promptClarifyOption: "prompt-clarify-option", // + data-index
  // Freetext refinement feedback (A2): a MULTILINE <textarea>, distinct
  // from the single-line clarify input, so the multiline transport
  // (incl. the terminating blank line the remote appends) is exercised.
  promptFreetextInput: "prompt-freetext-input",
  promptFreetextSubmit: "prompt-freetext-submit",
} as const;

// ── Dynamic testids ──

/** Inventory chip for a document/directory/checklist key. */
export function inventoryDocTestId(key: string): string {
  return `inventory-doc-${key}`;
}

/** Story row, keyed by the position-derived create id `<epic>.<pos>`. */
export function storyRowTestId(createId: string): string {
  return `story-row-${createId}`;
}

/** Clarify option chip at a 1-based index (AI spec A3). */
export function clarifyOption(page: Page, index: number): Locator {
  return page.locator(`[data-testid="${TID.promptClarifyOption}"][data-index="${index}"]`);
}

// ── Navigation ──

export const routes = {
  sessions: "/",
  session: (sessionId: string) => `/sessions/${sessionId}`,
  run: (runId: string) => `/runs/${runId}`,
};

export async function gotoSessions(page: Page, baseURL: string): Promise<void> {
  await page.goto(new URL(routes.sessions, baseURL).href);
}

export async function gotoSession(page: Page, baseURL: string, sessionId: string): Promise<void> {
  await page.goto(new URL(routes.session(sessionId), baseURL).href);
}

export async function gotoRun(page: Page, baseURL: string, runId: string): Promise<void> {
  await page.goto(new URL(routes.run(runId), baseURL).href);
}

// ── Locator helpers ──

export function sessionRow(page: Page, sessionId: string): Locator {
  return page.locator(`[data-testid="${TID.sessionRow}"][data-session-id="${sessionId}"]`);
}

export function inventoryDoc(page: Page, key: string): Locator {
  return page.getByTestId(inventoryDocTestId(key));
}

export function epicRow(page: Page, epicFile: string): Locator {
  return page.locator(`[data-testid="${TID.epicRow}"][data-epic-file="${epicFile}"]`);
}

export function storyRow(page: Page, createId: string): Locator {
  return page.getByTestId(storyRowTestId(createId));
}

export function runRow(page: Page, runId: string): Locator {
  return page.locator(`[data-testid="${TID.runRow}"][data-run-id="${runId}"]`);
}
