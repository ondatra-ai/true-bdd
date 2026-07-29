package remote

import "github.com/ondatra-ai/true-bdd/src/internal/app/inventory"

// This file defines the JSON wire DTOs for the harness RPC protocol
// (plan §3.3) shared by the ServerClient, RunExecutor, and Agent. The
// browser-facing shapes live in the Next.js server; these mirror only
// the agent-facing subset the remote speaks.

// RegisterRequest is the body of POST /api/agent/register.
type RegisterRequest struct {
	IncarnationID   string `json:"incarnation_id"`
	Folder          string `json:"folder"`
	CanonicalFolder string `json:"canonical_folder"`
	PID             int    `json:"pid"`
	Version         string `json:"version"`
}

// RegisterResponse is the reply to a successful register. InventoryLimitBytes
// is the server's effective inventory request byte budget (plan §1a /
// §r3-1): the remote derives its snapshot-fit budget from it and re-scans
// smaller on a 413. Zero (omitted) means the server did not negotiate a
// limit and the remote scans unbounded.
type RegisterResponse struct {
	SessionID           string `json:"session_id"`
	ScanEpoch           int    `json:"scan_epoch"`
	InventoryLimitBytes int    `json:"inventory_limit_bytes,omitempty"`
}

// PollRequest is the body of POST /api/agent/poll — the heartbeat. It also
// carries the OUT-OF-BAND inventory-unavailable signal (plan §1a / finding
// 1): when the server's negotiated inventory limit is below the minimum that
// can carry even the floor request, no inventory upload can ever be
// accepted, so the remote reports the terminal cannot-fit state here — on a
// channel that is not size-gated — rather than 413-looping forever.
type PollRequest struct {
	SessionID           string `json:"session_id"`
	ActiveRunID         string `json:"active_run_id,omitempty"`
	InventoryUnavailable string `json:"inventory_unavailable,omitempty"`
}

// PollResponse carries any dispatched run, a pending answer, and the
// current scan epoch.
type PollResponse struct {
	Run           *RunSpec   `json:"run,omitempty"`
	Answer        *AnswerMsg `json:"answer,omitempty"`
	WantInventory bool       `json:"want_inventory,omitempty"`
	ScanEpoch     int        `json:"scan_epoch,omitempty"`
}

// RunSpec describes one dispatched command the remote must execute.
type RunSpec struct {
	RunID   string `json:"run_id"`
	Command string `json:"command"`
	StoryID string `json:"story_id,omitempty"`
	Fix     bool   `json:"fix"`
}

// AnswerMsg is a browser-accepted answer relayed back to the child.
type AnswerMsg struct {
	PromptID  string `json:"prompt_id"`
	Value     string `json:"value"`
	AnswerSeq int    `json:"answer_seq"`
}

// EventsRequest posts a contiguous batch of remote-owned run events.
// AnswersConsumed is the highest answer_seq the remote has written to the
// child's stdin (memoized before the write — plan §3.3); the server
// confirms it back as answers_consumed_through so a lost answer is never
// re-delivered after it was consumed.
type EventsRequest struct {
	SessionID       string     `json:"session_id"`
	RunID           string     `json:"run_id"`
	Events          []OutEvent `json:"events"`
	AnswersConsumed int        `json:"answers_consumed,omitempty"`
}

// OutEvent is one remote-owned run event in the (run_id, seq) stream.
// The optional fields carry the payload for its `type`: output chunks
// use stream/data; prompts use prompt_id/kind/payload; the terminal
// event uses outcome/error_detail plus the FULL terminal envelope
// (engine_outcome / finalization_ok / exit_code / signal — plan §3.2 /
// finding 7); a `lock_acquired` event (plan §3.7 / finding 5) carries no
// payload — its arrival is the authoritative flock proof. A
// spool-truncation RANGE TOMBSTONE (`type:"gap"`) uses
// through_seq/dropped_bytes and covers the inclusive seq range
// [seq, through_seq] for contiguous ack (plan §3.6).
type OutEvent struct {
	Seq            int            `json:"seq"`
	Type           string         `json:"type"`
	Stream         string         `json:"stream,omitempty"`
	Data           string         `json:"data,omitempty"`
	PromptID       string         `json:"prompt_id,omitempty"`
	Kind           string         `json:"kind,omitempty"`
	Payload        map[string]any `json:"payload,omitempty"`
	Outcome        string         `json:"outcome,omitempty"`
	ErrorDetail    string         `json:"error_detail,omitempty"`
	EngineOutcome  string         `json:"engine_outcome,omitempty"`
	FinalizationOK *bool          `json:"finalization_ok,omitempty"`
	ExitCode       *int           `json:"exit_code,omitempty"`
	Signal         string         `json:"signal,omitempty"`
	ThroughSeq     int            `json:"through_seq,omitempty"`
	DroppedBytes   int            `json:"dropped_bytes,omitempty"`
}

// SpanEnd returns the highest seq this event covers: through_seq for a
// gap tombstone, otherwise its own seq. The server uses it to advance the
// contiguous acked_through watermark across truncated ranges.
func (e OutEvent) SpanEnd() int {
	if e.Type == eventTypeGap {
		return e.ThroughSeq
	}

	return e.Seq
}

// EventsResponse acknowledges a contiguous watermark and the answers
// the server has recorded as consumed.
type EventsResponse struct {
	AckedThrough           int `json:"acked_through"`
	AnswersConsumedThrough int `json:"answers_consumed_through"`
}

// InventoryRequest uploads a folder-scoped inventory snapshot.
type InventoryRequest struct {
	SessionID       string             `json:"session_id"`
	CanonicalFolder string             `json:"canonical_folder"`
	ScanEpoch       int                `json:"scan_epoch"`
	Snapshot        inventory.Snapshot `json:"snapshot"`
	ClientToken     string             `json:"client_token"`
}

// InventoryResponse returns the server-assigned generation.
type InventoryResponse struct {
	Generation int `json:"generation"`
}
