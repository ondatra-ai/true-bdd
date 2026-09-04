// Package claude is the headless `claude -p` command line, one of the typed
// wrappers pkg/shell may be reached through.
//
// Every `claude` turn this repo spawns comes through here. A vendored SDK
// under services/bdd-cli/claudecode used to run the engine's turns over a
// bidirectional stream-json protocol; it was deleted once it was measured,
// because the engine only ever sent one prompt per spawn and the second
// path was where an un-isolated child leaked in (docs/adr/0010).
//
// Cost accounting stays with the caller: pkg/ may import no root, so the
// report.Leaf that every scripts/ turn is measured by lives in
// scripts/internal/claudecli. The envelope's own totals ride back on Answer,
// which is plain JSON and imports nothing.
package claude

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/shell"
)

// Bin is the binary this package spawns.
const Bin = "claude"

// roleKey labels a turn in the conversation history so a run's own turns stay
// addressable afterwards.
const roleKey = "CLAUDE_HISTORY_ROLE"

// Errors a caller distinguishes.
var (
	// ErrTimeout reports that the turn outlived Options.Timeout. A stalled
	// MCP call must fail loudly: hanging would leave the caller believing
	// work was done while nothing was.
	ErrTimeout = errors.New("claude -p timed out")
	// ErrFailed reports a non-zero exit.
	ErrFailed = errors.New("claude -p failed")
	// ErrUnparseable reports that --output-format json did not produce JSON.
	ErrUnparseable = errors.New("unparseable JSON output")
)

// Options are the flags of one headless turn. The zero value runs `claude -p`
// with no allowlist, which is only ever right for a turn that touches nothing.
type Options struct {
	// AllowedTools is passed to --allowedTools, ahead of -p.
	AllowedTools string
	// PermissionMode is passed to --permission-mode.
	PermissionMode string
	// Schema, when set, switches the turn to --output-format json with this
	// as --json-schema. Use RunJSON to read the result.
	Schema string
	// Model pins the turn to one model id. Empty leaves the CLI's default,
	// which is only ever right for a turn nobody grades.
	Model string
	// SystemPrompt replaces the CLI's own system prompt.
	SystemPrompt string
	// DisallowedTools strips the turn of tools by name. A turn whose whole
	// input is its prompt must be given no way to reach anything else.
	DisallowedTools []string
	// Role labels the turn in the conversation history via CLAUDE_HISTORY_ROLE.
	Role string
	// Dir is the turn's working directory; empty inherits the parent's. A
	// turn that reads a host workspace is spawned in it, so the relative
	// paths its prompt names resolve the way the prompt means them.
	Dir string
	// Envelope forces --output-format json for a turn that declares no
	// Schema, so one that answers in prose still reports what it cost.
	// RunJSON sets it; a caller reading Run's raw stdout leaves it off.
	Envelope bool
	// Stream switches the turn to stream-json so its records arrive as they
	// happen. RunStream sets it — the answer is the same either way, but only
	// this form can say WHEN a turn first spoke or which tools it called.
	Stream bool
	// Timeout bounds the turn.
	Timeout time.Duration
}

// isolation is the preamble every turn carries. User scope is where an
// operator's advisorModel and enabledPlugins live; project stays, because the
// workspace's own settings carry the permissions the turn must run under.
func isolation() []string {
	// Not --bare: it also skips CLAUDE.md and forces API-key auth (ADR 0010).
	return []string{
		"--setting-sources", "project",
		"--strict-mcp-config",
	}
}

// AdvisorOff is the only gate there is: the advisor is a SERVER-side tool, so
// --disallowed-tools cannot reach it and naming it there does nothing. Every
// spawner of a claude binary sets it — see ADR 0010.
const AdvisorOff = "CLAUDE_CODE_DISABLE_ADVISOR_TOOL=1"

// Args is the argv this turn spawns, exported so a caller can assert on it.
func (o Options) Args(prompt string) []string {
	args := make([]string, 0, 18) //nolint:mnd // the longest form: isolation, 7 flag pairs and -p.
	args = append(args, isolation()...)

	if o.AllowedTools != "" {
		args = append(args, "--allowedTools", o.AllowedTools)
	}

	if len(o.DisallowedTools) > 0 {
		args = append(args, "--disallowed-tools", strings.Join(o.DisallowedTools, ","))
	}

	if o.Model != "" {
		args = append(args, "--model", o.Model)
	}

	if o.SystemPrompt != "" {
		args = append(args, "--system-prompt", o.SystemPrompt)
	}

	if o.PermissionMode != "" {
		args = append(args, "--permission-mode", o.PermissionMode)
	}

	switch {
	case o.Stream:
		args = append(args, "--output-format", "stream-json", "--verbose")

		if o.Schema != "" {
			args = append(args, "--json-schema", o.Schema)
		}
	case o.Schema != "":
		args = append(args, "--output-format", "json", "--json-schema", o.Schema)
	case o.Envelope:
		args = append(args, "--output-format", "json")
	}

	// Last, always: everything above has to precede -p or the prompt is read
	// as the preceding flag's argument.
	return append(args, "-p", prompt)
}

