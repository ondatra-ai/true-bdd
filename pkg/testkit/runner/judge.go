package runner

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/cli/claude"
	"github.com/ondatra-ai/true-bdd/pkg/enginelog"
)

// ErrJudgeMalformedResponse is returned when the call produced no
// schema-conforming verdict set — no payload, unparseable JSON, or one
// verdict per clause not accounted for.
var ErrJudgeMalformedResponse = errors.New("judge returned no schema-conforming verdict")

// ErrJudgeCLI marks a verdict session the CLI itself reported as failed.
var ErrJudgeCLI = errors.New("judge cli reported an error")

// JudgeRequest is the input to a single judge call.
type JudgeRequest struct {
	Cmd string
	// Clauses are the scenario's `judge:` clauses, in registry order.
	// One call carries all of them: a verdict is taken on the run as a
	// whole, and every clause reads the same diff.
	Clauses []string
	Diff    []FileChange
}

// JudgeOutcome is one judge call: its decision and the exact text it
// was made of — the only evidence for why two runs disagree. Returned,
// not written here, so Judge stays pure and the caller owns all I/O.
type JudgeOutcome struct {
	Pass   bool
	Reason string
	// Model is the pinned id this verdict was taken on, carried so the
	// harness record names the substrate rather than a floating alias.
	Model string
	// InputHash fingerprints the rendered user prompt. Same hash with a
	// different verdict is judge noise; a different hash is harness drift.
	InputHash string
	// SystemPrompt, UserPrompt and Response are empty for a call that
	// failed before the model answered.
	SystemPrompt string
	UserPrompt   string
	Response     string
}

// Judge evaluates whether a fixture run satisfied its judge.md spec.
type Judge interface {
	Verdict(ctx context.Context, req JudgeRequest) (JudgeOutcome, error)
}

// ClaudeJudge takes verdicts through pkg/cli/claude: a schema-constrained,
// tool-less headless turn on a pinned model, so a verdict is a function of
// its prompt and nothing else.
type ClaudeJudge struct {
	model string
	// replayed marks a judge whose turns are served from a cassette. Its
	// answers carry the recorded envelope's cost, which was spent once, at
	// recording time — billing it again would report money no run spent.
	replayed bool
}

// NewClaudeJudge pins the verdict model from the engine config. An
// unresolvable tier kills suite boot — never a silent fallback model.
func NewClaudeJudge(configPath, testsMode string) (*ClaudeJudge, error) {
	model, err := JudgeModel(configPath)
	if err != nil {
		return nil, err
	}

	return &ClaudeJudge{model: model, replayed: testsMode == ProxyModeReplay}, nil
}

const judgeSystemPrompt = `You are an automated test verdict judge. You will be given:
1. A CLI command that was run.
2. An English specification of what the command was expected to do.
3. A diff of the files that changed during the run.

Rule on each numbered rule independently, using ONLY the material in
this prompt. Do not use tools: everything relevant is included here.

For every rule return "pass", or "fail" with a one-sentence reason
grounded in the diff. A rule fails only if the material shown violates
it. Return one entry per rule, and nothing else.`

// judgeSchema constrains the answer to a verdict per clause. One entry
// per rule is what scales to many clauses: a disagreement names its rule
// instead of arriving as one holistic sentence.
const judgeSchema = `{
  "type": "object",
  "properties": {
    "verdicts": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "rule": {"type": "integer"},
          "verdict": {"type": "string", "enum": ["pass", "fail"]},
          "reason": {"type": "string"}
        },
        "required": ["rule", "verdict", "reason"],
        "additionalProperties": false
      }
    }
  },
  "required": ["verdicts"],
  "additionalProperties": false
}`

// judgeDisallowedTools strips the session of every way to reach state
// outside its prompt. Observed, not theoretical: an unrestricted judge
// prompt ran `cat` on its own initiative.
func judgeDisallowedTools() []string {
	return []string{
		"Bash", "Read", "Write", "Edit", "Glob", "Grep",
		"WebSearch", "WebFetch", "Task", "TodoWrite", "NotebookEdit",
	}
}

// clauseVerdict is one numbered rule's ruling.
type clauseVerdict struct {
	Rule    int    `json:"rule"`
	Verdict string `json:"verdict"`
	Reason  string `json:"reason"`
}

// judgeVerdicts is the schema's top-level object.
type judgeVerdicts struct {
	Verdicts []clauseVerdict `json:"verdicts"`
}

const verdictPass = "pass"

