package runner

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/adapters/ai"
)

const (
	judgeMaxBytesPerFile = 16 * 1024
	judgeMaxBytesTotal   = 64 * 1024
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

// JudgeOutcome is one judge call: its decision, and the exact text the
// call was made of.
//
// The prompts are carried out rather than discarded because they are the
// only evidence of what the judge was actually asked. Two runs of the
// same fixture can disagree solely because their diffs differed, and
// without the prompt that difference is invisible — the report can show
// "PASS then FAIL" but never why. They are returned rather than written
// here so the judge stays a pure function of its request and the caller
// keeps sole responsibility for touching the filesystem.
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

// judgeScope tells the judge what is NOT its job.
//
// Every clause that reaches it survived a step definition's failure to
// express it, so the run's exit code, its output and which files it
// touched were all asserted mechanically before this call was made. Left
// unsaid, a model re-derives them from the diff and fails a run for a
// file the suite already approved.
const judgeScope = `Rules 1..%d below are this run's judged clauses — the assertions no
comparison could make. Everything else about the run has already been
checked mechanically by the test suite: its exit code, its output, and
which files it created, modified or left alone. Do not re-derive those
and do not fail the run for anything outside the numbered rules.`

// judgeTolerances is the noise policy, identical for every scenario.
//
// It was per-fixture prose in all 46 rubrics and near-identical in all
// 46 — which made it 46 places to update when the harness started
// excluding a new directory, and 46 chances for one of them to say
// something slightly different from the others.
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
	if len(diff) == 0 {
		buf.WriteString("(no files changed)\n")

		return
	}

	totalBytes := 0

	for _, change := range diff {
		fmt.Fprintf(buf, "### %s: `%s`\n\n", change.Kind, change.Path)

		if change.Kind == "deleted" {
			buf.WriteString("(deleted; omitting before-content)\n\n")

			continue
		}

		clipped, used := clipBody(change.After, totalBytes)
		totalBytes = used

		if clipped == "" {
			buf.WriteString("(content omitted: total budget reached)\n\n")

			continue
		}

		buf.WriteString("```\n")
		buf.WriteString(clipped)

		if !strings.HasSuffix(clipped, "\n") {
			buf.WriteString("\n")
		}

		buf.WriteString("```\n\n")
	}
}

func clipBody(body []byte, totalSoFar int) (string, int) {
	remaining := judgeMaxBytesTotal - totalSoFar
	if remaining <= 0 {
		return "", totalSoFar
	}

	limit := judgeMaxBytesPerFile
	if remaining < limit {
		limit = remaining
	}

	if len(body) <= limit {
		return string(body), totalSoFar + len(body)
	}

	return string(body[:limit]) + "\n…(truncated)…", totalSoFar + limit
}

// parseJudgeVerdict extracts the judge's verdict from its reply.
//
// The LAST verdict-shaped line wins, not the first. The rubric asks for
// a bare "PASS" or "FAIL: <reason>", but a model that works through the
// rubric point by point states its conclusion at the END — and reading
// only the first line turned those replies into "malformed response",
// failing fixtures whose runs were correct and whose judge agreed.
// Scanning from the end also resolves a reply that quotes one verdict
// while reaching the other ("...would FAIL if X ... PASS") in favour of
// the conclusion, which is what the words mean.
//
// A line only counts as a verdict when it is EXACTLY the word (after
// stripping markdown emphasis) or opens with "FAIL:". Prose that merely
// contains the word does not qualify — that is what keeps this
// tolerance from becoming a coin flip.
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
