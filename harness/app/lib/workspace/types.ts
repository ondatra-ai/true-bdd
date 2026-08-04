/**
 * Shared workspace types (plan: workspace-file-as-source-ui). Mirrors the
 * CLI's wire shapes (`src/internal/app/remote/docs.go`, `chat.go`) and the
 * browser API contract in `tests/harness/helpers/README-testids.md`.
 */

export type WorkspaceSection = "home" | "architecture" | "product" | "builds";

/** One doc_tree manifest entry. */
export interface DocTreeEntry {
  path: string;
  exists: boolean;
  revision: string;
}

export type ParseStatus = "ok" | "invalid" | "missing";

/** The reply to a `doc_read` (browser: GET .../docs/read?path=). */
export interface DocReadResult {
  content: string;
  exists: boolean;
  parse_status: ParseStatus;
  revision: string;
}

/** The CLI's write receipt — the S1 proof. */
export interface WriteReceipt {
  path: string;
  committed_revision: string;
  content_hash: string;
  work_id?: string;
  bytes?: number;
}

export type SaveState = "idle" | "saving" | "saved" | "invalid" | "conflict" | "error";

/** Client-side per-document editing state held by FilesProvider. */
export interface DocState {
  /** The live editing buffer — reflects every keystroke/chat edit immediately. */
  buffer: string;
  /** The last buffer that parsed as YAML — outline/derived views use this
   * when the current buffer is invalid (P10b). */
  lastValidContent: string;
  /** The revision this doc is synced to (the base_revision the next write
   * sends): the initial doc_read revision, or the last successful write's
   * committed_revision. */
  syncedRevision: string;
  saveState: SaveState;
  exists: boolean;
}

export interface ChatMessage {
  role: "user" | "assistant";
  content: string;
}

export interface ChatEdit {
  path: string;
  new_content: string;
}

export interface ChatTurnResult {
  reply_text: string;
  edit: ChatEdit | null;
  error?: string;
}

/** A pending cross-page (or same-page) outline jump. */
export interface PendingJump {
  path: string;
  line: number;
}

// ── Fixed document paths (mirrors src/internal/app/remote/docs.go) ──

export const ARCHITECTURE_PATH = "docs/architecture/architecture.yaml";
export const PRD_PATH = "docs/prd/prd.yaml";
export const FEATURES_PATH = "docs/prd/features.yaml";
export const SCENARIOS_PATH = "docs/scenarios.yaml";

export function isStoryPath(path: string): boolean {
  return path.startsWith("docs/prd/stories/") && path.endsWith(".yaml");
}
