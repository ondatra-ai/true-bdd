/**
 * UI navigation + selector contract for the harness views (plan §4).
 * The protocol specs assert against exactly these routes and
 * data-testid attributes; the UI phase is implemented to match. The
 * full contract, including data-* attribute vocabularies, is
 * documented in helpers/README-testids.md.
 *
 * v2 changes (plan §3/§4, critique §1/§2/§13):
 *   - Run pages are SESSION-SCOPED: `/sessions/<sid>/runs/<rid>` (the
 *     global `/runs/<id>` route is impossible without server state).
 *   - Sessions are GONE on disconnect: there is no `unreachable` state,
 *     no reachability overlay, and no mark-abandoned control.
 *   - There is no `inventory_generation`: the latest completed live
 *     inventory read is what renders; Refresh triggers an immediate live
 *     `session_detail` read (a fresh scan), not a mutation.
 *   - PROMPTS ARE NATIVE `<dialog>` MODALS (Peter's literal requirement):
 *     the inline PromptPanel is replaced by a `prompt-dialog` opened with
 *     showModal(), same pattern as the story modal.
 *
 * Live-update contract: all three views poll their API and re-render
 * without a manual reload — Playwright assertions rely on locator
 * auto-wait against a self-updating page. `usePoll` returns
 * `{data, status, error}`: on `404 session_gone` the page clears data and
 * navigates away; on `504 cli_timeout` it renders an explicit unavailable
 * state — stale data is never silently presented as current.
 */

import type { Locator, Page } from "@playwright/test";

// ── Static testids ──

