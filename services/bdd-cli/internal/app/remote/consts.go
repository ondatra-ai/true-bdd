package remote

// Scan/command lock outcome strings returned by store.Locks.BeginScan /
// BeginCommand (plan §1.5). Mirrored here so the executor and handlers switch
// on them without importing store internals.
const (
	lockOutcomeOK            = "ok"
	lockOutcomeInventoryBusy = "inventory_busy"
	lockOutcomeFolderLocked  = "folder_locked"
	lockOutcomeTimeout       = "timeout"
)

// stateTerminalName is the terminal run state string in the store schema.
const stateTerminalName = "terminal"

// reconcileLeaseTTLMillis bounds the durable boot-reconciliation lease (plan
// §1.6).
const reconcileLeaseTTLMillis = 30_000

// replyBudgetFloor is the smallest reply budget the remote will size an
// inventory scan against when the relay negotiates a small (or zero) cap.
const replyBudgetFloor = 64 * 1024
