package store

// Run lifecycle states (plan §1.1). The state column is the projection the
// dispatch admission and retention gate on.
const (
	stateQueued          = "queued"
	statePromptPublished = "prompt_published"
	stateAnswerAccepted  = "answer_accepted"
	stateTerminal        = "terminal"
)

// Event types — the generic per-run ledger (plan §1.2).
const (
	evtOutput       = "output"
	evtPrompt       = "prompt"
	evtLockAcquired = "lock_acquired"
	evtTerminal     = "terminal"
	evtGap          = "gap"
)

// Prompt kinds (plan §4 collector vocabulary).
const (
	kindChoice   = "choice"
	kindClarify  = "clarify"
	kindFreetext = "freetext"
)

// Dispatch outcome kinds (plan §1.4).
const (
	dispatchCreated  = "created"
	dispatchDeduped  = "deduped"
	dispatchConflict = "conflict"
	dispatchInvalid  = "invalid"
)

// classificationAbandoned is the boot-reconciliation terminal classification.
const classificationAbandoned = "abandoned"

// Scan/command lock outcomes (plan §1.5).
const (
	lockOK            = "ok"
	lockInventoryBusy = "inventory_busy"
	lockFolderLocked  = "folder_locked"
	lockTimeout       = "timeout"
)
