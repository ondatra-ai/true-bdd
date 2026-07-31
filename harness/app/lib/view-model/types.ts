/**
 * Wire types the browser consumes from the harness API (v2 — plan §2/§3;
 * the binding contract is `tests/e2e/helpers/api-client.ts`), plus the
 * inventory snapshot schema the Go scanner produces (plan §1.5). These are
 * the INPUT shapes to the pure view-model functions in this directory —
 * kept in one place so both the client views and the vitest tables (§4.6)
 * map the identical structures.
 *
 * v2 (plan §1/§2/§3): the relay holds NO state. Sessions are registry-only
 * (every listed session is connected — no reachability, no active_run_id, no
 * generation). Run/session state comes from the per-project CLI store; the
 * status/detail views carry the active run, project run history, and the
 * same-project active owners (for the sibling warning). Run routes are
 * session-scoped.
 */

export type RunState =
  | "queued"
  | "claimed"
  | "running"
  | "prompt_published"
  | "answer_accepted"
  | "answer_consumed"
  | "terminal";

export type RunOutcome =
  | "ok"
  | "converged"
  | "not_fixed"
  | "user_exit"
  | "max_attempts"
  | "interrupted"
  | "abandoned"
  | "error";

export type RunErrorDetail = "spawn" | "no_result" | "contradiction" | "folder_locked";

export type PromptKind = "choice" | "clarify" | "freetext";

// ── Sessions (v2 registry-only summary + CLI-store status/detail) ──

/**
 * Registry-only session summary (plan §3): every listed session is CONNECTED
 * by definition. `session_id` is CLIENT-OWNED and process-stable (the
 * incarnation id); a relay restart does not change it.
 */
export interface SessionSummary {
  session_id: string;
  /** Canonical folder — realpath of the remote's cwd. */
  folder: string;
  /** The remote process's pid (disambiguates same-folder sessions). */
  pid: number;
  version: string;
  /** Wall-clock ms of the register that opened this connection. */
  connected_at: number;
}

/** The full terminal envelope (diagnostics beyond the badge). */
export interface TerminalEnvelope {
  classification: RunOutcome | null;
  engine_outcome: string | null;
  finalization_ok: boolean | null;
  exit_code: number | null;
  signal: string | null;
}

export interface RunSummary {
  run_id: string;
  /** The owning CLI incarnation (distinct from the serving session). */
  owner_id: string;
  command: string;
  story_id: string | null;
  fix: boolean;
  state: RunState;
  /** null until state === "terminal". */
  outcome: RunOutcome | null;
  error_detail: RunErrorDetail | null;
  /**
   * Cross-owner guard (plan §1.1): true only when this run's owner is the
   * serving live session AND a local active executor/prompt exists.
   */
  answerable: boolean;
  created_at: number;
  updated_at: number;
}

/** A same-project active owner, surfaced for the sibling warning (§3). */
export interface ActiveOwner {
  owner_id: string;
  /** The owning live session, when it is currently connected. */
  session_id: string | null;
  run_id: string;
  command: string;
}

/**
 * One `session_status` CLI work item (plan §3): active run, project run
 * history, and same-project active owners — from ONE SQLite snapshot.
 */
export interface SessionStatus {
  session_id: string;
  owner_id: string;
  active_run: RunSummary | null;
  /** Project run history, newest first (latest N). */
  runs: RunSummary[];
  /** Same-project active owners (drives the sibling warning). */
  active_owners: ActiveOwner[];
}

/**
 * One `session_detail` CLI work item (plan §3): `session_status` PLUS the
 * inventory, in one CLI-side consistent read.
 */
export interface SessionDetail extends SessionStatus {
  /** The parsed inventory snapshot, or null when unavailable. */
  inventory: InventorySnapshot | null;
}

// ── Runs ──

export interface RunEvent {
  seq: number;
  type: string;
  stream?: string;
  data?: string;
  prompt_id?: string;
  kind?: string;
  payload?: unknown;
  outcome?: string;
  error_detail?: string;
  through_seq?: number;
  dropped_bytes?: number;
  [key: string]: unknown;
}

