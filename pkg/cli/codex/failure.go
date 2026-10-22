package codex

import (
	"errors"
	"regexp"

	"github.com/ondatra-ai/true-bdd/pkg/shell"
)

// FailureStatus maps supervision errors to the persisted run status.
func FailureStatus(err error) string {
	if errors.Is(err, shell.ErrStalled) {
		return "stalled"
	}

	if errors.Is(err, shell.ErrTimeout) {
		return "timed-out"
	}

	return "failed"
}

// TerminalStatus recognizes codex-cli 0.162.1's terminal authentication and quota errors.
func TerminalStatus(output string) string {
	if regexp.MustCompile(`(?m)^ERROR:\s*Your access token could not be refreshed(?:\.| because )`).MatchString(output) {
		return "signed-out"
	}

	if regexp.MustCompile(`(?m)^ERROR:\s*You['’]ve hit your usage limit(?:\.| for )`).MatchString(output) {
		return "limit"
	}

	return ""
}
