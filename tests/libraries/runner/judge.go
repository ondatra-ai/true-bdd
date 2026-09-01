package runner

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/adapters/ai"
)

// ErrJudgeEmptyFailReason is returned when Claude produces "FAIL:" with
// no follow-up explanation. The contract requires a one-sentence reason.
var ErrJudgeEmptyFailReason = errors.New("judge returned FAIL with empty reason")

// ErrJudgeMalformedResponse is returned when Claude's reply is neither
// "PASS" nor "FAIL: <reason>" on the first non-empty line.
var ErrJudgeMalformedResponse = errors.New("judge response did not match PASS or FAIL: <reason>")

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

// judgeModel pins the verdict model. The judge is harness
// infrastructure, not engine configuration: it must not drift when a
// host project retargets `engine.models` at a different provider.
const judgeModel = "sonnet"

// ClaudeJudge calls the existing true-bdd Claude wrapper as a soft
// check. It reuses ai.ClaudeProvider so we don't pull in a new SDK and
// don't introduce a new env var (the `claude` CLI handles auth).
type ClaudeJudge struct {
	client *ai.ClaudeProvider
}

// NewClaudeJudge constructs a ClaudeJudge backed by the existing
// ai.ClaudeProvider.
func NewClaudeJudge() (*ClaudeJudge, error) {
	return &ClaudeJudge{client: ai.NewClaudeProvider()}, nil
}

const judgeSystemPrompt = `You are an automated test verdict judge. You will be given:
1. A CLI command that was run.
2. An English specification of what the command was expected to do.
3. A diff of the files that changed during the run.

Your job is to compare the diff against the specification and decide
whether the run satisfies it.

Reply with EXACTLY ONE LINE in one of these two formats:
  PASS
  FAIL: <one-sentence reason>

Do not output anything else. No preamble, no explanation, no apologies.
A FAIL with no reason is invalid; always include a one-sentence reason.`

// Verdict implements Judge by calling Claude with a structured prompt.
func (j *ClaudeJudge) Verdict(ctx context.Context, req JudgeRequest) (JudgeOutcome, error) {
	user := buildJudgeUserPrompt(req)

	// Carried on the outcome even down the error paths: a malformed reply
	// is exactly when someone needs to read what was asked and answered.
	outcome := JudgeOutcome{SystemPrompt: judgeSystemPrompt, UserPrompt: user}

	resp, err := j.client.Execute(ctx, ai.Request{
		SystemPrompt: judgeSystemPrompt,
		UserPrompt:   user,
		Model:        judgeModel,
	})
	if err != nil {
		return outcome, fmt.Errorf("claude execute: %w", err)
	}

	outcome.Response = resp

	pass, reason, err := parseJudgeVerdict(resp)
	if err != nil {
		return outcome, err
	}

	outcome.Pass = pass
	outcome.Reason = reason

	return outcome, nil
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

// parseJudgeVerdict extracts the judge's verdict from its reply: the
// LAST verdict-shaped line wins (see TestParseJudgeVerdict for the
// real-fixture bug this fixed); a line must match exactly, not just contain the word.
func parseJudgeVerdict(resp string) (bool, string, error) {
	lines := strings.Split(resp, "\n")

	for index := len(lines) - 1; index >= 0; index-- {
		line := trimVerdictDecoration(lines[index])

		if strings.EqualFold(line, VerdictPass) {
			return true, "", nil
		}

		if !hasFailPrefix(line) {
			continue
		}

		reason := strings.TrimSpace(line[len("FAIL:"):])
		if reason == "" {
			return false, "", fmt.Errorf("%w: response=%q", ErrJudgeEmptyFailReason, resp)
		}

		return false, reason, nil
	}

	return false, "", fmt.Errorf("%w: got=%q", ErrJudgeMalformedResponse, resp)
}

// trimVerdictDecoration strips what a model puts around a verdict when
// it is writing prose rather than answering a form: markdown emphasis,
// code ticks, list bullets, and a trailing full stop.
func trimVerdictDecoration(line string) string {
	line = strings.TrimLeft(strings.TrimSpace(line), "-*#_` ")

	// Right side takes the full stop too, and in the same pass: emphasis
	// closes AFTER it in "**PASS**." but BEFORE it in "**PASS.**", and
	// two ordered trims can only ever handle one of those.
	return strings.TrimRight(line, "*_`. ")
}

// hasFailPrefix reports whether a line opens with the FAIL verdict, in
// any case the model chose to write it.
func hasFailPrefix(line string) bool {
	const prefix = "FAIL:"

	return len(line) >= len(prefix) && strings.EqualFold(line[:len(prefix)], prefix)
}
