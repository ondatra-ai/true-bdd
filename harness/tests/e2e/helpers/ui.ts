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

  // Session detail: inventory truncation (plan §1a/§3 — degraded snapshot).
  inventoryTruncatedBanner: "inventory-truncated-banner", // visible when snapshot_truncated / any omission count > 0

  // Session detail: epic accordion (hierarchy rework, plan §3).
  epicSection: "epic-section", // wrapper; + data-epic-file, data-epic-number
  epicToggle: "epic-toggle", // expand control; aria-expanded; rendered ONLY when a story panel exists
  epicTitle: "epic-title", // text = Epic.Title (fallback: filename)
  epicRow: "epic-row", // + data-epic-file, data-epic-number, data-status
  epicFlagDuplicateNumber: "epic-flag-duplicate-number",
  epicFlagIdMismatch: "epic-flag-id-mismatch",
  epicFlagNoncanonicalFilename: "epic-flag-noncanonical-filename", // no toggle / no rows for such an epic

  // Session detail: story-row cells + flags.
  storyTitle: "story-title", // ALWAYS a button for declared rows; opens the modal; + data-match-count on ambiguous
  storyCreated: "story-created", // data-status: one|missing|ambiguous|invalid
  storyApplied: "story-applied", // text "x/y" or "unknown (<reason>)"
  storyRefined: "story-refined", // text "not recorded" in v1
  storyFlagDuplicateDeclaredId: "story-flag-duplicate-declared-id",
  storyFlagIdMismatch: "story-flag-id-mismatch",
  storyFlagDeprecatedFormat: "story-flag-deprecated-format",
  storyFlagNoAcs: "story-flag-no-acs",
  storyFlagEmptyInternalId: "story-flag-empty-internal-id",

  // Story review modal (native <dialog>, plan §3 "Story review modal").
  storyModal: "story-modal", // the <dialog> opened with showModal(); + data-story-id
  storyModalPanel: "story-modal-panel", // the single inner panel — a click on IT must NOT close the modal
  storyModalTitle: "story-modal-title", // heading; aria-labelledby target; accessible name = story id + title
  storyModalClose: "story-modal-close", // labelled close button
  storyModalStatus: "story-modal-status", // status chip (declared_status fallback)
  storyModalCreated: "story-modal-created", // lifecycle chip
  storyModalApplied: "story-modal-applied", // lifecycle chip
  storyModalRefined: "story-modal-refined", // lifecycle chip
  storyModalIdentity: "story-modal-identity", // identity line (declared / file ids)
  storyModalTablist: "story-modal-tablist", // role=tablist
  storyModalTabReview: "story-modal-tab-review", // role=tab, aria-selected; default
  storyModalTabRaw: "story-modal-tab-raw", // role=tab, aria-selected
  storyModalPanelReview: "story-modal-panel-review", // role=tabpanel
  storyModalPanelRaw: "story-modal-panel-raw", // role=tabpanel
  storyModalStatement: "story-modal-statement", // as-a / i-want / so-that block
  storyModalAc: "story-modal-ac", // per AC; + data-ac-id
  storyModalStep: "story-modal-step", // per step; + data-kind ∈ given|when|then|and|but; text = exact step text
  storyModalRaw: "story-modal-raw", // verbatim file in a scrollable <pre> (+ truncation/omission notice)
  storyModalError: "story-modal-error", // parse-error banner (invalid story)
  storyModalNotice: "story-modal-notice", // availability/omission notice; + data-reason

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

/** Epic accordion section, keyed by epic filename basename (plan §3). */
export function epicSection(page: Page, epicFile: string): Locator {
  return page.locator(`[data-testid="${TID.epicSection}"][data-epic-file="${epicFile}"]`);
}

/** The expand/collapse toggle inside an epic section (present only when a story panel exists). */
export function epicToggle(section: Locator): Locator {
  return section.getByTestId(TID.epicToggle);
}

/** The epic title cell inside an epic section. */
export function epicTitle(section: Locator): Locator {
  return section.getByTestId(TID.epicTitle);
}

/**
 * A story row scoped to a container (an epic section OR the page). Scoping
 * is REQUIRED where duplicate epic numbers make the position-derived
 * create id collide across sections (P4 duplicate-epic case).
 */
export function storyRowIn(scope: Page | Locator, createId: string): Locator {
  return scope.getByTestId(storyRowTestId(createId));
}

/** The story-title opener button for a story row (scoped to a container). */
export function storyTitle(scope: Page | Locator, createId: string): Locator {
  return storyRowIn(scope, createId).getByTestId(TID.storyTitle);
}

/** The open story review modal (native <dialog>). */
export function storyModal(page: Page): Locator {
  return page.getByTestId(TID.storyModal);
}

/** A modal AC block keyed by its ac id (data-ac-id). */
export function storyModalAc(page: Page, acId: string): Locator {
  return page.locator(`[data-testid="${TID.storyModalAc}"][data-ac-id="${acId}"]`);
}

/** Modal step(s) of a given kind (data-kind ∈ given|when|then|and|but). */
export function storyModalStep(page: Page, kind: string): Locator {
  return page.locator(`[data-testid="${TID.storyModalStep}"][data-kind="${kind}"]`);
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
