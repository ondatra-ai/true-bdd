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
