/**
 * Pure view-model for the session-detail inventory surface (plan §3.4 /
 * §3.5). Maps the Go scanner's InventorySnapshot into render-ready props
 * for the document chips, epics table, and per-story rows — WITHOUT any
 * React, so the vitest tables (§4.6) exercise every inventory state class
 * against the identical mapping the UI renders.
 *
 * The data-testid + data-* vocabulary rendered from these props is the
 * binding contract in helpers/README-testids.md.
 */

import type {
  InventoryApplied,
  InventoryContent,
  InventoryEpic,
  InventorySnapshot,
  InventoryStory,
  InventoryStoryFlags,
} from "./types";

/**
 * The canonical chip order (plan §3.4 / README-testids). The scanner
 * always emits every key; rendering in this fixed order keeps the layout
 * stable. Any unexpected extra key is appended after these.
 */
export const DOCUMENT_KEYS = [
  "config",
  "prd",
  "architecture",
  "registry",
  "stories-dir",
  "epics-dir",
  "checklist-us-create",
  "checklist-us-refine",
  "checklist-us-apply",
  "checklist-build-tests",
  "checklist-build-code",
] as const;

export interface DocumentChip {
  key: string;
  status: string;
  error?: string;
}

/** Ordered document/checklist chips with their status + optional error. */
export function documentChips(snapshot: InventorySnapshot | null): DocumentChip[] {
  if (!snapshot) {
    return [];
  }

  const documents = snapshot.documents ?? {};
  const errors = snapshot.document_errors ?? {};
  const ordered: string[] = [
    ...DOCUMENT_KEYS.filter((key) => key in documents),
    ...Object.keys(documents).filter((key) => !(DOCUMENT_KEYS as readonly string[]).includes(key)),
  ];

  return ordered.map((key) => {
    const chip: DocumentChip = { key, status: documents[key] };
    if (errors[key]) {
      chip.error = errors[key];
    }

    return chip;
  });
}

/** True when the configured architecture path diverges from the default. */
export function pathMismatch(snapshot: InventorySnapshot | null): boolean {
  return snapshot?.architecture_path_mismatch === true;
}

/**
 * Maps a document/story/epic status to an S&F status TONE for its chip
 * (colour lives only in chips): ok (green square), error (red), warn
 * (yellow), or neutral (monochrome). Purely presentational; the tests key
 * on `data-status`, never the colour.
 */
export type StatusTone = "ok" | "error" | "warn" | "neutral";

export function statusTone(status: string): StatusTone {
  switch (status) {
    case "present":
    case "one":
    case "counted":
    case "parseable":
    case "connected":
      return "ok";
    case "invalid":
    case "not_a_dir":
    case "error":
      return "error";
    case "ambiguous":
    case "unknown":
    case "present_empty":
    case "unreachable":
      return "warn";
    default:
      return "neutral";
  }
}

// ── Epics ──

export interface EpicRowView {
  file: string;
  number: number;
  status: string;
  flagDuplicateNumber: boolean;
  flagIdMismatch: boolean;
  flagNoncanonicalFilename: boolean;
  error?: string;
  stories: StoryRowView[];
}

export function epicRows(snapshot: InventorySnapshot | null): EpicRowView[] {
  if (!snapshot) {
    return [];
  }

  return (snapshot.epics ?? []).map(epicRow);
}

export function epicRow(epic: InventoryEpic): EpicRowView {
  return {
    file: epic.file,
    number: epic.number,
    status: epic.status,
    flagDuplicateNumber: epic.duplicate_number === true,
    flagIdMismatch: epic.id_mismatch === true,
    flagNoncanonicalFilename: epic.noncanonical_filename === true,
    error: epic.error || undefined,
    stories: (epic.stories ?? []).map(storyRow),
  };
}

// ── Stories ──

export interface AppliedCellView {
  /** Exact rendered text: `x/y` (counted) or `unknown (<reason>)`. */
  text: string;
  status: string;
  applied?: number;
  total?: number;
  reason?: string;
}

export interface StoryFlagView {
  /** The flag chip's data-testid. */
  testid: string;
}

export interface StoryRowView {
  createId: string;
  epicNumber: number;
  position: number;
  declaredId: string;
  fileId: string;
  created: string;
  applied: AppliedCellView;
  refined: string;
  flags: StoryFlagView[];
}

export function storyRow(story: InventoryStory): StoryRowView {
  return {
    createId: story.create_id,
    epicNumber: story.epic_number,
    position: story.position,
    declaredId: story.declared_id,
    fileId: story.file_id ?? "",
    created: story.created,
    applied: appliedCell(story.applied),
    refined: refinedText(story.refined),
    flags: storyFlags(story.flags),
  };
}

/**
 * The story-applied cell (README-testids): a countable `x/y` with
 * data-status="counted", or `unknown (<reason>)` with
 * data-status="unknown" carrying the concrete reason.
 */
