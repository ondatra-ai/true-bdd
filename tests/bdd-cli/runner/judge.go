package runner

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ondatra-ai/true-bdd/src/adapters/ai"
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
	Cmd       string
	JudgeSpec string
	Diff      []FileChange
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

func buildJudgeUserPrompt(req JudgeRequest) string {
	var buf strings.Builder

	fmt.Fprintf(&buf, "## Command\n\n```\ntrue-bdd %s\n```\n\n", req.Cmd)
	fmt.Fprintf(&buf, "## Specification\n\n%s\n\n", strings.TrimSpace(req.JudgeSpec))
	buf.WriteString("## File diff\n\n")
	writeDiffSummary(&buf, req.Diff)

	return buf.String()
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

func parseJudgeVerdict(resp string) (bool, string, error) {
	line := strings.TrimSpace(resp)

	// Take the first non-empty line — Claude sometimes emits trailing
	// whitespace or an explanation line despite instructions.
	if idx := strings.IndexByte(line, '\n'); idx >= 0 {
		line = strings.TrimSpace(line[:idx])
	}

	switch {
	case line == VerdictPass:
		return true, "", nil
	case strings.HasPrefix(line, "FAIL:"):
		reason := strings.TrimSpace(strings.TrimPrefix(line, "FAIL:"))
		if reason == "" {
			return false, "", fmt.Errorf("%w: response=%q", ErrJudgeEmptyFailReason, resp)
		}

		return false, reason, nil
	default:
		return false, "", fmt.Errorf("%w: got=%q", ErrJudgeMalformedResponse, resp)
	}
}
