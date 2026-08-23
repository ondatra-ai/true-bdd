// Package engine drives the (items × checklist) walk that every
// `us` subcommand performs. The four callbacks supplied by the caller
// describe the per-cell work; the engine orchestrates iteration, the
// fix-and-restart rule within an item, the outer fixpoint re-walk
// across items, and the interactive clarify/apply/refine/exit UX.
package engine

import "context"

// GenerateQFn renders one raw checklist item into the query value `Q`
// that `Query` and `GenerateFix` consume. Must be pure in `idx`/`p`:
// the engine renders each prompt once and reuses it across all items.
type GenerateQFn[P, Q any] func(idx int, p P) Q

// QueryFn evaluates one (item, query) cell. Returns true iff the
// cell passes. Closures may capture state shared with `GenerateFix`
// (e.g. the underlying validation result) — see per-command wiring.
type QueryFn[I, Q any] func(ctx context.Context, item I, q Q) (bool, error)

// GenerateFixFn is the per-cell, per-iteration fix-prompt generator
// the clarify/refine loop calls. iteration is 1-based and continues
// past MaxClarificationIterations, so callers can tell refinement from clarification by the number alone.
type GenerateFixFn[I, Q any] func(
	ctx context.Context,
	item I,
	q Q,
	userAnswers map[string]string,
	iteration int,
) (FixResult, error)

// FixFn applies an approved `FixDecision` to `item` and returns the
// post-fix item: a value item (e.g. Story) returns the new version;
// a reference item mutated externally returns the same item.
type FixFn[I any] func(ctx context.Context, item I, d FixDecision) (I, error)

// Action is the engine-internal user choice in the fix loop. Public
// so per-command code can read what the engine decided.
type Action int

const (
	// ActionApply means the engine ran Fix with the contained fix prompt.
	ActionApply Action = iota
	// ActionExit means the engine treated the cell as CellUserExited
	// without calling Fix.
	ActionExit
)

// FixDecision carries the approved fix prompt that Fix is invoked
// with. Always Action=ActionApply when Fix is called (ActionExit
// short-circuits earlier).
type FixDecision struct {
	Action    Action
	FixPrompt string
}

// ClarifyQuestion is one question the fix-prompt generator wants the
// user to answer before producing a concrete fix prompt. Mirrors the
// shape the existing FixPromptGenerator already emits.
type ClarifyQuestion struct {
	ID       string
	Question string
	Context  string
	Options  []string
}

// FixResult is one iteration of fix-prompt generation: either a
// concrete FixPrompt (success) or a set of clarifying Questions; see
// CellHandler.runClarificationLoop for how the engine loops on it.
type FixResult struct {
	FixPrompt string
	Questions []ClarifyQuestion
}

// HasFixPrompt is true when the generator produced a concrete prompt.
func (r FixResult) HasFixPrompt() bool { return r.FixPrompt != "" }

// HasQuestions is true when the generator needs more user input.
func (r FixResult) HasQuestions() bool { return len(r.Questions) > 0 }

// UserAction is the user's choice in the apply/refine/exit prompt.
type UserAction int

const (
	// UserActionApply means apply the current fix prompt.
	UserActionApply UserAction = iota
	// UserActionRefine means ask the user for feedback and regenerate.
	UserActionRefine
	// UserActionExit means abort this cell without applying.
	UserActionExit
)

// FixLoopUI is the UI surface the engine's fix loop drives. Any
// implementation is acceptable; the true-bdd's UserInputCollector +
// console.Display* helpers satisfy this in production.
type FixLoopUI interface {
	AskQuestions(questions []ClarifyQuestion) map[string]string
	AskApplyRefineOrExit() UserAction
	AskRefinementFeedback() string
	DisplayFixPrompt(prompt string)
}

// StopReason describes why `Engine.Run` returned.
type StopReason int

const (
	// Converged means every cell passed in a walk that applied no fixes.
	Converged StopReason = iota
	// UserExit means a cell's fix loop ended with UserActionExit.
	UserExit
	// NotFixed means at least one cell failed and no fix was applied
	// (e.g. non-fix mode, or fix-mode where GenerateFix produced no
	// applicable fix prompt within MaxClarificationIterations).
	NotFixed
	// MaxAttemptsExhausted means the fixpoint re-walk ran the configured
	// number of attempts without converging.
	MaxAttemptsExhausted
)

// CellOutcome is the verdict the `CellHandler` returns to its walker.
type CellOutcome int

const (
	// CellPassed means `Query` returned true.
	CellPassed CellOutcome = iota
	// CellFailedNoFix means `Query` returned false and either fix mode
	// was off, the clarification loop exhausted without a prompt, or
	// the user picked refine past the cap.
	CellFailedNoFix
	// CellFixed means `Fix` ran; the walker should restart the prompt
	// loop for this item.
	CellFixed
	// CellUserExited means the user chose UserActionExit in the fix
	// loop.
	CellUserExited
)

// CellResult bundles the verdict and the (possibly updated) item.
type CellResult[I any] struct {
	Outcome CellOutcome
	Item    I
}

// ItemRun is the per-item summary the walker hands back to the engine.
type ItemRun[I any] struct {
	Item       I
	Passed     bool
	FixApplied bool
	UserExited bool
}

// Options tunes engine behaviour.
type Options struct {
	// MaxApplyAttempts bounds the outer fixpoint re-walk (0 → default 5,
	// from `config.max_apply_attempts`). The runner also passes it to
	// SequentialWalker.MaxFixes — both loops need a bound of their own.
	MaxApplyAttempts int
	// OnAttemptStart, if non-nil, fires at the top of each outer-walk
	// attempt (1-indexed); the runner uses it for the "RE-WALK N/M"
	// banner. The engine itself stays console-free.
	OnAttemptStart func(attempt, maxAttempts int)
	// OnItemStart, if non-nil, fires before each per-item walk within an
	// attempt; the runner uses it for a per-item banner (e.g. "AC N/M:
	// <description>"). Single-item commands leave it nil.
	OnItemStart func(idx, total int)
}

// Result is what `Engine.Run` returns to the caller.
type Result[I any] struct {
	// Items is the final item slice. For commands whose Fix returns a
	// new value (create / refine), this holds the latest version.
	Items []I
	// AllPassed is true iff the engine returned with Reason==Converged.
	AllPassed bool
	// Reason explains how the engine terminated.
	Reason StopReason
}

const (
	defaultMaxApplyAttempts    = 5
	maxClarificationIterations = 5
	maxRefinementIterations    = 3
	// refinementMagicKey is the userAnswers slot the apply-fix-generator
	// template reads to inject user refinement feedback.
	refinementMagicKey = "_user_refinement"
)
