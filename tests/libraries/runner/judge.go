package runner

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/enginelog"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/claudecode"
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

// ClaudeJudge takes verdicts through the in-repo claudecode wrapper: a
// schema-constrained, tool-less session on a pinned model, so a verdict
// is a function of its prompt and nothing else.
type ClaudeJudge struct {
	model string
}

// NewClaudeJudge pins the verdict model from the engine config. An
// unresolvable tier kills suite boot — never a silent fallback model.
func NewClaudeJudge(configPath string) (*ClaudeJudge, error) {
	model, err := JudgeModel(configPath)
	if err != nil {
		return nil, err
	}

	return &ClaudeJudge{model: model}, nil
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

// ask runs the verdict session and returns the raw verdict JSON.
func (j *ClaudeJudge) ask(ctx context.Context, user string) (string, error) {
	var payload string

	err := claudecode.WithClient(ctx, func(client claudecode.Client) error {
		queryErr := client.Query(ctx, user)
		if queryErr != nil {
			return fmt.Errorf("judge query: %w", queryErr)
		}

		var streamErr error

		payload, streamErr = readJudgePayload(ctx, client)

		return streamErr
	},
		claudecode.WithSystemPrompt(judgeSystemPrompt),
		claudecode.WithModel(j.model),
		claudecode.WithDisallowedTools(judgeDisallowedTools()...),
		claudecode.WithJSONSchema(judgeSchema),
	)
	if err != nil {
		return "", fmt.Errorf("judge session: %w", err)
	}

	return payload, nil
}

// readJudgePayload drains the stream to its ResultMessage. Text blocks
// are ignored: under a schema they carry pre-answer narration, never the
// verdict.
func readJudgePayload(ctx context.Context, client claudecode.Client) (string, error) {
	iter := client.ReceiveResponse(ctx)
	if iter == nil {
		return "", fmt.Errorf("%w: stream closed before any message", ErrJudgeMalformedResponse)
	}

	defer func() { _ = iter.Close() }()

	for {
		message, err := iter.Next(ctx)
		if err != nil {
			if errors.Is(err, claudecode.ErrNoMoreMessages) {
				return "", fmt.Errorf("%w: stream ended with no result", ErrJudgeMalformedResponse)
			}

			return "", fmt.Errorf("judge stream: %w", err)
		}

		result, ok := message.(*claudecode.ResultMessage)
		if !ok {
			continue
		}

		logJudgeUsage(result)

		if result.IsError {
			return "", fmt.Errorf("%w: %s", ErrJudgeCLI, result.ResultText)
		}

		return verdictPayload(result)
	}
}

// verdictPayload prefers the parsed structured_output and falls back to
// the string result, which carries the same JSON.
func verdictPayload(result *claudecode.ResultMessage) (string, error) {
	if result.StructuredOutput != nil {
		encoded, err := json.Marshal(result.StructuredOutput)
		if err != nil {
			return "", fmt.Errorf("%w: re-encode structured output: %w", ErrJudgeMalformedResponse, err)
		}

		return string(encoded), nil
	}

	if strings.TrimSpace(result.ResultText) != "" {
		return result.ResultText, nil
	}

	return "", fmt.Errorf("%w: result carried neither structured output nor text", ErrJudgeMalformedResponse)
}

// logJudgeUsage bills the verdict call. The judge runs in the test
// process, so UsageSink only sees what the harness logs itself.
func logJudgeUsage(result *claudecode.ResultMessage) {
	fields := []any{"cli", "claude"}

	if result.TotalCostUSD != nil {
		fields = append(fields, "cost_usd", *result.TotalCostUSD)
	}

	if result.Usage != nil {
		for _, key := range usageTokenKeys() {
			if count, ok := (*result.Usage)[key]; ok {
				fields = append(fields, key, count)
			}
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
		if change.Kind != "created" {
			writeFileState(buf, "before", change.Before)
		}

		if change.Kind != "deleted" {
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
