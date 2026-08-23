package store

import (
	"crypto/rand"
	"encoding/hex"
)

// The production DTOs. They are field-identical to the seam's test-only types
// so the test bridge adapts them with a single struct conversion; production
// code (remote, queryserver) uses these directly.

// DispatchInput mirrors the seam DispatchRequest.
type DispatchInput struct {
	OwnerID     string
	Command     string
	StoryID     string
	Fix         bool
	ClientToken string
	RequestHash string
}

// DispatchOutcome mirrors the seam DispatchResult. Kind ∈
// created | deduped | conflict | invalid | not_found.
type DispatchOutcome struct {
	Kind   string
	RunID  string
	Pruned bool
}

// EventInput mirrors the seam Event: the normalized event a producer submits.
type EventInput struct {
	Type         string
	Stream       string
	Data         string
	PromptID     string
	Kind         string
	Payload      string
	Outcome      string
	ErrorDetail  string
	ThroughSeq   int
	DroppedBytes int
	Control      bool
}

// AppendOutcome mirrors the seam AppendResult. Terminal=true is a LEGITIMATE
// reject (run already terminal/gone; safe to drop) vs Terminal=false, a
// TRANSIENT failure the caller must retry or fail closed on (finding 7).
type AppendOutcome struct {
	Seq      int
	Rejected bool
	Terminal bool
}

// RunRow mirrors the seam RunView.
type RunRow struct {
	RunID       string
	OwnerID     string
	State       string
	Outcome     string
	ErrorDetail string
	Answerable  bool
	Pruned      bool
}

// AnswerInput mirrors the seam AnswerRequest.
type AnswerInput struct {
	OwnerID  string
	RunID    string
	PromptID string
	Kind     string
	Value    string
}

// AnswerResolution is the atomic answer outcome: Outcome plus ShouldDeliver
// (false for an exact retry — see answerRetry in answer.go) and StoredKind,
// used to format/validate delivery from ONE consistent read.
type AnswerResolution struct {
	Outcome       AnswerOutcome
	ShouldDeliver bool
	StoredKind    string
}

// AnswerOutcome mirrors the seam AnswerResult.
type AnswerOutcome string

const (
	outcomeAccepted   AnswerOutcome = "accepted"
	outcomeConflict   AnswerOutcome = "conflict"
	outcomeInvalid    AnswerOutcome = "invalid"
	outcomeNotFound   AnswerOutcome = "not_found"
	outcomeCrossOwner AnswerOutcome = "cross_owner"
)

// PromptTimes mirrors the seam PromptLifecycle.
type PromptTimes struct {
	AcceptedAt        *int64
	DeliveryStartedAt *int64
	ConsumedAt        *int64
	Answer            string
	AnswerSeq         int
}

// PendingRun mirrors the seam NonterminalRun.
type PendingRun struct {
	RunID         string
	OwnerID       string
	OwnerPID      int
	OwnerIdentity string
	ChildPGID     int
	ChildIdentity string
}

// Reconciled mirrors the seam ReconcileResult.
type Reconciled struct {
	Abandoned  []string
	Terminated []int
	Skipped    []string
}

// LeaseHandle is the production lock lease; it satisfies the seam Lease. The
// unexported marker keeps it structurally distinct from the test-only Lease
// interface (both expose Release) while a *heldLease still satisfies both.
type LeaseHandle interface {
	Release()
	isLease()
}

// Dispatch command names — the typed allowlist entries (plan §2 / api-client
// COMMANDS).
const (
	cmdVersion     = "version"
	cmdUsCreate    = "us-create"
	cmdUsRefine    = "us-refine"
	cmdUsApply     = "us-apply"
	cmdBuildTests  = "build-tests"
	cmdBuildCode   = "build-code"
	cmdPromptProbe = "prompt-probe"
)

// allowedCommands is the typed dispatch allowlist (plan §2 / api-client COMMANDS); anything else is invalid.
//
//nolint:gochecknoglobals // the dispatch allowlist is a package-wide constant set
var allowedCommands = map[string]bool{
	cmdVersion:     true,
	cmdUsCreate:    true,
	cmdUsRefine:    true,
	cmdUsApply:     true,
	cmdBuildTests:  true,
	cmdBuildCode:   true,
	cmdPromptProbe: true,
}

// newID mints a random lowercase-hex identifier for runs.
func newID() string {
	var buf [16]byte

	_, err := rand.Read(buf[:])
	if err != nil {
		// crypto/rand failing is catastrophic; fall back to a timestamp.
		return hex.EncodeToString([]byte("fallback"))
	}

	return hex.EncodeToString(buf[:])
}