// Env stamps the history role on and removes CLAUDECODE — removed, not
// blanked as the other wrappers do, because a nested `claude -p` must look
// entirely unlaunched-from-a-session.
func (o Options) Env() shell.Env {
	return shell.Inherit().Strip("CLAUDECODE", roleKey).Set(roleKey+"="+o.Role, AdvisorOff)
}

// diagnosticLimit is how much of a failed turn's output is quoted back.
const diagnosticLimit = 600

// Run performs one headless turn and returns its stdout.
func Run(prompt string, opts Options) (string, error) {
	result, err := shell.Run(context.Background(), append([]string{Bin}, opts.Args(prompt)...), shell.Options{
		Dir:     opts.Dir,
		Env:     opts.Env(),
		Timeout: opts.Timeout,
	})
	if err != nil {
		if errors.Is(err, shell.ErrTimeout) {
			return "", fmt.Errorf("%w after %s", ErrTimeout, opts.Timeout)
		}

		return "", fmt.Errorf("%w: %w", ErrFailed, err)
	}

	if result.Code != 0 {
		diagnostic := result.Stderr
		if diagnostic == "" {
			diagnostic = result.Stdout
		}

		return "", fmt.Errorf("%w (exit %d): %s",
			ErrFailed, result.Code, truncate(diagnostic, diagnosticLimit))
	}

	return result.Stdout, nil
}

// Answer is one schema-constrained turn: the structured payload, and what
// the turn cost. The totals are the envelope's own fields, carried so a
// caller that bills a turn does not have to spawn it a second way to find out.
type Answer struct {
	Data    json.RawMessage
	CostUSD float64
	Tokens  map[string]int
}

// RunJSON performs one headless turn under Options.Schema and returns the
// structured answer. `--output-format json` wraps it in an envelope: read
// structured_output when the schema was honoured, else result.
func RunJSON(prompt string, opts Options) (Answer, error) {
	opts.Envelope = true

	stdout, err := Run(prompt, opts)
	if err != nil {
		return Answer{}, err
	}

	return parseEnvelope([]byte(stdout), opts.Role)
}

// parseEnvelope reads the CLI's result object, whether it arrived alone under
// --output-format json or as the last line of a stream.
func parseEnvelope(stdout []byte, role string) (Answer, error) {
	var envelope struct {
		StructuredOutput json.RawMessage `json:"structured_output"`
		Result           json.RawMessage `json:"result"`
		CostUSD          float64         `json:"total_cost_usd"`
		// Nested, not flat: alongside the token counts the CLI reports
		// objects (output_tokens_details, server_tool_use), so the values
		// are read loosely and the non-numeric ones dropped.
		Usage map[string]any `json:"usage"`
		// A denial is not a failure: in -p anything that would have prompted
		// is refused silently and the run still reports success, so a turn
		// that could not read looks exactly like one that read everything.
		PermissionDenials []struct {
			ToolName string `json:"tool_name"`
		} `json:"permission_denials"`
	}

	if json.Unmarshal(stdout, &envelope) != nil {
		const envelopeLimit = 400

		return Answer{}, fmt.Errorf("%w: %s", ErrUnparseable, truncate(string(stdout), envelopeLimit))
	}

	warnDenials(envelope.PermissionDenials, role)

	answer := Answer{Data: envelope.StructuredOutput, CostUSD: envelope.CostUSD, Tokens: counts(envelope.Usage)}
	if isEmptyJSON(envelope.StructuredOutput) {
		answer.Data = envelope.Result
	}

	return answer, nil
}

// counts keeps the scalar token tallies and drops the nested breakdowns.
func counts(usage map[string]any) map[string]int {
	tokens := make(map[string]int, len(usage))

	for key, value := range usage {
		if number, ok := value.(float64); ok {
			tokens[key] = int(number)
		}
	}

	return tokens
}

func warnDenials(denials []struct {
	ToolName string `json:"tool_name"`
}, role string,
) {
	if len(denials) == 0 {
		return
	}

	denied := make([]string, 0, len(denials))
	for _, denial := range denials {
		denied = append(denied, denial.ToolName)
	}

	slog.Warn("Tool calls were denied; this turn did less than it was asked to",
		"role", role, "denied", strings.Join(denied, " "))
}

// isEmptyJSON reports whether raw is one of the values Python's `or` would
// have fallen through: absent, null, or an empty object.
func isEmptyJSON(raw json.RawMessage) bool {
	text := strings.TrimSpace(string(raw))

	return text == "" || text == "null" || text == "{}"
}

// truncate caps a diagnostic. scripts/internal/textutil does this for the
// tooling, but pkg/ may not import it.
func truncate(text string, limit int) string {
	if len(text) <= limit {
		return text
	}

	return text[:limit] + "…"
}