// Verdict implements Judge by taking a schema-constrained ruling.
func (j *ClaudeJudge) Verdict(ctx context.Context, req JudgeRequest) (JudgeOutcome, error) {
	user := buildJudgeUserPrompt(req)

	// Carried on the outcome even down the error paths: a malformed reply
	// is exactly when someone needs to read what was asked and answered.
	outcome := JudgeOutcome{
		SystemPrompt: judgeSystemPrompt,
		UserPrompt:   user,
		Model:        j.model,
		InputHash:    fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(user))),
	}

	payload, err := j.ask(ctx, user)
	if err != nil {
		return outcome, err
	}

	outcome.Response = payload

	pass, reason, err := decodeJudgeVerdicts(payload, len(req.Clauses))
	if err != nil {
		return outcome, err
	}

	outcome.Pass = pass
	outcome.Reason = reason

	return outcome, nil
}

// ask runs the verdict turn and returns the raw verdict JSON. One headless
// `claude -p` through pkg/cli, not the streaming transport: a verdict is a
// prompt in and one answer out, and this package now imports no root.
func (j *ClaudeJudge) ask(ctx context.Context, user string) (string, error) {
	answer, err := claude.RunJSON(user, claude.Options{
		Schema:          judgeSchema,
		Model:           j.model,
		SystemPrompt:    judgeSystemPrompt,
		DisallowedTools: judgeDisallowedTools(),
		Role:            judgeRole,
		Timeout:         judgeBudget(ctx),
	})
	if err != nil {
		// An envelope that will not parse is a malformed answer, not a CLI
		// that reported failure — the two are read differently downstream.
		if errors.Is(err, claude.ErrUnparseable) {
			return "", fmt.Errorf("%w: %w", ErrJudgeMalformedResponse, err)
		}

		return "", fmt.Errorf("%w: %w", ErrJudgeCLI, err)
	}

	if !j.replayed {
		logJudgeUsage(answer)
	}

	if len(answer.Data) == 0 {
		return "", fmt.Errorf("%w: turn carried neither structured output nor text",
			ErrJudgeMalformedResponse)
	}

	return string(answer.Data), nil
}

// judgeRole labels the turn in the conversation history.
const judgeRole = "judge"

// defaultJudgeBudget bounds a verdict whose caller set no deadline.
const defaultJudgeBudget = 5 * time.Minute

// judgeBudget spends what the caller's context still allows: the judge is
// given its own deadline so a CLI run that timed out still gets a verdict.
func judgeBudget(ctx context.Context) time.Duration {
	deadline, ok := ctx.Deadline()
	if !ok {
		return defaultJudgeBudget
	}

	return time.Until(deadline)
}

// logJudgeUsage bills the verdict turn. The judge runs in the test
// process, so UsageSink only sees what the harness logs itself.
func logJudgeUsage(answer claude.Answer) {
	fields := []any{"cli", "claude", "cost_usd", answer.CostUSD}

	for _, key := range usageTokenKeys() {
		if count, ok := answer.Tokens[key]; ok {
			fields = append(fields, key, count)
		}
	}

	slog.Info(enginelog.MsgUsage, fields...)
}

// decodeJudgeVerdicts turns the verdict JSON into a run-level ruling.
// The schema cannot pin array length, so the per-clause accounting is
// checked here: a missing rule is malformed, never a silent pass.
func decodeJudgeVerdicts(payload string, clauses int) (bool, string, error) {
	var decoded judgeVerdicts

	err := json.Unmarshal([]byte(payload), &decoded)
	if err != nil {
		return false, "", fmt.Errorf("%w: %w (payload=%q)", ErrJudgeMalformedResponse, err, payload)
	}

	err = checkVerdictCoverage(decoded.Verdicts, clauses)
	if err != nil {
		return false, "", err
	}

	reasons := make([]string, 0, len(decoded.Verdicts))

	for _, verdict := range decoded.Verdicts {
		if verdict.Verdict != verdictPass {
			reasons = append(reasons, fmt.Sprintf("rule %d: %s", verdict.Rule, verdict.Reason))
		}
	}

	if len(reasons) == 0 {
		return true, "", nil
	}

	return false, strings.Join(reasons, "; "), nil
}

