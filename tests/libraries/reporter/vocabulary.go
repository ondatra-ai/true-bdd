package reporter

// The three checklist roles, plus the harness's own verdict call.
// roleJudge is not an engine turn — it is recovered from the test
// process's records — but it spends real money, so it is carried
// alongside the others everywhere costs are totalled.
const (
	rolePrompt = "prompt"
	roleFix    = "fix"
	roleApply  = "apply"
	roleJudge  = "judge"
)

// The three agent CLIs the engine can route a turn to.
const (
	cliClaude = "claude"
	cliCrush  = "crush"
	cliCodex  = "codex"
)

// The operation a turn performs, as the report names it. A role says
// which of the three seats in a checklist cell a turn occupied; a verb
// says what it did, which is the role plus whether the cell had been
// entered before. The distinction is the whole point: the first and the
// fourth turn of us-apply-rewalk-converges are both `prompt`, but one
// establishes a verdict and the other confirms a fix.
// The words are the ENGINE'S, not invented here: the fix turn is the one
// the log calls "Generating fix prompt" and the applier is the one that
// logs "Applying fix prompt". A reader who greps the log for a row's verb
// has to land on the record that produced it.
const (
	verbValidate   = "Validate"
	verbReValidate = "Re-validate"
	verbGenFix     = "Generate fix prompt"
	verbRegenFix   = "Regenerate fix prompt"
	verbApplyFix   = "Apply fix"
	verbReApplyFix = "Re-apply fix"
	verbJudge      = "Judge run"
)

// Why a turn happened. Every kind is recoverable from the run log; see
// classifyCauses.
const (
	// causeWalk is the first look at a cell, on the first walk.
	causeWalk = "walk"
	// causeReWalk is a cell re-entered because the previous walk applied
	// a fix SOMEWHERE — possibly to a different item, which is exactly
	// why the engine re-walks everything.
	causeReWalk = "rewalk"
	// causeAfterFix is a cell re-entered because THIS item was just
	// fixed and the walker restarted it at query 0.
	causeAfterFix = "afterfix"
	// causeCheckFailed is the first fix turn of a cell: the validation
	// answered fail.
	causeCheckFailed = "checkfailed"
	// causeClarify is a fix turn that follows answered clarifying
	// questions.
	causeClarify = "clarify"
	// causeRefine is a fix turn that follows user refinement feedback.
	causeRefine = "refine"
	// causeUserApply is an apply turn: the user picked [1].
	causeUserApply = "userapply"
	// causeJudge is the harness's own verdict call, not an engine turn.
	causeJudge = "judge"
)