export function appliedCell(applied: InventoryApplied): AppliedCellView {
  if (applied.status === "counted") {
    const appliedCount = applied.applied ?? 0;
    const total = applied.total ?? 0;

    return { text: `${appliedCount}/${total}`, status: "counted", applied: appliedCount, total };
  }

  const reason = applied.reason ?? "";

  return { text: `unknown (${reason})`, status: "unknown", reason };
}

/**
 * The refined cell. In v1 the scanner only ever reports "not_recorded",
 * which renders as the literal text "not recorded" (README-testids). Any
 * other value passes through so a future refined state is not masked.
 */
export function refinedText(refined: string): string {
  return refined === "not_recorded" ? "not recorded" : refined;
}

/** The flag→testid map (README-testids). Only active flags are rendered. */
const FLAG_TESTIDS: Record<keyof InventoryStoryFlags, string> = {
  duplicate_declared_id: "story-flag-duplicate-declared-id",
  id_mismatch: "story-flag-id-mismatch",
  deprecated_format: "story-flag-deprecated-format",
  no_acs: "story-flag-no-acs",
  empty_internal_id: "story-flag-empty-internal-id",
};

/** The flag chips visible for a story (only those whose condition holds). */
export function storyFlags(flags: InventoryStoryFlags): StoryFlagView[] {
  const active: StoryFlagView[] = [];
  for (const key of Object.keys(FLAG_TESTIDS) as (keyof InventoryStoryFlags)[]) {
    if (flags?.[key]) {
      active.push({ testid: FLAG_TESTIDS[key] });
    }
  }

  return active;
}

// ── Per-action dispatch identifiers (plan §3.4 / handoff) ──

export type StoryAction = "create" | "refine" | "apply";

export interface StoryDispatch {
  command: "us-create" | "us-refine" | "us-apply";
  story_id: string;
}

/**
 * The (command, story_id) a per-story action dispatches:
 *   create → us-create with the position-derived create id (`42.1`);
 *   refine/apply → us-refine/us-apply with the epic-declared id (`77.5`).
 * (A1 asserts Create sends `42.1`; A2/A4 assert Refine/Apply send `77.5`.)
 */
export function storyDispatch(story: StoryRowView, action: StoryAction): StoryDispatch {
  switch (action) {
    case "create":
      return { command: "us-create", story_id: story.createId };
    case "refine":
      return { command: "us-refine", story_id: story.declaredId };
    case "apply":
      return { command: "us-apply", story_id: story.declaredId };
  }
}

// ── Epic accordion sections (hierarchy rework, plan §3) ──

export interface EpicSectionView {
  file: string;
  number: number;
  /** Epic.Title, falling back to the filename when unparseable/empty. */
  title: string;
  status: string;
  /** A toggle is rendered ONLY when a story panel exists (has rows). */
  hasPanel: boolean;
  /** Absence from the collapsed set = expanded (default); new epics expand. */
  collapsed: boolean;
  stories: { createId: string }[];
}

/**
 * Epic sections in SCANNER ORDER (no re-sort) with the panel-existence rule
 * and the collapsed-set behavior (absence = expanded; a new epic
 * auto-expands). The UI keys on `(epic.file, position)`, so duplicate epic
 * numbers never collide (plan §3).
 */
export function epicSections(snapshot: InventorySnapshot | null, collapsed?: Set<string>): EpicSectionView[] {
  if (!snapshot) {
    return [];
  }

  return (snapshot.epics ?? []).map((epic) => {
    const stories = epic.stories ?? [];

    return {
      file: epic.file,
      number: epic.number,
      title: epic.title && epic.title.length > 0 ? epic.title : epic.file,
      status: epic.status,
      hasPanel: stories.length > 0,
      collapsed: collapsed?.has(epic.file) ?? false,
      stories: stories.map((story) => ({ createId: story.create_id })),
    };
  });
}

// ── Story review modal model (native <dialog>, plan §3) ──

export type ModalSource = "file" | "declared" | "identity";

export interface ModalStatement {
  asA: string;
  iWant: string;
  soThat: string;
}

export interface ModalAc {
  id: string;
  description: string;
  steps: { kind: string; text: string }[];
}

export interface ModalNotice {
  reason: string;
}

export interface StoryModalView {
  storyId: string;
  title: string;
  status: string;
  source: ModalSource;
  defaultTab: "review" | "raw";
  errorMode: boolean;
  review: {
    enabled: boolean;
    statement: ModalStatement | null;
    acceptanceCriteria: ModalAc[];
  };
  raw: { available: boolean; text: string; truncated: boolean };
  chips: { created: string; applied: { text: string; status: string }; refined: string };
  notices: ModalNotice[];
  changedOnDisk: boolean;
}

/**
 * The render-ready story review modal (plan §3, README-testids). It resolves
 * the file→declared→identity content fallback chain (so every declared row
 * is openable), the invalid-story Raw+error mode with a declared-content
 * Review fallback (Review is enabled IFF declared content exists), the
 * lifecycle chips, the statement/AC/step normalization, and every
 * availability/omission/freshness notice + the changed-on-disk signal.
 */