// checkVerdictCoverage requires exactly one ruling per clause, rules
// 1..N, each ruled pass or fail.
func checkVerdictCoverage(verdicts []clauseVerdict, clauses int) error {
	if len(verdicts) != clauses {
		return fmt.Errorf("%w: %d verdict(s) for %d clause(s)",
			ErrJudgeMalformedResponse, len(verdicts), clauses)
	}

	seen := make(map[int]bool, clauses)

	for _, verdict := range verdicts {
		if verdict.Rule < 1 || verdict.Rule > clauses || seen[verdict.Rule] {
			return fmt.Errorf("%w: rule %d is out of range or repeated",
				ErrJudgeMalformedResponse, verdict.Rule)
		}

		if verdict.Verdict != verdictPass && verdict.Verdict != "fail" {
			return fmt.Errorf("%w: rule %d ruled %q",
				ErrJudgeMalformedResponse, verdict.Rule, verdict.Verdict)
		}

		seen[verdict.Rule] = true
	}

	return nil
}

// judgeScope tells the judge what is NOT its job: every clause that
// reaches it survived mechanical checks already (exit code, output,
// files touched) — left unsaid, a model re-derives and fails on them.
const judgeScope = `Rules 1..%d below are this run's judged clauses — the assertions no
comparison could make. Everything else about the run has already been
checked mechanically by the test suite: its exit code, its output, and
which files it created, modified or left alone. Do not re-derive those
and do not fail the run for anything outside the numbered rules.`

// judgeTolerances is the noise policy, identical for every scenario —
// centralized after living as near-identical prose in all 46 rubrics,
// which was 46 places to update and 46 chances to disagree.
const judgeTolerances = `Ignore in every case, and never fail a run for:
  - anything under ` + "`tmp/`" + ` — per-run scratch, excluded by design
  - ` + "`node_modules/`, `.next/`" + `, and any other build or dependency cache
  - test-runner output: ` + "`test-results/`, `playwright-report/`, `.last-run.json`" + `,
    coverage reports, and anything else a runner writes about a run rather
    than as part of the project
  - lockfiles (` + "`go.sum`, `package-lock.json`" + `) and generated type stubs
Wording, ordering, ids and formatting are free unless a rule pins them.`

func buildJudgeUserPrompt(req JudgeRequest) string {
	var buf strings.Builder

	fmt.Fprintf(&buf, "## Command\n\n```\ntrue-bdd %s\n```\n\n", req.Cmd)
	buf.WriteString("## Specification\n\n")
	writeSpecification(&buf, req)
	buf.WriteString("## File diff\n\n")
	writeDiffSummary(&buf, req.Diff)

	return buf.String()
}

// writeSpecification renders the clauses into the numbered rubric the
// judge is given. There is no other source: a run with no clauses never
// reaches a judge at all.
func writeSpecification(buf *strings.Builder, req JudgeRequest) {
	fmt.Fprintf(buf, judgeScope+"\n\n", len(req.Clauses))

	for index, clause := range req.Clauses {
		fmt.Fprintf(buf, "%d. %s\n", index+1, strings.TrimSpace(clause))
	}

	fmt.Fprintf(buf, "\n## Tolerances\n\n%s\n\n", judgeTolerances)
}

func writeDiffSummary(buf *strings.Builder, diff []FileChange) {
	if len(judgeGraded(diff)) == 0 {
		buf.WriteString("(no files changed outside tmp/)\n")

		return
	}

	for _, change := range judgeGraded(diff) {
		fmt.Fprintf(buf, "### %s: `%s`\n\n", change.Kind, change.Path)

		// Both states, because a clause asking whether an untouched part
		// SURVIVED is unanswerable from the result alone — E2E-024 failed
		// on exactly that, the judge saying so in its own reason.
		if change.Kind != KindCreated {
			writeFileState(buf, "before", change.Before)
		}

		if change.Kind != KindDeleted {
			writeFileState(buf, "after", change.After)
		}
	}
}

func writeFileState(buf *strings.Builder, label string, body []byte) {
	fmt.Fprintf(buf, "%s:\n\n```\n", label)
	buf.Write(body)

	if len(body) > 0 && !strings.HasSuffix(string(body), "\n") {
		buf.WriteString("\n")
	}

	buf.WriteString("```\n\n")
}

// judgeGraded drops `tmp/`: the goldens already ignore it, no clause names
// it, and it carries the agent CLIs' own scratch — a 128KB SQLite db among it,
// which is machine state to judge nothing by.
func judgeGraded(diff []FileChange) []FileChange {
	graded := make([]FileChange, 0, len(diff))

	for _, change := range diff {
		if !strings.HasPrefix(change.Path, "tmp/") {
			graded = append(graded, change)
		}
	}

	return graded
}
