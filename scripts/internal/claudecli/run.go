package claudecli

import (
	"encoding/json"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/cli/claude"
	"github.com/ondatra-ai/true-bdd/scripts/report"
)

// Errors a caller distinguishes, and the flags of one turn. Both are the
// wrapper's, aliased here: pkg/ may import no root, so what stayed behind is
// the cost accounting below and nothing else.
var (
	// ErrTimeout reports that the turn outlived Options.Timeout.
	ErrTimeout = claude.ErrTimeout
	// ErrFailed reports a non-zero exit.
	ErrFailed = claude.ErrFailed
	// ErrUnparseable reports that --output-format json did not produce JSON.
	ErrUnparseable = claude.ErrUnparseable
)

// Options are the flags of one headless turn.
type Options = claude.Options

// turnName labels an AI turn in the report. Every model call in scripts/ comes
// through here, so this is the one place their cost is visible at all.
const turnName = "ai turn"

// Run performs one headless turn and returns its stdout.
func Run(prompt string, opts Options) (string, error) {
	started := time.Now()

	stdout, err := claude.Run(prompt, opts)
	report.Leaf(turnName, started, attrs(opts.Role, err)...)

	return stdout, err
}

// RunJSON performs one headless turn under Options.Schema and returns the
// structured answer.
func RunJSON(prompt string, opts Options) (json.RawMessage, error) {
	started := time.Now()

	answer, err := claude.RunJSON(prompt, opts)
	report.Leaf(turnName, started, attrs(opts.Role, err)...)

	return answer.Data, err
}

// attrs marks the report node failed when the turn was, and says nothing
// otherwise: a node with no status reads as done.
func attrs(role string, err error) []any {
	if err == nil {
		return []any{"role", role}
	}

	return []any{"role", role, report.KeyStatus, report.StatusFailed}
}
