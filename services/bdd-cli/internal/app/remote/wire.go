package remote

import (
	"encoding/json"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/inventory"
)

// This file defines the v2 register / poll / reply wire DTOs (plan §2) the
// remote speaks to the STATELESS RELAY, plus the browser-facing projection
// shapes, which must match tests/legacy/bdd-web-playwright/helpers/api-client.ts exactly.

// ── Agent protocol (plan §2) ──

// registerRequest is the body of POST /api/agent/register.
type registerRequest struct {
	SessionID       string `json:"session_id"`
	Folder          string `json:"folder"`
	CanonicalFolder string `json:"canonical_folder"`
	PID             int    `json:"pid"`
	StartIdentity   string `json:"start_identity"`
	Version         string `json:"version"`
}

// registerResult is the reply to a successful register: the negotiated
// connection epoch, reply byte budget, capability token, and the
// inventory-fit budget — distinct from the reply cap: a tiny one degrades the snapshot, not 413s reads.
type registerResult struct {
	ConnectionEpoch      int    `json:"connection_epoch"`
	ReplyBudgetBytes     int    `json:"reply_budget_bytes"`
	CapabilityToken      string `json:"capability_token"`
	InventoryBudgetBytes int    `json:"inventory_budget_bytes"`
}

// pollRequest is the body of POST /api/agent/poll — the correlation triple.
type pollRequest struct {
	SessionID       string `json:"session_id"`
	ConnectionEpoch int    `json:"connection_epoch"`
	CapabilityToken string `json:"capability_token"`
}

// workItem is one dispatched unit of work the relay hands the CLI on a 200
// poll (plan §2). Payload is the type-specific body (query / dispatch /
// answer), decoded lazily.
type workItem struct {
	WorkID          string          `json:"work_id"`
	SessionID       string          `json:"session_id"`
	ConnectionEpoch int             `json:"connection_epoch"`
	Type            string          `json:"type"`
	Payload         json.RawMessage `json:"payload"`
	Deadline        int64           `json:"deadline"`
}

// replyEnvelope is the body of POST /api/agent/reply: an HTTP status plus the
// JSON result the relay returns to the awaiting browser (plan §2). The
// correlation triple travels in headers, OUTSIDE this capped body.
type replyEnvelope struct {
	Status int `json:"status"`
	Body   any `json:"body"`
}

// Work item types (plan §2; doc_tree/doc_read/doc_write/chat are plan Slice
// 0/5 — the workspace document + chat surface, off the run machinery).
const (
	workQuery    = "query"
	workDispatch = "dispatch"
	workAnswer   = "answer"
	workDocTree  = "doc_tree"
	workDocRead  = "doc_read"
	workDocWrite = "doc_write"
	workChat     = "chat"
)

// queryPayload is the body of a `query` work item (plan §3): the view to
// project, plus the session and (for run_detail) run id.
type queryPayload struct {
	View      string `json:"view"`
	SessionID string `json:"session_id"`
	RunID     string `json:"run_id"`
}

// Query views (plan §3).
const (
	viewSessionStatus = "session_status"
	viewSessionDetail = "session_detail"
	viewRunDetail     = "run_detail"
)

// dispatchPayload is the body of a `dispatch` work item (plan §3).
type dispatchPayload struct {
	SessionID   string `json:"session_id"`
	Command     string `json:"command"`
	StoryID     string `json:"story_id"`
	Fix         bool   `json:"fix"`
	ClientToken string `json:"client_token"`
}

// answerPayload is the body of an `answer` work item (plan §3).
type answerPayload struct {
	SessionID string `json:"session_id"`
	RunID     string `json:"run_id"`
	PromptID  string `json:"prompt_id"`
	Value     string `json:"value"`
}

// ── Browser-facing projection shapes (must match api-client.ts) ──

// RunSummary is one row of project run history (plan §3). Nullable columns
// (story_id, outcome, error_detail) are pointers so they serialize to JSON
// null, not omitted, matching the TS `string | null` contract.
type RunSummary struct {
	RunID       string  `json:"run_id"`
	OwnerID     string  `json:"owner_id"`
	Command     string  `json:"command"`
	StoryID     *string `json:"story_id"`
	Fix         bool    `json:"fix"`
	State       string  `json:"state"`
	Outcome     *string `json:"outcome"`
	ErrorDetail *string `json:"error_detail"`
	Answerable  bool    `json:"answerable"`
	CreatedAt   int64   `json:"created_at"`
	UpdatedAt   int64   `json:"updated_at"`
}

// ActiveOwner is a same-project active owner surfaced for the sibling warning
// (plan §3). SessionID is the owning live session id (owner_id IS the session
// id in v2); null when unknown.
type ActiveOwner struct {
	OwnerID   string  `json:"owner_id"`
	SessionID *string `json:"session_id"`
	RunID     string  `json:"run_id"`
	Command   string  `json:"command"`
}

// SessionStatus is the `session_status` projection (plan §3): active run,
// project run history newest-first, and same-project active owners — from ONE
// consistent CLI-side read.
type SessionStatus struct {
	SessionID    string        `json:"session_id"`
	OwnerID      string        `json:"owner_id"`
	ActiveRun    *RunSummary   `json:"active_run"`
	Runs         []RunSummary  `json:"runs"`
	ActiveOwners []ActiveOwner `json:"active_owners"`
}

// SessionDetail is the `session_detail` projection: SessionStatus PLUS the
// live inventory scan, in one consistent read (plan §3). SessionStatus is
// embedded anonymously so its fields flatten into the JSON object.
type SessionDetail struct {
	SessionStatus

	Inventory *inventory.Snapshot `json:"inventory"`
}

// TerminalEnvelope is the full diagnostic envelope beyond the outcome badge
// (plan §3.2 / finding 7).
type TerminalEnvelope struct {
	Classification *string `json:"classification"`
	EngineOutcome  *string `json:"engine_outcome"`
	FinalizationOK *bool   `json:"finalization_ok"`
	ExitCode       *int    `json:"exit_code"`
	Signal         *string `json:"signal"`
}

// PendingPrompt is the current unanswered prompt (plan §3).
type PendingPrompt struct {
	PromptID string `json:"prompt_id"`
	Kind     string `json:"kind"`
	Payload  any    `json:"payload"`
}

// RunDetail is the `run_detail` projection: a RunSummary plus the serving
// session id, the entire current capped event window, the pending prompt, and
// (once terminal) the diagnostic envelope (plan §3).
type RunDetail struct {
	RunSummary

	SessionID     string            `json:"session_id"`
	Events        []map[string]any  `json:"events"`
	PendingPrompt *PendingPrompt    `json:"pending_prompt"`
	Envelope      *TerminalEnvelope `json:"envelope"`
}

// RunSpec describes one dispatched command the remote executes as a child
// process. It is an internal value carried from the store run row to
// commandArgs (not a wire type in v2 — the store owns run identity).
type RunSpec struct {
	RunID   string
	Command string
	StoryID string
	Fix     bool
}
