package clickup

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/ondatra-ai/true-bdd/scripts/internal/claudecli"
	"github.com/ondatra-ai/true-bdd/scripts/internal/textutil"
)

var (
	errStatusRefused = errors.New("the status was not set")
	errBlankArgument = errors.New("ticket, status and comment are all required")
)

const statusPromptTemplate = `Using the ClickUp MCP tools, do exactly two things to task %s:

1. set its status to %q;
2. add this comment, verbatim:

%s

Change NOTHING else — not the name, not the description, not a custom field.
Then reply with one word: OK if both succeeded, or FAILED: <reason>.
`

// Status moves one Ticket and records why — /task-done and /task-fail both
// reach ClickUp through it. The body is off limits: a closing turn that could
// rewrite the description could rewrite the spec it was just measured against.
func Status(out io.Writer, ticketID, status, comment string) error {
	if strings.TrimSpace(ticketID) == "" ||
		strings.TrimSpace(status) == "" ||
		strings.TrimSpace(comment) == "" {
		return errBlankArgument
	}

	answer, err := claudecli.Run(
		fmt.Sprintf(statusPromptTemplate, ticketID, status, comment),
		claudecli.Options{
			AllowedTools: statusTools,
			Role:         roleClickUp,
			Timeout:      claudeTimeout(),
		})
	if err != nil {
		return fmt.Errorf("setting %s to %s: %w", ticketID, status, err)
	}

	if !strings.HasPrefix(strings.TrimSpace(answer), "OK") {
		return fmt.Errorf("%w: %s", errStatusRefused, textutil.Truncate(answer, diagnosticLimit))
	}

	_, _ = fmt.Fprintf(out, "%s -> %s\n", ticketID, status)

	return nil
}
