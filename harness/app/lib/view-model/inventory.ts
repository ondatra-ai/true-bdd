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