export const TID = {
  // Sessions list / home (`/`) — one LIVE row per connected CLI session
  // (every listed session is connected by definition). The list stays live
  // without a manual reload: a new CLI appears and a stopped CLI vanishes on
  // the poll; an honest empty state shows only after a successful zero read;
  // a sustained read failure raises the unavailable notice (auto-returning on
  // recovery). Design baseline: the prototype `/sessions` MINUS Test connection
  // PLUS the empty state (README-testids → "Sessions list (`/`)").
  sessionsList: "sessions-list", // row-list container; exact-count row assertions scope to it
  sessionRow: "session-row", // one per session; + data-session-id, data-folder
  sessionFolder: "session-folder", // text = canonical folder (realpath)
  sessionMeta: "session-meta", // per-row meta line; CONTAINS the session id
  sessionVersion: "session-version", // text = remote version
  sessionOpen: "session-open", // per-row Open-workspace <a>; href = wsRoutes.home(sid)
  sessionsEmpty: "sessions-empty", // honest empty state (P6) — REPLACES the list on a zero read
  sessionsUnavailable: "sessions-unavailable", // P10 notice — sustained read-failure state
  // P5 NEGATIVE contract — markers production must NEVER render (sessions are
  // GONE on disconnect: no dead-session/reachability/reconnect affordance). Each
  // is asserted count 0 page-wide THROUGHOUT the disconnect window (w18.5).
  sessionDisconnected: "session-disconnected", // never rendered
  sessionUnreachable: "session-unreachable", // never rendered
  sessionReconnect: "session-reconnect", // never rendered

  // Session detail.
  refresh: "refresh", // triggers an immediate live session_detail READ (§1.5)
  unavailableState: "unavailable-state", // explicit 504 cli_timeout / disconnected view (§4)
  folderWarningBanner: "folder-warning-banner", // a same-project SIBLING owner is active
  pathMismatchWarning: "path-mismatch-warning", // non-default architecture path
  runHistory: "run-history",
  runRow: "run-row", // + data-run-id, data-command; link is session-scoped
  actionBuildTests: "action-build-tests", // <button>, disabled per plan §4
  actionBuildCode: "action-build-code", // <button>
  actionCreate: "action-create", // per story row
  actionRefine: "action-refine", // per story row
  actionApply: "action-apply", // per story row
  fixToggle: "fix-toggle",

  // Session detail: inventory truncation (plan §1.5 fit ladder — degraded snapshot).
  inventoryTruncatedBanner: "inventory-truncated-banner", // snapshot_truncated / any omission / limit_too_small

  // Session detail: epic accordion.
  epicSection: "epic-section", // wrapper; + data-epic-file, data-epic-number
  epicToggle: "epic-toggle", // expand control; aria-expanded; only when a story panel exists
  epicTitle: "epic-title", // text = Epic.Title (fallback: filename)
  epicRow: "epic-row", // + data-epic-file, data-epic-number, data-status
  epicFlagDuplicateNumber: "epic-flag-duplicate-number",
  epicFlagIdMismatch: "epic-flag-id-mismatch",
  epicFlagNoncanonicalFilename: "epic-flag-noncanonical-filename",

  // Session detail: story-row cells + flags.
  storyTitle: "story-title", // ALWAYS a button for declared rows; opens the modal; + data-match-count on ambiguous
  storyCreated: "story-created", // data-status: one|missing|ambiguous|invalid
  storyApplied: "story-applied", // text "x/y" or "unknown (<reason>)"
  storyRefined: "story-refined", // text "not recorded" in v2
  storyFlagDuplicateDeclaredId: "story-flag-duplicate-declared-id",
  storyFlagIdMismatch: "story-flag-id-mismatch",
  storyFlagDeprecatedFormat: "story-flag-deprecated-format",
  storyFlagNoAcs: "story-flag-no-acs",
  storyFlagEmptyInternalId: "story-flag-empty-internal-id",

  // Story review modal (native <dialog>).
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
  runEnvelopeEngineOutcome: "run-envelope-engine-outcome",
  runEnvelopeFinalization: "run-envelope-finalization",
  runEnvelopeExitCode: "run-envelope-exit-code",
  runEnvelopeSignal: "run-envelope-signal",

  // Prompt DIALOG (native <dialog>, plan §4 — PROMPTS BECOME DIALOGS).
  promptDialog: "prompt-dialog", // the <dialog>; data-kind ∈ choice|clarify|freetext; data-prompt-id
  promptDialogPanel: "prompt-dialog-panel", // inner panel; a click on IT must NOT answer/close
  promptDialogTitle: "prompt-dialog-title", // heading; aria-labelledby target
  promptDialogError: "prompt-dialog-error", // visible after a FAILED answer RPC; dialog stays open
  promptChoiceApply: "prompt-choice-apply",
  promptChoiceRefine: "prompt-choice-refine",
  promptChoiceExit: "prompt-choice-exit",
  promptAnswerInput: "prompt-answer-input", // single-line clarify answer (a number, A3)
  promptAnswerSubmit: "prompt-answer-submit",
  // Clarify numbered options (AI specs): one chip per option, carrying
  // data-index (1-based) with text = the exact option string.
  promptClarifyOption: "prompt-clarify-option", // + data-index
  // Freetext refinement feedback (A2): a MULTILINE <textarea>, distinct
  // from the single-line clarify input.
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

/** Epic accordion section, keyed by epic filename basename. */
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

/** The open prompt dialog (native <dialog>), optionally bound to a prompt id. */
export function promptDialog(page: Page, promptId?: string): Locator {
  const suffix = promptId === undefined ? "" : `[data-prompt-id="${promptId}"]`;

  return page.locator(`[data-testid="${TID.promptDialog}"]${suffix}`);
}

// ── Navigation ──

export const routes = {
  sessions: "/",
  session: (sessionId: string) => `/sessions/${sessionId}`,
  run: (sessionId: string, runId: string) => `/sessions/${sessionId}/runs/${runId}`,
};

export async function gotoSessions(page: Page, baseURL: string): Promise<void> {
  await page.goto(new URL(routes.sessions, baseURL).href);
}

export async function gotoSession(page: Page, baseURL: string, sessionId: string): Promise<void> {
  await page.goto(new URL(routes.session(sessionId), baseURL).href);
}

/** Session-scoped run view (critique §1 — the global run URL is gone). */
export async function gotoRun(page: Page, baseURL: string, sessionId: string, runId: string): Promise<void> {
  await page.goto(new URL(routes.run(sessionId, runId), baseURL).href);
}

// ── Locator helpers ──

export function sessionRow(page: Page, sessionId: string): Locator {
  return page.locator(`[data-testid="${TID.sessionRow}"][data-session-id="${sessionId}"]`);
}

/** The sessions-list container (row-list); exact-count row assertions scope to it. */
export function sessionsList(page: Page): Locator {
  return page.getByTestId(TID.sessionsList);
}

/**
 * A row's Open-workspace control (`session-open`), scoped to its row so the
 * per-row href → wsRoutes.home(sid) association is pinned (w18.3). Pass a row
 * Locator; passing the Page returns every open control on the list.
 */
export function sessionOpen(scope: Page | Locator): Locator {
  return scope.getByTestId(TID.sessionOpen);
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

// ═══════════════════════════════════════════════════════════════════════════
// Workspace file-as-source UI contract (the `w*` project + a10 assert against
// exactly these routes + testids; the coder implements the workspace to match).
// The human summary lives in helpers/README-testids.md → "Workspace UI".
//
// Section keys used by the icon rail + docked sidebar.
export type WorkspaceSection = "home" | "architecture" | "product" | "builds";

// ── Workspace static testids ──
export const WTID = {
  // App shell (100vh frame; the CONTENT pane owns the scroll — Established fact).
  appShell: "app-shell",
  workspaceMain: "workspace-main", // breadcrumb + content-pane column, right of the sidebar
  contentBreadcrumb: "content-breadcrumb", // persistent breadcrumb bar (design/SPEC.md §1 frame)
  contentPane: "content-pane",

  // Icon rail (narrow, dark, far-left) + hover flyout.
  rail: "rail", // + data-active-section
  railFlyout: "rail-flyout", // hover preview of a NON-active section's tree
  railUtilities: "rail-utilities", // utility items pinned at the rail BOTTOM (w7.1a)
  railUtilityItem: "rail-utility-item",
  // rail-item-<section> per section; each carries data-section and the active
  // marker (aria-current="page" when active). Use railItem(page, section).

  // Docked sidebar (section tree).
  sidebar: "sidebar",
  // sidebar-section-<section> per section tree. Use sidebarSection(page, section).
  sidebarGroup: "sidebar-group", // collapsible group; + data-group="<Label>"
  sidebarGroupName: "sidebar-group-name", // the NAME link in a group header (navigates)
  sidebarCaret: "sidebar-caret", // hover-revealed toggle; + data-expanded ("true"|"false"); glyph ▸/▾
  sidebarGroupBody: "sidebar-group-body", // the group's child-rows container (hidden when collapsed)
  sidebarGuideLine: "sidebar-guide-line", // thin child-indentation guide line (w7.1a)

  // Sidebar rows. Every navigable row carries data-selected ("true"|"false")
  // for the open-page highlight (P6) and a kind-specific id attribute.
  archServiceRow: "arch-service-row", // + data-service (Architecture › Services)
  archTermRow: "arch-term-row", // + data-term (Architecture › Terms)
  archDockerRow: "arch-docker-row", // the compose_file path (Architecture › Docker)
  prdRow: "prd-row", // Product › PRD entry
  featureRow: "feature-row", // Product › Features row; + data-feature
  storyRow: "story-row", // Product › Stories row; + data-story-id
  scenarioRow: "scenario-row", // Product › Scenarios row; + data-scenario-id

  // GitHub-style file view.
  fileView: "file-view",
  fileViewPath: "file-view-path", // text = the docs/ path
  fileViewGutter: "file-view-gutter",
  fileViewGutterLine: "file-view-gutter-line", // one per content line
  fileViewEditor: "file-view-editor", // the edit-in-place surface (monospace exception)
  fileViewFlash: "file-view-flash", // exact-line jump flash; + data-line
  yamlInvalidIndicator: "yaml-invalid-indicator",
  saveState: "save-state", // + data-save-state (idle|saving|saved|invalid|conflict|error), + data-revision

  // Architecture per-service derived details region (on the file page).
  archServiceDetails: "arch-service-details", // + data-service
  serviceTech: "service-tech", // + data-tech (descendant of arch-service-details)
  serviceEndpoint: "service-endpoint", // + data-method, data-path (custom services)
  serviceConnection: "service-connection", // + data-key (supporting services)
  serviceDockerProvenance: "service-docker-provenance", // + data-kind (dockerfile|compose_ref)

  // Feature aggregation (derived page) + unaligned bucket.
  featureDescription: "feature-description",
  featureStoriesList: "feature-stories-list",
  featureScenariosList: "feature-scenarios-list",
  featureStoryRow: "feature-story-row", // + data-story-id
  featureScenarioRow: "feature-scenario-row", // + data-scenario-id
  unalignedBucket: "unaligned-bucket",
  unalignedScenarioRow: "unaligned-scenario-row", // + data-scenario-id, data-dangling ("true"|"false"), data-dangling-ref

  // Searchable feature picker (reused: new-story form, a story/scenario row, the
  // unaligned bucket). Multiple pickers on a page are disambiguated by SCOPING
  // to their container (row/form) — the sub-testids are shared, not per-id.
  featurePicker: "feature-picker",
  featurePickerToggle: "feature-picker-toggle", // collapsed pill → opens the searchable dropdown
  featurePickerInput: "feature-picker-input",
  featurePickerOption: "feature-picker-option", // + data-feature (existing feature)
  featurePickerCreate: "feature-picker-create", // "+ Create <query>" (inline new feature)

  // New-story form (P22).
  newStoryOpen: "new-story-open", // opens the form
  newStoryForm: "new-story-form",
  newStoryTitle: "new-story-title",
  newStorySubmit: "new-story-submit",

  // Docked chat (P10/P11/P12).
  chatDock: "chat-dock",
  chatDockToggle: "chat-dock-toggle", // edge tab (collapsed) / open control
  chatDockPanel: "chat-dock-panel",
  chatDockResizer: "chat-dock-resizer",
  chatDockHeader: "chat-dock-header",
  chatDockNew: "chat-dock-new", // new-chat control in the header (w7.1a)
  chatDockHistory: "chat-dock-history",
  chatDockMessage: "chat-dock-message", // + data-role (user|assistant)
  chatDockInput: "chat-dock-input",
  chatDockSend: "chat-dock-send",

  // Section landings.
  homeLanding: "home-landing",
  buildsLanding: "builds-landing",

  // Workspace-overview canvas (Home landing design-parity surfaces — the `/home`
  // canvas mirrors the prototype's /workspace-overview page; the icon rail +
  // docked sidebar chrome is UNCHANGED). See README-testids → "Workspace overview".
  overviewTitle: "overview-title", // h1 "Workspace overview"
  overviewMeta: "overview-meta", // metadata line beneath the title: folder path + session id
  overviewActions: "overview-actions", // the build-actions row wrapping the three buttons
  overviewActionBuildTests: "overview-action-build-tests", // <button>; dispatches command "build-tests"
  overviewActionBuildCode: "overview-action-build-code", // <button>; dispatches command "build-code"
  overviewActionRefresh: "overview-action-refresh", // <button>; triggers an immediate session_detail READ
  overviewInventory: "overview-inventory", // the inventory-health list container (from SessionDetail.inventory)
  overviewInventoryRow: "overview-inventory-row", // one per inventory entry; + data-key; carries path + kind label + chip
  overviewInventoryChip: "overview-inventory-chip", // status chip; + data-status ∈ INVENTORY_STATUSES
  overviewBanner: "overview-banner", // bordered degraded-state banner; + data-kind ∈ OVERVIEW_BANNER_KINDS; absent states render nothing

  // ── Product-section prototype-parity additions (w12–w15). Additive to the
  // contract above; documented in README-testids.md → "Product prototype parity".
  //
  // Rail item anatomy (w12.2): each rail-item-<section> nests an icon glyph +
  // a small-caps FULL-word label (icon above/before the label).
  railItemIcon: "rail-item-icon", // non-empty glyph element inside a rail item
  railItemLabel: "rail-item-label", // full-word label (Home/Architecture/Product/Builds); uppercase transform + --ls-label

  // Branded sidebar (w12.1): brand header + workspace-context meta + an
  // underlined per-section header.
  sidebarBrand: "sidebar-brand", // brand header; text contains "TrueBDD"
  sidebarBrandMeta: "sidebar-brand-meta", // distinct workspace-context meta line (non-empty)
  sidebarSectionHeader: "sidebar-section-header", // underlined "02—PRODUCT" header inside the section

  // GitHub-style file-view page header + card header bar (w13.1/w13.2/w13.3).
  fileViewKicker: "file-view-kicker", // "02—PRODUCT[…]" kicker, structurally ABOVE the title
  fileViewTitle: "file-view-title", // display title (e.g. "prd.yaml" / "<id> — <story title>")
  fileViewMeta: "file-view-meta", // muted subtitle, BELOW the title; color = --text-muted
  fileViewHeader: "file-view-header", // the card header bar; CONTAINS the path + line-count
  fileViewLineCount: "file-view-line-count", // "N lines" counter; N == buffer.split("\n").length

  // Story feature-pill anatomy (w13.2): the collapsed toggle nests three parts.
  featurePillLabel: "feature-pill-label", // text "Feature:"
  featurePillValue: "feature-pill-value", // current feature value (or "(none)")
  featurePillChange: "feature-pill-change", // the CHANGE affordance; text matches /change/i

  // Feature-detail page kicker (w14.1).
  featurePageKicker: "feature-page-kicker", // "02—PRODUCT / FEATURES / <NAME>"

  // Scenarios registry TABLE (w13.4 — orchestrator TABLE ruling, overrides the
  // FileView reading). Columns SCENARIO(link)/DESCRIPTION/SERVICE/LINKED STORY(link),
  // one row per docs/scenarios.yaml entry. NOTE: distinct from the sidebar's
  // `scenario-row` so the two never collide on the scenarios page.
  scenarioTable: "scenario-table", // the <table> element (thead has the 4 column headers)
  scenarioTableRow: "scenario-table-row", // one per scenario; + data-scenario-id
  scenarioIdLink: "scenario-id-link", // SCENARIO-column link; text = scenario id
  scenarioDescriptionCell: "scenario-description-cell", // DESCRIPTION cell text
  scenarioServiceCell: "scenario-service-cell", // SERVICE cell text
  scenarioLinkedStoryLink: "scenario-linked-story-link", // LINKED STORY-column link; text = story id, href → story route

  // ── Workspace design-scale fidelity additions (w16). Additive; documented in
  // README-testids.md → "Workspace design scale".
  scenarioRowService: "scenario-row-service", // muted service annotation inside a sidebar scenario-row (prototype `.sidebar-row-service`)
  breadcrumbSep: "breadcrumb-sep", // dedicated "/" separator element between breadcrumb crumbs (prototype `.crumb-sep`)
} as const;

/** The status vocabulary an inventory-health chip may carry (mockup + README). */
export const INVENTORY_STATUSES = [
  "present",
  "present_empty",
  "missing",
  "not_a_dir",
  "ambiguous",
  "invalid",
  "unknown",
] as const;
export type InventoryStatus = (typeof INVENTORY_STATUSES)[number];

/** The degraded/flagged banner kinds the overview may surface (mockup §05). */
export const OVERVIEW_BANNER_KINDS = ["inventory_truncated", "folder_conflict", "path_mismatch"] as const;
export type OverviewBannerKind = (typeof OVERVIEW_BANNER_KINDS)[number];

// ── Workspace dynamic testids ──
export function railItemTestId(section: WorkspaceSection): string {
  return `rail-item-${section}`;
}
export function sidebarSectionTestId(section: WorkspaceSection): string {
  return `sidebar-section-${section}`;
}

// ── Workspace navigation (session-scoped; the `(workspace)` route group is
// elided from the URL, consistent with Next App Router). ──
export const wsRoutes = {
  sessions: "/",
  home: (sid: string) => `/sessions/${sid}/home`,
  architecture: (sid: string) => `/sessions/${sid}/architecture`,
  product: (sid: string) => `/sessions/${sid}/product`,
  features: (sid: string) => `/sessions/${sid}/product/features`,
  feature: (sid: string, id: string) => `/sessions/${sid}/product/features/${id}`,
  story: (sid: string, storyId: string) => `/sessions/${sid}/product/stories/${storyId}`,
  scenarios: (sid: string) => `/sessions/${sid}/product/scenarios`,
  builds: (sid: string) => `/sessions/${sid}/builds`,
} as const;

export async function gotoWorkspace(page: Page, baseURL: string, route: string): Promise<void> {
  await page.goto(new URL(route, baseURL).href);
}

// ── Workspace locator helpers ──
export function railItem(page: Page, section: WorkspaceSection): Locator {
  return page.getByTestId(railItemTestId(section));
}

export function sidebarSection(page: Page, section: WorkspaceSection): Locator {
  return page.getByTestId(sidebarSectionTestId(section));
}

/** A sidebar group (Services / Terms / Docker / PRD / Features / Stories / Scenarios). */
export function sidebarGroup(scope: Page | Locator, label: string): Locator {
  return scope.locator(`[data-testid="${WTID.sidebarGroup}"][data-group="${label}"]`);
}

/** The caret toggle inside a group header. */
export function sidebarCaret(group: Locator): Locator {
  return group.getByTestId(WTID.sidebarCaret);
}

export function archServiceRow(scope: Page | Locator, service: string): Locator {
  return scope.locator(`[data-testid="${WTID.archServiceRow}"][data-service="${service}"]`);
}

/** The file-page derived details region for one service (endpoints/tech/etc. are its descendants). */
export function archServiceDetails(page: Page, service: string): Locator {
  return page.locator(`[data-testid="${WTID.archServiceDetails}"][data-service="${service}"]`);
}

export function sidebarStoryRow(scope: Page | Locator, storyId: string): Locator {
  return scope.locator(`[data-testid="${WTID.storyRow}"][data-story-id="${storyId}"]`);
}

export function sidebarScenarioRow(scope: Page | Locator, scenarioId: string): Locator {
  return scope.locator(`[data-testid="${WTID.scenarioRow}"][data-scenario-id="${scenarioId}"]`);
}

export function sidebarFeatureRow(scope: Page | Locator, feature: string): Locator {
  return scope.locator(`[data-testid="${WTID.featureRow}"][data-feature="${feature}"]`);
}

export function fileView(page: Page): Locator {
  return page.getByTestId(WTID.fileView);
}

// ── Workspace-overview locator helpers ──

/** An inventory-health row on the overview, keyed by its inventory `data-key`. */
export function overviewInventoryRow(scope: Page | Locator, key: string): Locator {
  return scope.locator(`[data-testid="${WTID.overviewInventoryRow}"][data-key="${key}"]`);
}

/** The status chip inside an inventory-health row (or scoped to the list). */
export function overviewInventoryChip(scope: Page | Locator): Locator {
  return scope.getByTestId(WTID.overviewInventoryChip);
}

/** A degraded-state banner on the overview, optionally bound to a `data-kind`. */
export function overviewBanner(page: Page, kind?: OverviewBannerKind): Locator {
  const suffix = kind === undefined ? "" : `[data-kind="${kind}"]`;

  return page.locator(`[data-testid="${WTID.overviewBanner}"]${suffix}`);
}

export function saveState(page: Page): Locator {
  return page.getByTestId(WTID.saveState);
}

// ── Workspace UI action helpers ──

/** Reads the file-view editor buffer (textarea value or contenteditable text). */
export function readEditor(editor: Locator): Promise<string> {
  return editor.evaluate((el) => {
    const value = (el as HTMLTextAreaElement).value;

    return typeof value === "string" ? value : (el.textContent ?? "");
  });
}

/** Replaces the whole editor buffer (a live edit; autosave debounces a doc_write). */
export async function writeEditor(editor: Locator, text: string): Promise<void> {
  await editor.fill(text);
}

/** Sends a chat message through the docked chat input. */
export async function sendChat(page: Page, text: string): Promise<void> {
  await page.getByTestId(WTID.chatDockInput).fill(text);
  await page.getByTestId(WTID.chatDockSend).click();
}

/** Picks an EXISTING feature via the searchable picker, scoped to its container. */
export async function pickFeatureIn(scope: Page | Locator, feature: string): Promise<void> {
  await scope.getByTestId(WTID.featurePickerToggle).click();
  await scope.getByTestId(WTID.featurePickerInput).fill(feature);
  await scope.locator(`[data-testid="${WTID.featurePickerOption}"][data-feature="${feature}"]`).click();
}

/** Types a novel query and clicks "+ Create" to inline-create a feature. */
export async function createFeatureIn(scope: Page | Locator, query: string): Promise<void> {
  await scope.getByTestId(WTID.featurePickerToggle).click();
  await scope.getByTestId(WTID.featurePickerInput).fill(query);
  await scope.getByTestId(WTID.featurePickerCreate).click();
}

/** Resolves a CSS color literal (hex / var value) to its computed `rgb(...)` string. */
export function toRgb(page: Page, color: string): Promise<string> {
  return page.evaluate((c) => {
    const probe = document.createElement("span");
    probe.style.color = c;
    document.body.appendChild(probe);
    const rgb = getComputedStyle(probe).color;
    probe.remove();

    return rgb;
  }, color);
}

/** 0-based index of the first buffer line matching `re` (for exact-line jump oracles). */
export function lineIndex(content: string, re: RegExp): number {
  const index = content.split("\n").findIndex((line) => re.test(line));
  if (index < 0) {
    throw new Error(`lineIndex: no line matched ${re}`);
  }

  return index;
}

// ── Product-section prototype-parity locator helpers (w12–w15) ──

/** The icon glyph element inside a rail item (w12.2). */
export function railItemIcon(page: Page, section: WorkspaceSection): Locator {
  return railItem(page, section).getByTestId(WTID.railItemIcon);
}

/** The full-word label element inside a rail item (w12.2). */
export function railItemLabel(page: Page, section: WorkspaceSection): Locator {
  return railItem(page, section).getByTestId(WTID.railItemLabel);
}

/** A scenarios-registry table row scoped to a container, keyed by `data-scenario-id` (w13.4). */
export function scenarioTableRow(scope: Page | Locator, scenarioId: string): Locator {
  return scope.locator(`[data-testid="${WTID.scenarioTableRow}"][data-scenario-id="${scenarioId}"]`);
}

/**
 * Resolves a `:root` CSS custom property to its computed `rgb(...)` form, so a
 * token-linked colour assertion (e.g. the active rail tile = `--surface-page`)
 * reads the SAME resolved value the browser paints. Mirrors w7.1's discipline.
 */
export async function cssVarRgb(page: Page, varName: string): Promise<string> {
  const raw = (
    await page.evaluate((name) => getComputedStyle(document.documentElement).getPropertyValue(name), varName)
  ).trim();
  if (raw === "") {
    throw new Error(`cssVarRgb: :root ${varName} is empty (token mirror not consumed)`);
  }

  return toRgb(page, raw);
}
