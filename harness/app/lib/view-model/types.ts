/**
 * Wire types the browser consumes from the harness API (plan §3.3), plus
 * the inventory snapshot schema the Go scanner uploads (plan §3.4, the
 * binding contract in src/internal/app/inventory/snapshot.go). These are
 * the INPUT shapes to the pure view-model functions in this directory —
 * kept in one place so both the client views and the vitest tables (§4.6)
 * map the identical structures.
 */

export type Reachability = "connected" | "unreachable";

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

// ── Sessions ──

export interface SessionSummary {
  id: string;
  /** Canonical folder — realpath of the remote's cwd. */
  folder: string;
  pid: number;
  reachability: Reachability;
  active_run_id: string | null;
  inventory_generation: number;
  /** Wall-clock ms of the last promoted inventory; null until one exists. */
  inventory_updated_at: number | null;
  /**
   * The OUT-OF-BAND terminal cannot-fit reason (e.g. `limit_too_small`) the
   * remote reports on its poll when the server's inventory limit is below the
   * minimum viable request (plan §1a / finding 1). Null/absent normally.
   */
  inventory_unavailable?: string | null;
}

/** The full remote-synthesized terminal envelope (plan §3.2 / finding 7). */
export interface TerminalEnvelope {
  classification: RunOutcome | string | null;
  engine_outcome: string | null;
  finalization_ok: boolean | null;
  exit_code: number | null;
  signal: string | null;
}

export interface RunSummary {
  id: string;
  command: string;
  story_id: string | null;
  fix: boolean;
  state: RunState | string;
  outcome: RunOutcome | string | null;
  error_detail?: RunErrorDetail | string | null;
  /** The full terminal envelope; its fields are null until terminal. */
  envelope?: TerminalEnvelope;
  /** Wall-clock ms the run was dispatched. */
  created_at?: number;
  /** Wall-clock ms of the run's last activity (state change). */
  updated_at?: number;
}

export interface SessionDetail extends SessionSummary {
  runs: RunSummary[];
  /** The parsed inventory snapshot, or null until one is promoted. */
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
