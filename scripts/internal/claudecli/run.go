package claudecli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/scripts/internal/textutil"
	"github.com/ondatra-ai/true-bdd/scripts/report"
)

// Errors a caller distinguishes. Everything else is reported as it arrived.
var (
	// ErrTimeout reports that the turn outlived Options.Timeout. A stalled
	// MCP call must fail loudly: hanging here would leave the caller
	// believing work was done while nothing was.
	ErrTimeout = errors.New("claude -p timed out")
	// ErrFailed reports a non-zero exit.
	ErrFailed = errors.New("claude -p failed")
	// ErrUnparseable reports that --output-format json did not produce JSON.
	ErrUnparseable = errors.New("unparseable JSON output")
)

// diagnosticLimit is how much of a failed turn's output is quoted back. The
// scripts this ports all cut at 600 characters.
const diagnosticLimit = 600

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
	// Role labels the turn in the conversation history, via
	// CLAUDE_HISTORY_ROLE, so a run's own turns stay addressable afterwards.
	Role string
	// Timeout bounds the turn.
	Timeout time.Duration
}

func (o Options) args(prompt string) []string {
	args := make([]string, 0, 9) //nolint:mnd // the longest form: 4 flag pairs and -p.
	if o.AllowedTools != "" {
		args = append(args, "--allowedTools", o.AllowedTools)
	}

	if o.PermissionMode != "" {
		args = append(args, "--permission-mode", o.PermissionMode)
	}

	if o.Schema != "" {
		args = append(args, "--output-format", "json", "--json-schema", o.Schema)
	}

	// Last, always: everything above has to precede -p or the prompt is read
	// as the preceding flag's argument.
	return append(args, "-p", prompt)
}

// environ is the parent environment with the history role stamped on and
// CLAUDECODE removed — not blanked like other subprocess helpers do: a
// nested `claude -p` must look entirely unlaunched-from-a-session.
func (o Options) environ() []string {
	const roleKey = "CLAUDE_HISTORY_ROLE"

	env := make([]string, 0, len(os.Environ())+1)

	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if name == "CLAUDECODE" || name == roleKey {
			continue
		}

		env = append(env, entry)
	}

	return append(env, roleKey+"="+o.Role)
}

// turnName labels an AI turn in the report. Every model call in scripts/ comes
// through here, so this is the one place their cost is visible at all.
const turnName = "ai turn"

// Run performs one headless turn and returns its stdout.
func Run(prompt string, opts Options) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()

	var stdout, stderr bytes.Buffer

	cmd := exec.CommandContext(ctx, "claude", opts.args(prompt)...)
	cmd.Env = opts.environ()
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	started := time.Now()
	err := cmd.Run()

	if ctx.Err() != nil {
		report.Leaf(turnName, started, "role", opts.Role, report.KeyStatus, report.StatusFailed)

		return "", fmt.Errorf("%w after %s", ErrTimeout, opts.Timeout)
	}

	if err != nil {
		report.Leaf(turnName, started, "role", opts.Role, report.KeyStatus, report.StatusFailed)

		diagnostic := stderr.String()
		if diagnostic == "" {
			diagnostic = stdout.String()
		}

		return "", fmt.Errorf("%w (%s): %s",
			ErrFailed, exitCode(err), textutil.Truncate(diagnostic, diagnosticLimit))
	}

	report.Leaf(turnName, started, "role", opts.Role)

	return stdout.String(), nil
}

// RunJSON performs one headless turn under Options.Schema and returns the
// structured answer. `--output-format json` wraps it in an envelope: read
// structured_output when the schema was honoured, else result.
func RunJSON(prompt string, opts Options) (json.RawMessage, error) {
	stdout, err := Run(prompt, opts)
	if err != nil {
		return nil, err
	}

	var envelope struct {
		StructuredOutput json.RawMessage `json:"structured_output"`
		Result           json.RawMessage `json:"result"`
		// A denial is not a failure: in -p anything that would have prompted
		// is refused silently and the run still reports success, so a turn
		// that could not read looks exactly like one that read everything.
		PermissionDenials []struct {
			ToolName string `json:"tool_name"`
		} `json:"permission_denials"`
	}

	if json.Unmarshal([]byte(stdout), &envelope) != nil {
		const envelopeLimit = 400

		return nil, fmt.Errorf("%w: %s", ErrUnparseable, textutil.Truncate(stdout, envelopeLimit))
	}

	if len(envelope.PermissionDenials) > 0 {
		denied := make([]string, 0, len(envelope.PermissionDenials))
		for _, denial := range envelope.PermissionDenials {
			denied = append(denied, denial.ToolName)
		}

		slog.Warn("Tool calls were denied; this turn did less than it was asked to",
			"role", opts.Role, "denied", strings.Join(denied, " "))
	}

	if isEmptyJSON(envelope.StructuredOutput) {
		return envelope.Result, nil
	}

	return envelope.StructuredOutput, nil
}

// isEmptyJSON reports whether raw is one of the values Python's `or` would
// have fallen through: absent, null, or an empty object.
func isEmptyJSON(raw json.RawMessage) bool {
	text := strings.TrimSpace(string(raw))

	return text == "" || text == "null" || text == "{}"
}

func exitCode(err error) string {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return fmt.Sprintf("exit %d", exitErr.ExitCode())
	}

	return err.Error()
}