export interface PendingPrompt {
  prompt_id: string;
  kind: PromptKind | string;
  payload: unknown;
}

export interface RunDetail extends RunSummary {
  session_id: string;
  events: RunEvent[];
  pending_prompt: PendingPrompt | null;
  /** The full terminal envelope; null until terminal. */
  envelope: TerminalEnvelope | null;
}

// ── Inventory snapshot (plan §3.4 / §1 hierarchy + review-modal enrichment) ──

export interface InventorySnapshot {
  documents: Record<string, string>;
  document_errors?: Record<string, string>;
  architecture_path_mismatch: boolean;
  configured_architecture_path?: string;
  canonical_architecture_path?: string;
  epics: InventoryEpic[];
  /** True global counts, surviving epic-entry truncation (plan §1a). */
  totals?: InventoryTotals;
  /** The snapshot was degraded to fit the request budget (plan §1a). */
  snapshot_truncated?: boolean;
  /** Story rows dropped to fit the budget. */
  stories_omitted?: number;
  /**
   * The server inventory cap was below the minimum viable request — no
   * snapshot could fit (plan §1.5 fit ladder). Renders the limit-too-small
   * banner; global counts only.
   */
  limit_too_small?: boolean;
  /** Session-level cannot-fit state, e.g. `limit_too_small` (plan §1a). */
  unavailable?: string;
}

export interface InventoryTotals {
  epics: number;
  declared_stories: number;
}

export interface InventoryEpic {
  file: string;
  number: number;
  status: string;
  /** The epic document's `name:`; empty when unparseable (plan §1). */
  title?: string;
  doc_id: number;
  id_mismatch: boolean;
  duplicate_number: boolean;
  /**
   * True when the epic filename is not the canonical `epic-%02d-*` encoding
   * `us create` resolves (finding 4). Such an epic carries NO
   * Create-addressable story rows, so its `stories` list is empty.
   */
  noncanonical_filename?: boolean;
  error?: string;
  stories: InventoryStory[];
}

// ── Review-modal content (plan §1; extracted by the Go typed decoder) ──

export interface InventoryStep {
  kind: "given" | "when" | "then" | "and" | "but" | string;
  text: string;
}

export interface InventoryContentAc {
  id: string;
  description: string;
  steps: InventoryStep[];
}

export interface InventoryStatement {
  as_a: string;
  i_want: string;
  so_that: string;
}

export interface InventoryContent {
  id: string;
  title: string;
  status: string;
  statement: InventoryStatement;
  acceptance_criteria: InventoryContentAc[];
}

export interface InventoryStory {
  create_id: string;
  epic_number: number;
  position: number;
  declared_id: string;
  file_id?: string;
  created: string;
  applied: InventoryApplied;
  refined: string;
  flags: InventoryStoryFlags;
  /** Epic-declared title/status, available even when the file is absent. */
  declared_title?: string;
  declared_status?: string;
  /** Number of `<declared-id>-*.yaml` matches (drives the ambiguous view). */
  match_count?: number;
  /** Resolved story-file basename (only when exactly one match). */
  source_file?: string;
  /** Stable parse/validation error text (invalid stories). */
  error?: string;
  /** Parsed file content (Review tab); null/absent when not created=one. */
  content?: InventoryContent | null;
  /** Epic-declared content fallback so every declared row is openable. */
  declared_content?: InventoryContent | null;
  /** Verbatim story file for the Raw tab (+ raw_truncated). */
  raw?: string;
  raw_truncated?: boolean;
  /** Omission metadata from the request-budget degrade ladder (plan §1a). */
  raw_omitted?: boolean;
  content_omitted?: boolean;
  omission_reason?: string;
}

export interface InventoryApplied {
  status: string;
  applied?: number;
  total?: number;
  reason?: string;
}

export interface InventoryStoryFlags {
  duplicate_declared_id: boolean;
  id_mismatch: boolean;
  deprecated_format: boolean;
  no_acs: boolean;
  empty_internal_id: boolean;
}