export function storyModalModel(
  story: InventoryStory,
  _epic: InventoryEpic,
  flags?: { changedOnDisk?: boolean },
): StoryModalView {
  const reviewContent: InventoryContent | null = story.content ?? story.declared_content ?? null;
  const source: ModalSource = story.content ? "file" : story.declared_content ? "declared" : "identity";
  const errorMode = story.created === "invalid";

  const rawText = story.raw ?? "";
  const rawAvailable = rawText.length > 0 && story.raw_omitted !== true;

  const status =
    story.content?.status || story.declared_status || story.declared_content?.status || "unknown";
  // Heading id/title fallback chain (finding 5a): file-content id/title →
  // declared id/title → create identity / "unknown". data-story-id stays the
  // create id (selector compatibility) — see StoryModal.
  const storyId = story.content?.id || story.declared_id || story.create_id || "unknown";
  const title = story.content?.title || story.declared_title || story.declared_content?.title || "unknown";

  return {
    storyId,
    title,
    status,
    source,
    defaultTab: errorMode ? "raw" : "review",
    errorMode,
    review: {
      enabled: reviewContent !== null,
      statement: reviewContent ? statementOf(reviewContent) : null,
      acceptanceCriteria: reviewContent ? reviewContent.acceptance_criteria.map(modalAc) : [],
    },
    raw: { available: rawAvailable, text: rawText, truncated: story.raw_truncated === true },
    chips: {
      created: story.created,
      applied: appliedChip(story.applied),
      refined: refinedText(story.refined),
    },
    notices: modalNotices(story, errorMode, rawAvailable, flags?.changedOnDisk === true),
    changedOnDisk: flags?.changedOnDisk === true,
  };
}

function statementOf(content: InventoryContent): ModalStatement {
  return {
    asA: content.statement?.as_a ?? "",
    iWant: content.statement?.i_want ?? "",
    soThat: content.statement?.so_that ?? "",
  };
}

function modalAc(ac: InventoryContent["acceptance_criteria"][number]): ModalAc {
  return {
    id: ac.id,
    description: ac.description,
    steps: (ac.steps ?? []).map((step) => ({ kind: step.kind, text: step.text })),
  };
}

function appliedChip(applied: InventoryApplied): { text: string; status: string } {
  const cell = appliedCell(applied);

  return { text: cell.text, status: cell.status };
}

/**
 * Every availability / omission / freshness notice the modal surfaces
 * (README-testids data-reason vocabulary). Availability (not_created /
 * ambiguous) reflects the created state; the omission notices reflect the
 * degrade ladder; invalid_without_raw is the terminal invalid-and-no-raw
 * state; changed_on_disk is the generation-change signal.
 */
function modalNotices(
  story: InventoryStory,
  errorMode: boolean,
  rawAvailable: boolean,
  changedOnDisk: boolean,
): ModalNotice[] {
  const notices: ModalNotice[] = [];

  if (story.created === "missing") {
    notices.push({ reason: "not_created" });
  } else if (story.created === "ambiguous") {
    notices.push({ reason: "ambiguous" });
  }

  if (errorMode && !rawAvailable) {
    notices.push({ reason: "invalid_without_raw" });
  } else if (story.raw_omitted && story.content_omitted) {
    notices.push({ reason: "both_omitted" });
  } else if (story.content_omitted) {
    notices.push({ reason: "content_omitted" });
  } else if (story.raw_omitted) {
    notices.push({ reason: "raw_omitted" });
  }

  if (story.raw_truncated) {
    notices.push({ reason: "raw_truncated" });
  }

  if (changedOnDisk) {
    notices.push({ reason: "changed_on_disk" });
  }

  return notices;
}

// ── Inventory truncation banner (plan §1a/§3) ──

/**
 * Whether the degraded-snapshot banner shows: the promoted snapshot was
 * truncated to fit the request budget, any story row lost its raw/content,
 * or the whole snapshot is unavailable (limit_too_small).
 */
export function inventoryTruncated(snapshot: InventorySnapshot | null): boolean {
  if (!snapshot) {
    return false;
  }

  if (
    snapshot.snapshot_truncated === true ||
    snapshot.limit_too_small === true ||
    (snapshot.unavailable ?? "").length > 0
  ) {
    return true;
  }

  if ((snapshot.stories_omitted ?? 0) > 0) {
    return true;
  }

  return (snapshot.epics ?? []).some((epic) =>
    (epic.stories ?? []).some((story) => story.raw_omitted === true || story.content_omitted === true),
  );
}

/** True when the fit ladder collapsed to global counts only (§1.5). */
export function inventoryLimitTooSmall(snapshot: InventorySnapshot | null): boolean {
  return snapshot?.limit_too_small === true || (snapshot?.unavailable ?? "").length > 0;
}
