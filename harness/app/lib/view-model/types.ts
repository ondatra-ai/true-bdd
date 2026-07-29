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

// ── Inventory snapshot (plan §3.4) ──

export interface InventorySnapshot {
  documents: Record<string, string>;
  document_errors?: Record<string, string>;
  architecture_path_mismatch: boolean;
  configured_architecture_path?: string;
  canonical_architecture_path?: string;
  epics: InventoryEpic[];
}

export interface InventoryEpic {
  file: string;
  number: number;
  status: string;
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
